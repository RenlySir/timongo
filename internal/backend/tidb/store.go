package tidb

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/RenlySir/timongo/internal/backend"
	"github.com/RenlySir/timongo/internal/bsonutil"
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

	return &Store{db: db}, nil
}

// Close closes the underlying SQL pool.
func (s *Store) Close() error {
	return s.db.Close()
}

// Insert stores documents in a per-collection table.
func (s *Store) Insert(ctx context.Context, dbName, collection string, docs []bson.M) (backend.InsertResult, error) {
	table := PhysicalTableName(dbName, collection)
	if err := s.ensureCollection(ctx, table); err != nil {
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

func (s *Store) ensureCollection(ctx context.Context, table string) error {
	_, err := s.db.ExecContext(ctx, DocumentTableDDL(table))
	return err
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
