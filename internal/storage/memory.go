package storage

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/RenlySir/timongo/internal/bsonutil"
)

// MemoryStore is an in-memory Store implementation for tests and local demos.
type MemoryStore struct {
	mu          sync.RWMutex
	collections map[string]map[string]bson.M
}

// NewMemoryStore creates an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		collections: make(map[string]map[string]bson.M),
	}
}

// Insert stores documents in memory.
func (s *MemoryStore) Insert(_ context.Context, db, collection string, docs []bson.M) (InsertResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	coll := s.collection(db, collection)

	for _, doc := range docs {
		key, err := bsonutil.DocumentIDKey(doc)
		if err != nil {
			return InsertResult{}, err
		}

		if _, ok := coll[key]; ok {
			return InsertResult{}, fmt.Errorf("duplicate key: %s", key)
		}

		coll[key] = cloneDoc(doc)
	}

	return InsertResult{Inserted: len(docs)}, nil
}

// Find returns documents matching a small equality-only filter subset.
func (s *MemoryStore) Find(_ context.Context, db, collection string, req FindRequest) (FindResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	coll := s.collections[namespace(db, collection)]
	if coll == nil {
		return FindResult{}, nil
	}

	limit := req.Limit
	res := make([]bson.M, 0)

	for _, doc := range coll {
		if !matches(doc, req.Filter) {
			continue
		}

		res = append(res, cloneDoc(doc))
		if limit > 0 && int64(len(res)) >= limit {
			break
		}
	}

	return FindResult{Documents: res}, nil
}

func (s *MemoryStore) collection(db, collection string) map[string]bson.M {
	ns := namespace(db, collection)
	coll := s.collections[ns]
	if coll == nil {
		coll = make(map[string]bson.M)
		s.collections[ns] = coll
	}

	return coll
}

func namespace(db, collection string) string {
	return db + "." + collection
}

func matches(doc bson.M, filter bson.M) bool {
	for k, want := range filter {
		got, ok := doc[k]
		if !ok {
			return false
		}

		if !reflect.DeepEqual(got, want) {
			return false
		}
	}

	return true
}

func cloneDoc(doc bson.M) bson.M {
	res := make(bson.M, len(doc))
	for k, v := range doc {
		res[k] = v
	}

	return res
}
