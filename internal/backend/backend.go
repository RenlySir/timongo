package backend

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Store is the durable backend used by MongoDB command handlers.
type Store interface {
	CreateCollection(ctx context.Context, db, collection string, opts CreateCollectionOptions) error
	ListCollections(ctx context.Context, db string, filter bson.M) (ListCollectionsResult, error)
	DropCollection(ctx context.Context, db, collection string) error
	Insert(ctx context.Context, db, collection string, docs []bson.M) (InsertResult, error)
	Find(ctx context.Context, db, collection string, req FindRequest) (FindResult, error)
	Count(ctx context.Context, db, collection string, req CountRequest) (CountResult, error)
	Distinct(ctx context.Context, db, collection string, req DistinctRequest) (DistinctResult, error)
	Update(ctx context.Context, db, collection string, req UpdateRequest) (UpdateResult, error)
	Delete(ctx context.Context, db, collection string, req DeleteRequest) (DeleteResult, error)
	CreateIndexes(ctx context.Context, db, collection string, indexes []IndexModel) (CreateIndexesResult, error)
	ListIndexes(ctx context.Context, db, collection string) (ListIndexesResult, error)
	DropDatabase(ctx context.Context, db string) error
}

// CreateCollectionOptions contains M0 collection metadata.
type CreateCollectionOptions struct {
	Validator bson.M
}

// CollectionInfo describes one collection.
type CollectionInfo struct {
	Name    string
	Options bson.M
}

// ListCollectionsResult contains collection metadata.
type ListCollectionsResult struct {
	Collections []CollectionInfo
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

// CountRequest describes a count command.
type CountRequest struct {
	Filter bson.M
}

// CountResult describes a count command result.
type CountResult struct {
	Count int64
}

// DistinctRequest describes a distinct command.
type DistinctRequest struct {
	Key    string
	Filter bson.M
}

// DistinctResult describes distinct values.
type DistinctResult struct {
	Values bson.A
}

// UpdateRequest describes an update command.
type UpdateRequest struct {
	Updates []UpdateModel
}

// UpdateModel describes one update operation.
type UpdateModel struct {
	Filter bson.M
	Update bson.M
	Multi  bool
	Upsert bool
}

// UpdateResult describes update results.
type UpdateResult struct {
	Matched  int64
	Modified int64
	Upserted []bson.M
}

// DeleteRequest describes a delete command.
type DeleteRequest struct {
	Deletes []DeleteModel
}

// DeleteModel describes one delete operation.
type DeleteModel struct {
	Filter bson.M
	Limit  int64
}

// DeleteResult describes delete results.
type DeleteResult struct {
	Deleted int64
}

// IndexModel describes one MongoDB index definition.
type IndexModel struct {
	Name string
	Key  bson.M
	Opts bson.M
}

// CreateIndexesResult describes createIndexes output.
type CreateIndexesResult struct {
	Names []string
}

// ListIndexesResult describes listIndexes output.
type ListIndexesResult struct {
	Indexes []bson.M
}
