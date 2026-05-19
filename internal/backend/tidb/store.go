package tidb

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/RenlySir/timongo/internal/backend"
	"github.com/RenlySir/timongo/internal/bsonutil"
	"github.com/RenlySir/timongo/internal/catalog"
)

// Store stores documents in TiDB/MySQL-compatible tables.
type Store struct {
	db *sql.DB
}

// NewStore opens a TiDB-backed store.
func NewStore(dsn string) (*Store, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := catalog.Bootstrap(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

// Close closes the underlying SQL pool.
func (s *Store) Close() error {
	return s.db.Close()
}

// Ready reports whether the bound TiDB connection is reachable.
func (s *Store) Ready() error {
	return s.db.Ping()
}

// CreateCollection creates collection metadata and a physical document table.
func (s *Store) CreateCollection(ctx context.Context, dbName, collection string, opts backend.CreateCollectionOptions) error {
	table := PhysicalTableName(dbName, collection)
	if err := s.ensureCollection(ctx, table); err != nil {
		return err
	}
	options := bson.M{}
	if len(opts.Validator) > 0 {
		options["validator"] = opts.Validator
	}
	rawOptions, err := bsonutil.MarshalExtJSON(options)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		"INSERT INTO `_timongo`.`collections` (`database_name`, `collection_name`, `physical_table`, `options_json`, `schema_version`) VALUES (?, ?, ?, CAST(? AS JSON), 1) "+
			"ON DUPLICATE KEY UPDATE `physical_table` = VALUES(`physical_table`), `options_json` = VALUES(`options_json`)",
		[]byte(dbName), []byte(collection), table, string(rawOptions),
	)
	return err
}

// ListCollections returns collection metadata.
func (s *Store) ListCollections(ctx context.Context, dbName string, filter bson.M) (backend.ListCollectionsResult, error) {
	query := "SELECT `collection_name`, JSON_PRETTY(`options_json`) FROM `_timongo`.`collections` WHERE `database_name` = ?"
	args := []any{[]byte(dbName)}
	if name, ok := filter["name"].(string); ok {
		query += " AND `collection_name` = ?"
		args = append(args, []byte(name))
	}
	query += " ORDER BY `collection_name`"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return backend.ListCollectionsResult{}, err
	}
	defer rows.Close()

	var collections []backend.CollectionInfo
	for rows.Next() {
		var name []byte
		var rawOptions string
		if err = rows.Scan(&name, &rawOptions); err != nil {
			return backend.ListCollectionsResult{}, err
		}
		options, err := bsonutil.UnmarshalExtJSON([]byte(rawOptions))
		if err != nil {
			return backend.ListCollectionsResult{}, err
		}
		collections = append(collections, backend.CollectionInfo{Name: string(name), Options: options})
	}
	if err = rows.Err(); err != nil {
		return backend.ListCollectionsResult{}, err
	}
	return backend.ListCollectionsResult{Collections: collections}, nil
}

// DropCollection removes one collection table and metadata.
func (s *Store) DropCollection(ctx context.Context, dbName, collection string) error {
	table := PhysicalTableName(dbName, collection)
	if _, err := s.db.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS `%s`", table)); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, "DELETE FROM `_timongo`.`collections` WHERE `database_name` = ? AND `collection_name` = ?", []byte(dbName), []byte(collection))
	return err
}

// Insert stores documents in a per-collection table.
func (s *Store) Insert(ctx context.Context, dbName, collection string, docs []bson.M) (backend.InsertResult, error) {
	table := PhysicalTableName(dbName, collection)
	if err := s.CreateCollection(ctx, dbName, collection, backend.CreateCollectionOptions{}); err != nil {
		return backend.InsertResult{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return backend.InsertResult{}, err
	}
	defer tx.Rollback()

	stmt := fmt.Sprintf(
		"INSERT INTO `%s` (`id_key`, `id_bson`, `doc_bson`, `doc_json`, `revision`) VALUES (?, ?, ?, CAST(? AS JSON), 1)",
		table,
	)
	for _, doc := range docs {
		key, err := bsonutil.DocumentIDKey(doc)
		if err != nil {
			return backend.InsertResult{}, err
		}

		raw, err := bsonutil.MarshalExtJSON(doc)
		if err != nil {
			return backend.InsertResult{}, err
		}

		// M0 stores Extended JSON in doc_bson as a compatibility scaffold.
		// M1 replaces this with canonical BSON bytes.
		if _, err = tx.ExecContext(ctx, stmt, key, key, raw, string(raw)); err != nil {
			return backend.InsertResult{}, err
		}
	}

	if err = tx.Commit(); err != nil {
		return backend.InsertResult{}, err
	}

	return backend.InsertResult{Inserted: len(docs)}, nil
}

// Find reads documents from a per-collection table.
func (s *Store) Find(ctx context.Context, dbName, collection string, req backend.FindRequest) (backend.FindResult, error) {
	table := PhysicalTableName(dbName, collection)
	if err := s.ensureCollection(ctx, table); err != nil {
		return backend.FindResult{}, err
	}

	query, args, err := BuildFindSQL(dbName, collection, req)
	if err != nil {
		return backend.FindResult{}, err
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return backend.FindResult{}, err
	}
	defer rows.Close()

	var docs []bson.M
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return backend.FindResult{}, err
		}

		doc, err := bsonutil.UnmarshalExtJSON([]byte(raw))
		if err != nil {
			return backend.FindResult{}, err
		}

		docs = append(docs, doc)
	}

	if err = rows.Err(); err != nil {
		return backend.FindResult{}, err
	}

	return backend.FindResult{Documents: docs}, nil
}

// Count returns the number of documents matching the M0 filter subset.
func (s *Store) Count(ctx context.Context, dbName, collection string, req backend.CountRequest) (backend.CountResult, error) {
	find, err := s.Find(ctx, dbName, collection, backend.FindRequest{Filter: req.Filter})
	if err != nil {
		return backend.CountResult{}, err
	}
	return backend.CountResult{Count: int64(len(find.Documents))}, nil
}

// Distinct returns unique scalar values for one field.
func (s *Store) Distinct(ctx context.Context, dbName, collection string, req backend.DistinctRequest) (backend.DistinctResult, error) {
	find, err := s.Find(ctx, dbName, collection, backend.FindRequest{Filter: req.Filter})
	if err != nil {
		return backend.DistinctResult{}, err
	}
	seen := map[any]struct{}{}
	values := bson.A{}
	for _, doc := range find.Documents {
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

// Update applies a small MongoDB update subset using read-modify-write.
func (s *Store) Update(ctx context.Context, dbName, collection string, req backend.UpdateRequest) (backend.UpdateResult, error) {
	table := PhysicalTableName(dbName, collection)
	if err := s.CreateCollection(ctx, dbName, collection, backend.CreateCollectionOptions{}); err != nil {
		return backend.UpdateResult{}, err
	}

	docs, err := s.Find(ctx, dbName, collection, backend.FindRequest{})
	if err != nil {
		return backend.UpdateResult{}, err
	}

	var result backend.UpdateResult
	for _, op := range req.Updates {
		matchedForOp := int64(0)
		for _, doc := range docs.Documents {
			if !matchesFilter(doc, op.Filter) {
				continue
			}
			matchedForOp++
			result.Matched++
			next := cloneDoc(doc)
			if err := applyUpdate(next, op.Update); err != nil {
				return backend.UpdateResult{}, err
			}
			if err := s.replaceDocument(ctx, table, next); err != nil {
				return backend.UpdateResult{}, err
			}
			result.Modified++
			if !op.Multi {
				break
			}
		}
		if matchedForOp == 0 && op.Upsert {
			doc := cloneDoc(op.Filter)
			if err := applyUpdate(doc, op.Update); err != nil {
				return backend.UpdateResult{}, err
			}
			if _, ok := doc["_id"]; !ok {
				doc["_id"] = fmt.Sprintf("upsert-%d", time.Now().UnixNano())
			}
			if err := s.replaceDocument(ctx, table, doc); err != nil {
				return backend.UpdateResult{}, err
			}
			result.Matched++
			result.Modified++
			result.Upserted = append(result.Upserted, bson.M{"index": int32(0), "_id": doc["_id"]})
		}
	}
	return result, nil
}

// Delete deletes matching documents.
func (s *Store) Delete(ctx context.Context, dbName, collection string, req backend.DeleteRequest) (backend.DeleteResult, error) {
	table := PhysicalTableName(dbName, collection)
	find, err := s.Find(ctx, dbName, collection, backend.FindRequest{})
	if err != nil {
		return backend.DeleteResult{}, err
	}

	var deleted int64
	for _, op := range req.Deletes {
		for _, doc := range find.Documents {
			if !matchesFilter(doc, op.Filter) {
				continue
			}
			key, err := bsonutil.DocumentIDKey(doc)
			if err != nil {
				return backend.DeleteResult{}, err
			}
			if _, err = s.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM `%s` WHERE `id_key` = ?", table), key); err != nil {
				return backend.DeleteResult{}, err
			}
			deleted++
			if op.Limit == 1 {
				break
			}
		}
	}
	return backend.DeleteResult{Deleted: deleted}, nil
}

// Aggregate executes a small pipeline subset.
func (s *Store) Aggregate(ctx context.Context, dbName, collection string, req backend.AggregateRequest) (backend.AggregateResult, error) {
	find, err := s.Find(ctx, dbName, collection, backend.FindRequest{})
	if err != nil {
		return backend.AggregateResult{}, err
	}
	docs, err := applyPipeline(find.Documents, req.Pipeline)
	if err != nil {
		return backend.AggregateResult{}, err
	}
	return backend.AggregateResult{Documents: docs}, nil
}

// CreateIndexes stores index metadata.
func (s *Store) CreateIndexes(ctx context.Context, dbName, collection string, indexes []backend.IndexModel) (backend.CreateIndexesResult, error) {
	if err := s.CreateCollection(ctx, dbName, collection, backend.CreateCollectionOptions{}); err != nil {
		return backend.CreateIndexesResult{}, err
	}
	collectionID, err := s.collectionID(ctx, dbName, collection)
	if err != nil {
		return backend.CreateIndexesResult{}, err
	}

	names := make([]string, 0, len(indexes))
	for _, idx := range indexes {
		name := idx.Name
		if name == "" {
			name = "unnamed_1"
		}
		def := bson.M{"key": idx.Key}
		for k, v := range idx.Opts {
			def[k] = v
		}
		rawDef, err := bsonutil.MarshalExtJSON(def)
		if err != nil {
			return backend.CreateIndexesResult{}, err
		}
		if _, err = s.db.ExecContext(ctx,
			"INSERT INTO `_timongo`.`indexes` (`collection_id`, `name`, `definition_json`, `state`) VALUES (?, ?, CAST(? AS JSON), 'ready') "+
				"ON DUPLICATE KEY UPDATE `definition_json` = VALUES(`definition_json`), `state` = VALUES(`state`)",
			collectionID, name, string(rawDef),
		); err != nil {
			return backend.CreateIndexesResult{}, err
		}
		names = append(names, name)
	}
	return backend.CreateIndexesResult{Names: names}, nil
}

func (s *Store) replaceDocument(ctx context.Context, table string, doc bson.M) error {
	key, err := bsonutil.DocumentIDKey(doc)
	if err != nil {
		return err
	}
	raw, err := bsonutil.MarshalExtJSON(doc)
	if err != nil {
		return err
	}
	stmt := fmt.Sprintf(
		"REPLACE INTO `%s` (`id_key`, `id_bson`, `doc_bson`, `doc_json`, `revision`) VALUES (?, ?, ?, CAST(? AS JSON), 1)",
		table,
	)
	_, err = s.db.ExecContext(ctx, stmt, key, key, raw, string(raw))
	return err
}

// ListIndexes returns stored index metadata.
func (s *Store) ListIndexes(ctx context.Context, dbName, collection string) (backend.ListIndexesResult, error) {
	collectionID, err := s.collectionID(ctx, dbName, collection)
	if err == sql.ErrNoRows {
		return backend.ListIndexesResult{Indexes: []bson.M{{"name": "_id_", "key": bson.M{"_id": int32(1)}}}}, nil
	}
	if err != nil {
		return backend.ListIndexesResult{}, err
	}

	rows, err := s.db.QueryContext(ctx, "SELECT `name`, JSON_PRETTY(`definition_json`) FROM `_timongo`.`indexes` WHERE `collection_id` = ? ORDER BY `name`", collectionID)
	if err != nil {
		return backend.ListIndexesResult{}, err
	}
	defer rows.Close()

	indexes := []bson.M{{"name": "_id_", "key": bson.M{"_id": int32(1)}}}
	for rows.Next() {
		var name string
		var rawDef string
		if err = rows.Scan(&name, &rawDef); err != nil {
			return backend.ListIndexesResult{}, err
		}
		def, err := bsonutil.UnmarshalExtJSON([]byte(rawDef))
		if err != nil {
			return backend.ListIndexesResult{}, err
		}
		doc := bson.M{"name": name}
		if key, ok := def["key"]; ok {
			doc["key"] = key
		}
		for k, v := range def {
			if k != "key" {
				doc[k] = v
			}
		}
		indexes = append(indexes, doc)
	}
	if err = rows.Err(); err != nil {
		return backend.ListIndexesResult{}, err
	}
	return backend.ListIndexesResult{Indexes: indexes}, nil
}

// DropDatabase removes all collection tables and catalog rows for a MongoDB database.
func (s *Store) DropDatabase(ctx context.Context, dbName string) error {
	prefix := PhysicalTableName(dbName, "")
	prefix = strings.TrimSuffix(prefix, "_")
	likePattern := prefix + "_%"

	rows, err := s.db.QueryContext(ctx, "SHOW TABLES LIKE ?", likePattern)
	if err != nil {
		return err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err = rows.Scan(&table); err != nil {
			return err
		}
		tables = append(tables, table)
	}
	if err = rows.Err(); err != nil {
		return err
	}

	for _, table := range tables {
		if _, err = s.db.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS `%s`", table)); err != nil {
			return err
		}
	}

	_, err = s.db.ExecContext(ctx, "DELETE FROM `_timongo`.`collections` WHERE `database_name` = ?", []byte(dbName))
	return err
}

func (s *Store) ensureCollection(ctx context.Context, table string) error {
	_, err := s.db.ExecContext(ctx, DocumentTableDDL(table))
	return err
}

func (s *Store) collectionID(ctx context.Context, dbName, collection string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx,
		"SELECT `id` FROM `_timongo`.`collections` WHERE `database_name` = ? AND `collection_name` = ?",
		[]byte(dbName), []byte(collection),
	).Scan(&id)
	return id, err
}

func cloneDoc(doc bson.M) bson.M {
	res := make(bson.M, len(doc))
	for k, v := range doc {
		res[k] = v
	}
	return res
}

func matchesFilter(doc bson.M, filter bson.M) bool {
	for key, want := range filter {
		got, ok := doc[key]
		if !ok {
			return false
		}
		if !reflect.DeepEqual(got, want) {
			return false
		}
	}
	return true
}

func applyUpdate(doc bson.M, update bson.M) error {
	if len(update) == 0 {
		return nil
	}
	operatorStyle := false
	for key := range update {
		if len(key) > 0 && key[0] == '$' {
			operatorStyle = true
			break
		}
	}
	if !operatorStyle {
		for key := range doc {
			delete(doc, key)
		}
		for key, value := range update {
			doc[key] = value
		}
		return nil
	}
	for op, raw := range update {
		body, _ := raw.(bson.M)
		switch op {
		case "$set":
			for key, value := range body {
				doc[key] = value
			}
		case "$inc":
			for key, value := range body {
				doc[key] = addNumbers(doc[key], value)
			}
		case "$unset":
			for key := range body {
				delete(doc, key)
			}
		default:
			return fmt.Errorf("unsupported update operator %s", op)
		}
	}
	return nil
}

func addNumbers(left any, right any) any {
	switch l := left.(type) {
	case int32:
		return l + toInt32(right)
	case int64:
		return l + int64(toInt32(right))
	case int:
		return l + int(toInt32(right))
	case float64:
		return l + float64(toInt32(right))
	default:
		return toInt32(right)
	}
}

func toInt32(v any) int32 {
	switch n := v.(type) {
	case int32:
		return n
	case int64:
		return int32(n)
	case int:
		return int32(n)
	case float64:
		return int32(n)
	default:
		return 0
	}
}

func applyPipeline(input []bson.M, pipeline bson.A) ([]bson.M, error) {
	docs := make([]bson.M, 0, len(input))
	for _, doc := range input {
		docs = append(docs, cloneDoc(doc))
	}
	for _, rawStage := range pipeline {
		stage, ok := rawStage.(bson.M)
		if !ok {
			return nil, fmt.Errorf("aggregation stage has invalid type %T", rawStage)
		}
		for op, raw := range stage {
			switch op {
			case "$match":
				filter, _ := raw.(bson.M)
				filtered := docs[:0]
				for _, doc := range docs {
					if matchesFilter(doc, filter) {
						filtered = append(filtered, doc)
					}
				}
				docs = filtered
			case "$limit":
				n := int(toInt32(raw))
				if n < len(docs) {
					docs = docs[:n]
				}
			case "$skip":
				n := int(toInt32(raw))
				if n >= len(docs) {
					docs = nil
				} else {
					docs = docs[n:]
				}
			case "$sort":
				spec, _ := raw.(bson.M)
				for field, dir := range spec {
					desc := toInt32(dir) < 0
					sort.SliceStable(docs, func(i, j int) bool {
						less := fmt.Sprint(docs[i][field]) < fmt.Sprint(docs[j][field])
						if desc {
							return !less
						}
						return less
					})
					break
				}
			case "$project":
				spec, _ := raw.(bson.M)
				projected := make([]bson.M, 0, len(docs))
				for _, doc := range docs {
					next := bson.M{}
					includeID := true
					for field, include := range spec {
						if field == "_id" && toInt32(include) == 0 {
							includeID = false
							continue
						}
						if toInt32(include) != 0 {
							if value, ok := doc[field]; ok {
								next[field] = value
							}
						}
					}
					if includeID {
						if value, ok := doc["_id"]; ok {
							next["_id"] = value
						}
					}
					projected = append(projected, next)
				}
				docs = projected
			case "$count":
				name, _ := raw.(string)
				if name == "" {
					name = "count"
				}
				docs = []bson.M{{name: int64(len(docs))}}
			default:
				return nil, fmt.Errorf("unsupported aggregation stage %s", op)
			}
		}
	}
	return docs, nil
}

// BuildFindSQL builds SQL for the supported find subset.
func BuildFindSQL(dbName, collection string, req backend.FindRequest) (string, []any, error) {
	table := PhysicalTableName(dbName, collection)
	query := fmt.Sprintf("SELECT JSON_PRETTY(doc_json) FROM `%s`", table)
	args := make([]any, 0, 3)

	if len(req.Filter) > 1 {
		return "", nil, fmt.Errorf("only a single equality filter is supported")
	}

	for k, v := range req.Filter {
		if k == "_id" {
			key, err := bsonutil.ValueKey(v)
			if err != nil {
				return "", nil, err
			}
			query += " WHERE `id_key` = ?"
			args = append(args, key)
		} else {
			query += " WHERE JSON_UNQUOTE(JSON_EXTRACT(doc_json, ?)) = ?"
			args = append(args, "$."+k, fmt.Sprint(v))
		}
	}

	if req.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, req.Limit)
	}

	return query, args, nil
}
