package storage

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/RenlySir/timongo/internal/bsonutil"
)

var unsafeIdent = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// TiDBStore stores documents in TiDB/MySQL-compatible tables.
type TiDBStore struct {
	db *sql.DB
}

// NewTiDBStore opens a TiDB-backed store.
func NewTiDBStore(dsn string) (*TiDBStore, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	return &TiDBStore{db: db}, nil
}

// Close closes the underlying SQL pool.
func (s *TiDBStore) Close() error {
	return s.db.Close()
}

// Insert stores documents in a per-collection table.
func (s *TiDBStore) Insert(ctx context.Context, dbName, collection string, docs []bson.M) (InsertResult, error) {
	table := PhysicalTableName(dbName, collection)
	if err := s.ensureCollection(ctx, table); err != nil {
		return InsertResult{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return InsertResult{}, err
	}
	defer tx.Rollback()

	stmt := fmt.Sprintf("INSERT INTO `%s` (`_id_key`, `doc`) VALUES (?, CAST(? AS JSON))", table)
	for _, doc := range docs {
		key, err := bsonutil.DocumentIDKey(doc)
		if err != nil {
			return InsertResult{}, err
		}

		raw, err := bsonutil.MarshalExtJSON(doc)
		if err != nil {
			return InsertResult{}, err
		}

		if _, err = tx.ExecContext(ctx, stmt, key, string(raw)); err != nil {
			return InsertResult{}, err
		}
	}

	if err = tx.Commit(); err != nil {
		return InsertResult{}, err
	}

	return InsertResult{Inserted: len(docs)}, nil
}

// Find reads documents from a per-collection table.
func (s *TiDBStore) Find(ctx context.Context, dbName, collection string, req FindRequest) (FindResult, error) {
	table := PhysicalTableName(dbName, collection)
	if err := s.ensureCollection(ctx, table); err != nil {
		return FindResult{}, err
	}

	query, args, err := BuildFindSQL(dbName, collection, req)
	if err != nil {
		return FindResult{}, err
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return FindResult{}, err
	}
	defer rows.Close()

	var docs []bson.M
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return FindResult{}, err
		}

		doc, err := bsonutil.UnmarshalExtJSON([]byte(raw))
		if err != nil {
			return FindResult{}, err
		}

		docs = append(docs, doc)
	}

	if err = rows.Err(); err != nil {
		return FindResult{}, err
	}

	return FindResult{Documents: docs}, nil
}

func (s *TiDBStore) ensureCollection(ctx context.Context, table string) error {
	stmt := fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS `%s` ("+
			"`_id_key` VARBINARY(512) NOT NULL,"+
			"`doc` JSON NOT NULL,"+
			"`created_at` TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6),"+
			"`updated_at` TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),"+
			"PRIMARY KEY (`_id_key`)"+
			")",
		table,
	)

	_, err := s.db.ExecContext(ctx, stmt)
	return err
}

// PhysicalTableName returns a TiDB-safe table name for a MongoDB namespace.
func PhysicalTableName(dbName, collection string) string {
	base := "tm_" + dbName + "_" + collection
	base = unsafeIdent.ReplaceAllString(base, "_")
	base = strings.Trim(base, "_")
	if base == "" {
		return "tm_collection"
	}

	return strings.ToLower(base)
}

// BuildFindSQL builds SQL for the supported find subset.
func BuildFindSQL(dbName, collection string, req FindRequest) (string, []any, error) {
	table := PhysicalTableName(dbName, collection)
	query := fmt.Sprintf("SELECT doc FROM `%s`", table)
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
			query += " WHERE `_id_key` = ?"
			args = append(args, key)
		} else {
			query += " WHERE JSON_UNQUOTE(JSON_EXTRACT(doc, ?)) = ?"
			args = append(args, "$."+k, fmt.Sprint(v))
		}
	}

	if req.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, req.Limit)
	}

	return query, args, nil
}
