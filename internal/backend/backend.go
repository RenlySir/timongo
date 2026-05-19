package backend

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Store is the durable backend used by MongoDB command handlers.
type Store interface {
	Insert(ctx context.Context, db, collection string, docs []bson.M) (InsertResult, error)
	Find(ctx context.Context, db, collection string, req FindRequest) (FindResult, error)
}

// InsertResult describes an insert command result.
type InsertResult struct {
	Inserted int
}

// FindRequest describes the supported M0 find subset.
type FindRequest struct {
	Filter bson.M
	Limit  int64
}

// FindResult describes a find command result.
type FindResult struct {
	Documents []bson.M
}
