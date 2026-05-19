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

func TestHandleUnknownCommandReturnsCommandError(t *testing.T) {
	h := New(memory.NewStore())

	_, err := h.Handle(context.Background(), bson.M{"dropDatabase": 1})
	if err == nil {
		t.Fatal("Handle returned nil error for unsupported command")
	}
}
