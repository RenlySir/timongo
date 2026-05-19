package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/RenlySir/timongo/internal/backend"
	"github.com/RenlySir/timongo/internal/bsonutil"
	"github.com/RenlySir/timongo/internal/mql"
)

// Store is an in-memory backend implementation for tests.
type Store struct {
	mu          sync.RWMutex
	collections map[string]map[string]bson.M
	options     map[string]bson.M
	indexes     map[string][]bson.M
}

// NewStore creates an empty in-memory store.
func NewStore() *Store {
	return &Store{
		collections: make(map[string]map[string]bson.M),
		options:     make(map[string]bson.M),
		indexes:     make(map[string][]bson.M),
	}
}

// CreateCollection creates collection metadata.
func (s *Store) CreateCollection(_ context.Context, db, collection string, opts backend.CreateCollectionOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns := namespace(db, collection)
	if s.collections[ns] == nil {
		s.collections[ns] = make(map[string]bson.M)
	}
	options := bson.M{}
	if len(opts.Validator) > 0 {
		options["validator"] = opts.Validator
	}
	s.options[ns] = options
	s.ensureDefaultIndex(ns)
	return nil
}

// ListCollections returns collection metadata.
func (s *Store) ListCollections(_ context.Context, db string, filter bson.M) (backend.ListCollectionsResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var collections []backend.CollectionInfo
	prefix := db + "."
	for ns := range s.collections {
		if len(ns) < len(prefix) || ns[:len(prefix)] != prefix {
			continue
		}
		name := ns[len(prefix):]
		if filterName, ok := filter["name"].(string); ok && filterName != name {
			continue
		}
		collections = append(collections, backend.CollectionInfo{Name: name, Options: cloneDoc(s.options[ns])})
	}
	return backend.ListCollectionsResult{Collections: collections}, nil
}

// DropCollection removes one collection.
func (s *Store) DropCollection(_ context.Context, db, collection string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns := namespace(db, collection)
	delete(s.collections, ns)
	delete(s.options, ns)
	delete(s.indexes, ns)
	return nil
}

// Insert stores documents in memory.
func (s *Store) Insert(_ context.Context, db, collection string, docs []bson.M) (backend.InsertResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	coll := s.collection(db, collection)
	ns := namespace(db, collection)
	s.ensureDefaultIndex(ns)

	for _, doc := range docs {
		if err := mql.ValidateJSONSchema(s.options[ns]["validator"], doc); err != nil {
			return backend.InsertResult{}, err
		}
		key, err := bsonutil.DocumentIDKey(doc)
		if err != nil {
			return backend.InsertResult{}, err
		}

		if _, ok := coll[key]; ok {
			return backend.InsertResult{}, fmt.Errorf("duplicate key: %s", key)
		}

		coll[key] = cloneDoc(doc)
	}

	return backend.InsertResult{Inserted: len(docs)}, nil
}

// Find returns documents matching a practical MongoDB filter subset.
func (s *Store) Find(_ context.Context, db, collection string, req backend.FindRequest) (backend.FindResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	coll := s.collections[namespace(db, collection)]
	if coll == nil {
		return backend.FindResult{}, nil
	}

	matched := make([]bson.M, 0)

	for _, doc := range coll {
		if !mql.Matches(doc, req.Filter) {
			continue
		}

		matched = append(matched, mql.CloneDoc(doc))
	}

	res := mql.ApplyFindOptions(matched, mql.FindOptions{
		Projection: req.Projection,
		Sort:       req.Sort,
		Skip:       req.Skip,
		Limit:      req.Limit,
	})
	return backend.FindResult{Documents: res}, nil
}

// Count returns the number of documents matching a filter.
func (s *Store) Count(ctx context.Context, db, collection string, req backend.CountRequest) (backend.CountResult, error) {
	res, err := s.Find(ctx, db, collection, backend.FindRequest{Filter: req.Filter})
	if err != nil {
		return backend.CountResult{}, err
	}
	return backend.CountResult{Count: int64(len(res.Documents))}, nil
}

// Distinct returns unique scalar values for a field.
func (s *Store) Distinct(ctx context.Context, db, collection string, req backend.DistinctRequest) (backend.DistinctResult, error) {
	res, err := s.Find(ctx, db, collection, backend.FindRequest{Filter: req.Filter})
	if err != nil {
		return backend.DistinctResult{}, err
	}
	seen := map[any]struct{}{}
	values := bson.A{}
	for _, doc := range res.Documents {
		value, ok := doc[req.Key]
		if !ok {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return backend.DistinctResult{Values: values}, nil
}

// Update applies a small MongoDB update subset.
func (s *Store) Update(_ context.Context, db, collection string, req backend.UpdateRequest) (backend.UpdateResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	coll := s.collection(db, collection)
	var result backend.UpdateResult
	for _, update := range req.Updates {
		matchedForOp := int64(0)
		for key, doc := range coll {
			if !mql.Matches(doc, update.Filter) {
				continue
			}
			matchedForOp++
			result.Matched++
			next := mql.CloneDoc(doc)
			if err := mql.ApplyUpdate(next, update.Update); err != nil {
				return backend.UpdateResult{}, err
			}
			coll[key] = next
			result.Modified++
			if !update.Multi {
				break
			}
		}
		if matchedForOp == 0 && update.Upsert {
			doc := mql.CloneDoc(update.Filter)
			if err := mql.ApplyUpdate(doc, update.Update); err != nil {
				return backend.UpdateResult{}, err
			}
			if _, ok := doc["_id"]; !ok {
				doc["_id"] = fmt.Sprintf("upsert-%d", time.Now().UnixNano())
			}
			key, err := bsonutil.DocumentIDKey(doc)
			if err != nil {
				return backend.UpdateResult{}, err
			}
			coll[key] = doc
			result.Upserted = append(result.Upserted, bson.M{"index": int32(0), "_id": doc["_id"]})
			result.Matched++
			result.Modified++
		}
	}
	return result, nil
}

// Delete deletes matching documents.
func (s *Store) Delete(_ context.Context, db, collection string, req backend.DeleteRequest) (backend.DeleteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	coll := s.collections[namespace(db, collection)]
	var deleted int64
	for _, del := range req.Deletes {
		for key, doc := range coll {
			if !mql.Matches(doc, del.Filter) {
				continue
			}
			delete(coll, key)
			deleted++
			if del.Limit == 1 {
				break
			}
		}
	}
	return backend.DeleteResult{Deleted: deleted}, nil
}

// Aggregate executes a small pipeline subset.
func (s *Store) Aggregate(ctx context.Context, db, collection string, req backend.AggregateRequest) (backend.AggregateResult, error) {
	find, err := s.Find(ctx, db, collection, backend.FindRequest{})
	if err != nil {
		return backend.AggregateResult{}, err
	}
	docs, err := mql.ApplyPipeline(find.Documents, req.Pipeline)
	if err != nil {
		return backend.AggregateResult{}, err
	}
	return backend.AggregateResult{Documents: docs}, nil
}

// CreateIndexes stores index metadata.
func (s *Store) CreateIndexes(_ context.Context, db, collection string, indexes []backend.IndexModel) (backend.CreateIndexesResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns := namespace(db, collection)
	if s.collections[ns] == nil {
		s.collections[ns] = make(map[string]bson.M)
	}
	s.ensureDefaultIndex(ns)
	names := make([]string, 0, len(indexes))
	for _, idx := range indexes {
		name := idx.Name
		if name == "" {
			name = indexName(idx.Key)
		}
		doc := bson.M{"name": name, "key": idx.Key}
		for k, v := range idx.Opts {
			doc[k] = v
		}
		s.indexes[ns] = append(s.indexes[ns], doc)
		names = append(names, name)
	}
	return backend.CreateIndexesResult{Names: names}, nil
}

// ListIndexes returns index metadata.
func (s *Store) ListIndexes(_ context.Context, db, collection string) (backend.ListIndexesResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ns := namespace(db, collection)
	indexes := make([]bson.M, 0, len(s.indexes[ns]))
	for _, idx := range s.indexes[ns] {
		indexes = append(indexes, mql.CloneDoc(idx))
	}
	return backend.ListIndexesResult{Indexes: indexes}, nil
}

// DropDatabase removes all in-memory collections for a database.
func (s *Store) DropDatabase(_ context.Context, db string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	prefix := db + "."
	for ns := range s.collections {
		if len(ns) >= len(prefix) && ns[:len(prefix)] == prefix {
			delete(s.collections, ns)
			delete(s.options, ns)
			delete(s.indexes, ns)
		}
	}
	return nil
}

func (s *Store) collection(db, collection string) map[string]bson.M {
	ns := namespace(db, collection)
	coll := s.collections[ns]
	if coll == nil {
		coll = make(map[string]bson.M)
		s.collections[ns] = coll
	}
	s.ensureDefaultIndex(ns)

	return coll
}

func (s *Store) ensureDefaultIndex(ns string) {
	if len(s.indexes[ns]) == 0 {
		s.indexes[ns] = []bson.M{{"name": "_id_", "key": bson.M{"_id": int32(1)}}}
	}
}

func namespace(db, collection string) string {
	return db + "." + collection
}

func cloneDoc(doc bson.M) bson.M {
	res := make(bson.M, len(doc))
	for k, v := range doc {
		res[k] = v
	}

	return res
}

func indexName(key bson.M) string {
	if len(key) == 0 {
		return "unnamed_1"
	}
	for field, direction := range key {
		return fmt.Sprintf("%s_%v", field, direction)
	}
	return "unnamed_1"
}
