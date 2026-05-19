package storage

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMemoryStoreInsertAndFindByID(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	doc := bson.M{"_id": int32(1), "name": "Ada", "age": int32(20)}
	res, err := store.Insert(ctx, "app", "users", []bson.M{doc})
	if err != nil {
		t.Fatalf("Insert returned error: %v", err)
	}
	if res.Inserted != 1 {
		t.Fatalf("Inserted = %d, want 1", res.Inserted)
	}

	got, err := store.Find(ctx, "app", "users", FindRequest{Filter: bson.M{"_id": int32(1)}})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if len(got.Documents) != 1 {
		t.Fatalf("len(Documents) = %d, want 1", len(got.Documents))
	}
	if got.Documents[0]["name"] != "Ada" {
		t.Fatalf("name = %v, want Ada", got.Documents[0]["name"])
	}
}

func TestMemoryStoreFindByScalarEquality(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	_, err := store.Insert(ctx, "app", "users", []bson.M{
		{"_id": int32(1), "name": "Ada", "age": int32(20)},
		{"_id": int32(2), "name": "Grace", "age": int32(30)},
	})
	if err != nil {
		t.Fatalf("Insert returned error: %v", err)
	}

	got, err := store.Find(ctx, "app", "users", FindRequest{Filter: bson.M{"name": "Grace"}})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if len(got.Documents) != 1 {
		t.Fatalf("len(Documents) = %d, want 1", len(got.Documents))
	}
	if got.Documents[0]["_id"] != int32(2) {
		t.Fatalf("_id = %v, want 2", got.Documents[0]["_id"])
	}
}

func TestMemoryStoreRejectsDuplicateID(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	doc := bson.M{"_id": "same", "name": "Ada"}
	if _, err := store.Insert(ctx, "app", "users", []bson.M{doc}); err != nil {
		t.Fatalf("first Insert returned error: %v", err)
	}

	_, err := store.Insert(ctx, "app", "users", []bson.M{doc})
	if err == nil {
		t.Fatal("second Insert returned nil error")
	}
}
