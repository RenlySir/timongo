package handler

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/RenlySir/timongo/internal/backend/memory"
)

func TestHandleHello(t *testing.T) {
	h := New(memory.NewStore())

	res, err := h.Handle(context.Background(), bson.M{"hello": 1})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if res["ok"] != float64(1) {
		t.Fatalf("ok = %v, want 1", res["ok"])
	}
	if res["isWritablePrimary"] != true {
		t.Fatalf("isWritablePrimary = %v, want true", res["isWritablePrimary"])
	}
}

func TestHandleInsertAndFind(t *testing.T) {
	h := New(memory.NewStore())
	ctx := context.Background()

	insertRes, err := h.Handle(ctx, bson.M{
		"insert": "users",
		"$db":    "app",
		"documents": bson.A{
			bson.M{"_id": int32(1), "name": "Ada"},
		},
	})
	if err != nil {
		t.Fatalf("insert returned error: %v", err)
	}
	if insertRes["n"] != int32(1) {
		t.Fatalf("insert n = %v, want 1", insertRes["n"])
	}

	findRes, err := h.Handle(ctx, bson.M{
		"find":   "users",
		"$db":    "app",
		"filter": bson.M{"_id": int32(1)},
	})
	if err != nil {
		t.Fatalf("find returned error: %v", err)
	}

	cursor, ok := findRes["cursor"].(bson.M)
	if !ok {
		t.Fatalf("cursor has type %T, want bson.M", findRes["cursor"])
	}

	batch, ok := cursor["firstBatch"].(bson.A)
	if !ok {
		t.Fatalf("firstBatch has type %T, want bson.A", cursor["firstBatch"])
	}
	if len(batch) != 1 {
		t.Fatalf("len(firstBatch) = %d, want 1", len(batch))
	}
}

func TestHandleCreateAndListCollections(t *testing.T) {
	h := New(memory.NewStore())
	ctx := context.Background()

	if _, err := h.Handle(ctx, bson.M{
		"create": "orders",
		"$db":    "app",
		"validator": bson.M{
			"$jsonSchema": bson.M{"required": bson.A{"orderNo"}},
		},
	}); err != nil {
		t.Fatalf("create returned error: %v", err)
	}

	res, err := h.Handle(ctx, bson.M{"listCollections": int32(1), "$db": "app"})
	if err != nil {
		t.Fatalf("listCollections returned error: %v", err)
	}
	cursor := res["cursor"].(bson.M)
	batch := cursor["firstBatch"].(bson.A)
	if len(batch) != 1 {
		t.Fatalf("len(firstBatch) = %d, want 1", len(batch))
	}
	info := batch[0].(bson.M)
	if info["name"] != "orders" {
		t.Fatalf("name = %v, want orders", info["name"])
	}
	options := info["options"].(bson.M)
	if _, ok := options["validator"]; !ok {
		t.Fatalf("options = %#v, want validator", options)
	}
}

func TestHandleCreateAndListIndexes(t *testing.T) {
	h := New(memory.NewStore())
	ctx := context.Background()

	res, err := h.Handle(ctx, bson.M{
		"createIndexes": "orders",
		"$db":           "app",
		"indexes": bson.A{
			bson.M{"name": "status_1", "key": bson.M{"status": int32(1)}},
		},
	})
	if err != nil {
		t.Fatalf("createIndexes returned error: %v", err)
	}
	names := res["createdCollectionAutomatically"].(bool)
	if !names {
		t.Fatal("createdCollectionAutomatically = false, want true")
	}

	list, err := h.Handle(ctx, bson.M{"listIndexes": "orders", "$db": "app"})
	if err != nil {
		t.Fatalf("listIndexes returned error: %v", err)
	}
	cursor := list["cursor"].(bson.M)
	batch := cursor["firstBatch"].(bson.A)
	if len(batch) < 2 {
		t.Fatalf("len(firstBatch) = %d, want at least 2", len(batch))
	}
}

func TestHandleCountAndDropCollection(t *testing.T) {
	h := New(memory.NewStore())
	ctx := context.Background()

	if _, err := h.Handle(ctx, bson.M{
		"insert": "users",
		"$db":    "app",
		"documents": bson.A{
			bson.M{"_id": int32(1), "name": "Ada"},
			bson.M{"_id": int32(2), "name": "Grace"},
		},
	}); err != nil {
		t.Fatalf("insert returned error: %v", err)
	}

	count, err := h.Handle(ctx, bson.M{"count": "users", "$db": "app", "query": bson.M{"name": "Ada"}})
	if err != nil {
		t.Fatalf("count returned error: %v", err)
	}
	if count["n"] != int64(1) {
		t.Fatalf("n = %v, want 1", count["n"])
	}

	if _, err := h.Handle(ctx, bson.M{"drop": "users", "$db": "app"}); err != nil {
		t.Fatalf("drop returned error: %v", err)
	}
	count, err = h.Handle(ctx, bson.M{"count": "users", "$db": "app"})
	if err != nil {
		t.Fatalf("count after drop returned error: %v", err)
	}
	if count["n"] != int64(0) {
		t.Fatalf("n after drop = %v, want 0", count["n"])
	}
}

func TestHandleUpdateDeleteAndDistinct(t *testing.T) {
	h := New(memory.NewStore())
	ctx := context.Background()

	if _, err := h.Handle(ctx, bson.M{
		"insert": "users",
		"$db":    "app",
		"documents": bson.A{
			bson.M{"_id": int32(1), "name": "Ada", "visits": int32(1), "region": "cn"},
			bson.M{"_id": int32(2), "name": "Grace", "visits": int32(3), "region": "us"},
		},
	}); err != nil {
		t.Fatalf("insert returned error: %v", err)
	}

	update, err := h.Handle(ctx, bson.M{
		"update": "users",
		"$db":    "app",
		"updates": bson.A{
			bson.M{
				"q": bson.M{"name": "Ada"},
				"u": bson.M{"$set": bson.M{"region": "eu"}, "$inc": bson.M{"visits": int32(2)}},
			},
		},
	})
	if err != nil {
		t.Fatalf("update returned error: %v", err)
	}
	if update["n"] != int64(1) {
		t.Fatalf("update n = %v, want 1", update["n"])
	}

	distinct, err := h.Handle(ctx, bson.M{"distinct": "users", "$db": "app", "key": "region"})
	if err != nil {
		t.Fatalf("distinct returned error: %v", err)
	}
	values := distinct["values"].(bson.A)
	if len(values) != 2 {
		t.Fatalf("len(values) = %d, want 2", len(values))
	}

	del, err := h.Handle(ctx, bson.M{
		"delete": "users",
		"$db":    "app",
		"deletes": bson.A{
			bson.M{"q": bson.M{"name": "Grace"}, "limit": int32(1)},
		},
	})
	if err != nil {
		t.Fatalf("delete returned error: %v", err)
	}
	if del["n"] != int64(1) {
		t.Fatalf("delete n = %v, want 1", del["n"])
	}
}

func TestHandleUnknownCommandReturnsCommandError(t *testing.T) {
	h := New(memory.NewStore())

	_, err := h.Handle(context.Background(), bson.M{"unsupportedCommand": 1})
	if err == nil {
		t.Fatal("Handle returned nil error for unsupported command")
	}
}

func TestHandleDropDatabase(t *testing.T) {
	h := New(memory.NewStore())
	ctx := context.Background()

	if _, err := h.Handle(ctx, bson.M{
		"insert": "users",
		"$db":    "app",
		"documents": bson.A{
			bson.M{"_id": int32(1), "name": "Ada"},
		},
	}); err != nil {
		t.Fatalf("insert returned error: %v", err)
	}

	res, err := h.Handle(ctx, bson.M{"dropDatabase": int32(1), "$db": "app"})
	if err != nil {
		t.Fatalf("dropDatabase returned error: %v", err)
	}
	if res["ok"] != float64(1) {
		t.Fatalf("ok = %v, want 1", res["ok"])
	}
	if res["dropped"] != "app" {
		t.Fatalf("dropped = %v, want app", res["dropped"])
	}

	findRes, err := h.Handle(ctx, bson.M{
		"find":   "users",
		"$db":    "app",
		"filter": bson.M{"_id": int32(1)},
	})
	if err != nil {
		t.Fatalf("find returned error: %v", err)
	}
	cursor := findRes["cursor"].(bson.M)
	batch := cursor["firstBatch"].(bson.A)
	if len(batch) != 0 {
		t.Fatalf("len(firstBatch) = %d, want 0", len(batch))
	}
}
