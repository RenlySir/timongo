package handler

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/RenlySir/timongo/internal/mongoerrors"
	"github.com/RenlySir/timongo/internal/storage"
)

// Handler dispatches MongoDB commands to a backend store.
type Handler struct {
	store storage.Store
}

// New creates a Handler.
func New(store storage.Store) *Handler {
	return &Handler{store: store}
}

// Handle processes one BSON command document.
func (h *Handler) Handle(ctx context.Context, cmd bson.M) (bson.M, error) {
	name, value, err := commandName(cmd)
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
	default:
		return nil, mongoerrors.New(mongoerrors.CodeCommandNotFound, "CommandNotFound", "no such command: %s", name)
	}
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

	res, err := h.store.Find(ctx, db, coll, storage.FindRequest{Filter: filter, Limit: limit})
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

func commandName(cmd bson.M) (string, any, error) {
	for _, k := range []string{
		"hello",
		"isMaster",
		"ismaster",
		"ping",
		"buildInfo",
		"insert",
		"find",
	} {
		if v, ok := cmd[k]; ok {
			return k, v, nil
		}
	}

	return "", nil, mongoerrors.New(mongoerrors.CodeBadValue, "BadValue", "empty command")
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
