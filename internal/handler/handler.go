package handler

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/RenlySir/timongo/internal/backend"
	"github.com/RenlySir/timongo/internal/command"
	"github.com/RenlySir/timongo/internal/mongoerrors"
)

// Handler dispatches MongoDB commands to a backend store.
type Handler struct {
	store backend.Store
}

// New creates a Handler.
func New(store backend.Store) *Handler {
	return &Handler{store: store}
}

// Handle processes one BSON command document.
func (h *Handler) Handle(ctx context.Context, cmd bson.M) (bson.M, error) {
	name, value, err := command.Name(cmd)
	if err != nil {
		return nil, err
	}

	switch name {
	case "hello", "isMaster", "ismaster":
		return bson.M{
			"ok":                  float64(1),
			"isWritablePrimary":   true,
			"ismaster":            true,
			"minWireVersion":      int32(0),
			"maxWireVersion":      int32(21),
			"maxBsonObjectSize":   int32(16777216),
			"maxMessageSizeBytes": int32(48000000),
			"maxWriteBatchSize":   int32(100000),
		}, nil
	case "ping":
		return bson.M{"ok": float64(1)}, nil
	case "buildInfo":
		return bson.M{
			"ok":         float64(1),
			"version":    "0.1.0",
			"gitVersion": "timongo-mvp",
		}, nil
	case "serverStatus":
		return bson.M{"ok": float64(1), "version": "0.1.0", "timongo": bson.M{"compatVersion": "6.0"}}, nil
	case "create":
		coll, ok := value.(string)
		if !ok || coll == "" {
			return nil, mongoerrors.New(mongoerrors.CodeBadValue, "BadValue", "create must be a collection name")
		}
		return h.createCollection(ctx, cmd, coll)
	case "listCollections":
		return h.listCollections(ctx, cmd)
	case "drop":
		coll, ok := value.(string)
		if !ok || coll == "" {
			return nil, mongoerrors.New(mongoerrors.CodeBadValue, "BadValue", "drop must be a collection name")
		}
		return h.dropCollection(ctx, cmd, coll)
	case "dropDatabase":
		db, err := requiredString(cmd, "$db")
		if err != nil {
			return nil, err
		}
		if err := h.store.DropDatabase(ctx, db); err != nil {
			return nil, err
		}
		return bson.M{"ok": float64(1), "dropped": db}, nil
	case "createIndexes":
		coll, ok := value.(string)
		if !ok || coll == "" {
			return nil, mongoerrors.New(mongoerrors.CodeBadValue, "BadValue", "createIndexes must be a collection name")
		}
		return h.createIndexes(ctx, cmd, coll)
	case "listIndexes":
		coll, ok := value.(string)
		if !ok || coll == "" {
			return nil, mongoerrors.New(mongoerrors.CodeBadValue, "BadValue", "listIndexes must be a collection name")
		}
		return h.listIndexes(ctx, cmd, coll)
	case "insert":
		coll, ok := value.(string)
		if !ok || coll == "" {
			return nil, mongoerrors.New(mongoerrors.CodeBadValue, "BadValue", "insert must be a collection name")
		}
		return h.insert(ctx, cmd, coll)
	case "find":
		coll, ok := value.(string)
		if !ok || coll == "" {
			return nil, mongoerrors.New(mongoerrors.CodeBadValue, "BadValue", "find must be a collection name")
		}
		return h.find(ctx, cmd, coll)
	case "count":
		coll, ok := value.(string)
		if !ok || coll == "" {
			return nil, mongoerrors.New(mongoerrors.CodeBadValue, "BadValue", "count must be a collection name")
		}
		return h.count(ctx, cmd, coll)
	case "listDatabases":
		return bson.M{
			"ok":        float64(1),
			"databases": bson.A{},
			"totalSize": int64(0),
		}, nil
	default:
		return nil, mongoerrors.New(mongoerrors.CodeCommandNotFound, "CommandNotFound", "no such command: %s", name)
	}
}

func (h *Handler) createCollection(ctx context.Context, cmd bson.M, coll string) (bson.M, error) {
	db, err := requiredString(cmd, "$db")
	if err != nil {
		return nil, err
	}

	var validator bson.M
	if raw, ok := cmd["validator"]; ok {
		validator, ok = raw.(bson.M)
		if !ok {
			return nil, mongoerrors.New(mongoerrors.CodeBadValue, "BadValue", "validator has invalid type %T", raw)
		}
	}
	if err := h.store.CreateCollection(ctx, db, coll, backend.CreateCollectionOptions{Validator: validator}); err != nil {
		return nil, err
	}
	return bson.M{"ok": float64(1)}, nil
}

func (h *Handler) listCollections(ctx context.Context, cmd bson.M) (bson.M, error) {
	db, err := requiredString(cmd, "$db")
	if err != nil {
		return nil, err
	}
	filter := bson.M{}
	if raw, ok := cmd["filter"]; ok {
		filter, ok = raw.(bson.M)
		if !ok {
			return nil, mongoerrors.New(mongoerrors.CodeBadValue, "BadValue", "filter has invalid type %T", raw)
		}
	}
	res, err := h.store.ListCollections(ctx, db, filter)
	if err != nil {
		return nil, err
	}
	batch := make(bson.A, 0, len(res.Collections))
	for _, coll := range res.Collections {
		batch = append(batch, bson.M{
			"name":    coll.Name,
			"type":    "collection",
			"options": coll.Options,
			"info":    bson.M{"readOnly": false},
		})
	}
	return bson.M{
		"ok": float64(1),
		"cursor": bson.M{
			"id":         int64(0),
			"ns":         db + ".$cmd.listCollections",
			"firstBatch": batch,
		},
	}, nil
}

func (h *Handler) dropCollection(ctx context.Context, cmd bson.M, coll string) (bson.M, error) {
	db, err := requiredString(cmd, "$db")
	if err != nil {
		return nil, err
	}
	if err := h.store.DropCollection(ctx, db, coll); err != nil {
		return nil, err
	}
	return bson.M{"ok": float64(1), "ns": db + "." + coll}, nil
}

func (h *Handler) insert(ctx context.Context, cmd bson.M, coll string) (bson.M, error) {
	db, err := requiredString(cmd, "$db")
	if err != nil {
		return nil, err
	}

	rawDocs, ok := cmd["documents"].(bson.A)
	if !ok {
		return nil, mongoerrors.New(mongoerrors.CodeBadValue, "BadValue", "documents must be an array")
	}

	docs := make([]bson.M, 0, len(rawDocs))
	for _, raw := range rawDocs {
		doc, ok := raw.(bson.M)
		if !ok {
			return nil, mongoerrors.New(mongoerrors.CodeBadValue, "BadValue", "document has invalid type %T", raw)
		}
		docs = append(docs, doc)
	}

	res, err := h.store.Insert(ctx, db, coll, docs)
	if err != nil {
		return nil, err
	}

	return bson.M{"ok": float64(1), "n": int32(res.Inserted)}, nil
}

func (h *Handler) find(ctx context.Context, cmd bson.M, coll string) (bson.M, error) {
	db, err := requiredString(cmd, "$db")
	if err != nil {
		return nil, err
	}

	filter := bson.M{}
	if raw, ok := cmd["filter"]; ok {
		filter, ok = raw.(bson.M)
		if !ok {
			return nil, mongoerrors.New(mongoerrors.CodeBadValue, "BadValue", "filter has invalid type %T", raw)
		}
	}

	var limit int64
	switch raw := cmd["limit"].(type) {
	case nil:
	case int32:
		limit = int64(raw)
	case int64:
		limit = raw
	case int:
		limit = int64(raw)
	default:
		return nil, mongoerrors.New(mongoerrors.CodeBadValue, "BadValue", "limit has invalid type %T", raw)
	}

	res, err := h.store.Find(ctx, db, coll, backend.FindRequest{Filter: filter, Limit: limit})
	if err != nil {
		return nil, err
	}

	firstBatch := make(bson.A, 0, len(res.Documents))
	for _, doc := range res.Documents {
		firstBatch = append(firstBatch, doc)
	}

	return bson.M{
		"ok": float64(1),
		"cursor": bson.M{
			"id":         int64(0),
			"ns":         db + "." + coll,
			"firstBatch": firstBatch,
		},
	}, nil
}

func (h *Handler) count(ctx context.Context, cmd bson.M, coll string) (bson.M, error) {
	db, err := requiredString(cmd, "$db")
	if err != nil {
		return nil, err
	}
	filter := bson.M{}
	if raw, ok := cmd["query"]; ok {
		filter, ok = raw.(bson.M)
		if !ok {
			return nil, mongoerrors.New(mongoerrors.CodeBadValue, "BadValue", "query has invalid type %T", raw)
		}
	}
	res, err := h.store.Count(ctx, db, coll, backend.CountRequest{Filter: filter})
	if err != nil {
		return nil, err
	}
	return bson.M{"ok": float64(1), "n": res.Count}, nil
}

func (h *Handler) createIndexes(ctx context.Context, cmd bson.M, coll string) (bson.M, error) {
	db, err := requiredString(cmd, "$db")
	if err != nil {
		return nil, err
	}
	rawIndexes, ok := cmd["indexes"].(bson.A)
	if !ok {
		return nil, mongoerrors.New(mongoerrors.CodeBadValue, "BadValue", "indexes must be an array")
	}
	indexes := make([]backend.IndexModel, 0, len(rawIndexes))
	for _, raw := range rawIndexes {
		doc, ok := raw.(bson.M)
		if !ok {
			return nil, mongoerrors.New(mongoerrors.CodeBadValue, "BadValue", "index has invalid type %T", raw)
		}
		key, _ := doc["key"].(bson.M)
		name, _ := doc["name"].(string)
		opts := bson.M{}
		for k, v := range doc {
			if k == "name" || k == "key" {
				continue
			}
			opts[k] = v
		}
		indexes = append(indexes, backend.IndexModel{Name: name, Key: key, Opts: opts})
	}
	res, err := h.store.CreateIndexes(ctx, db, coll, indexes)
	if err != nil {
		return nil, err
	}
	return bson.M{
		"ok":                             float64(1),
		"createdCollectionAutomatically": true,
		"numIndexesBefore":               int32(1),
		"numIndexesAfter":                int32(1 + len(res.Names)),
	}, nil
}

func (h *Handler) listIndexes(ctx context.Context, cmd bson.M, coll string) (bson.M, error) {
	db, err := requiredString(cmd, "$db")
	if err != nil {
		return nil, err
	}
	res, err := h.store.ListIndexes(ctx, db, coll)
	if err != nil {
		return nil, err
	}
	batch := make(bson.A, 0, len(res.Indexes))
	for _, idx := range res.Indexes {
		batch = append(batch, idx)
	}
	return bson.M{
		"ok": float64(1),
		"cursor": bson.M{
			"id":         int64(0),
			"ns":         db + "." + coll,
			"firstBatch": batch,
		},
	}, nil
}

func requiredString(cmd bson.M, key string) (string, error) {
	raw, ok := cmd[key]
	if !ok {
		return "", mongoerrors.New(mongoerrors.CodeBadValue, "BadValue", "missing required field %s", key)
	}

	value, ok := raw.(string)
	if !ok || value == "" {
		return "", mongoerrors.New(mongoerrors.CodeBadValue, "BadValue", "%s must be a non-empty string", key)
	}

	return value, nil
}
