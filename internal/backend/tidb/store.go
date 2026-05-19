package tidb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

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
