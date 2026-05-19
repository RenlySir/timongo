package memory

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/RenlySir/timongo/internal/backend"
)

func TestMemoryStoreInsertAndFindByID(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	doc := bson.M{"_id": int32(1), "name": "Ada", "age": int32(20)}
	res, err := store.Insert(ctx, "app", "users", []bson.M{doc})
	if err != nil {
		t.Fatalf("Insert returned error: %v", err)
	}
	if res.Inserted != 1 {
		t.Fatalf("Inserted = %d, want 1", res.Inserted)
	}

	got, err := store.Find(ctx, "app", "users", backend.FindRequest{Filter: bson.M{"_id": int32(1)}})
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
	store := NewStore()

	_, err := store.Insert(ctx, "app", "users", []bson.M{
		{"_id": int32(1), "name": "Ada", "age": int32(20)},
		{"_id": int32(2), "name": "Grace", "age": int32(30)},
	})
	if err != nil {
		t.Fatalf("Insert returned error: %v", err)
	}

	got, err := store.Find(ctx, "app", "users", backend.FindRequest{Filter: bson.M{"name": "Grace"}})
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

func TestMemoryStoreFindSupportsComparisonAndLogicalFilter(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	_, err := store.Insert(ctx, "app", "orders", []bson.M{
		{"_id": int32(1), "status": "paid", "total": int32(10), "region": "cn"},
		{"_id": int32(2), "status": "new", "total": int32(20), "region": "us"},
		{"_id": int32(3), "status": "paid", "total": int32(30), "region": "eu"},
	})
	if err != nil {
		t.Fatalf("Insert returned error: %v", err)
	}

	got, err := store.Find(ctx, "app", "orders", backend.FindRequest{
		Filter: bson.M{
			"$and": bson.A{
				bson.M{"status": "paid"},
				bson.M{"total": bson.M{"$gte": int32(20)}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if len(got.Documents) != 1 {
		t.Fatalf("len(Documents) = %d, want 1", len(got.Documents))
	}
	if got.Documents[0]["_id"] != int32(3) {
		t.Fatalf("_id = %v, want 3", got.Documents[0]["_id"])
	}
}

func TestMemoryStoreFindAppliesSortSkipLimitProjection(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	_, err := store.Insert(ctx, "app", "orders", []bson.M{
		{"_id": int32(1), "orderNo": "A", "status": "paid", "total": int32(20), "createdAt": int32(1)},
		{"_id": int32(2), "orderNo": "B", "status": "paid", "total": int32(40), "createdAt": int32(3)},
		{"_id": int32(3), "orderNo": "C", "status": "paid", "total": int32(60), "createdAt": int32(2)},
	})
	if err != nil {
		t.Fatalf("Insert returned error: %v", err)
	}

	got, err := store.Find(ctx, "app", "orders", backend.FindRequest{
		Filter:     bson.M{"status": "paid"},
		Sort:       bson.M{"createdAt": int32(-1)},
		Skip:       1,
		Limit:      1,
		Projection: bson.M{"orderNo": int32(1), "total": int32(1), "_id": int32(0)},
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if len(got.Documents) != 1 {
		t.Fatalf("len(Documents) = %d, want 1", len(got.Documents))
	}
	if got.Documents[0]["orderNo"] != "C" {
		t.Fatalf("orderNo = %v, want C", got.Documents[0]["orderNo"])
	}
	if _, ok := got.Documents[0]["_id"]; ok {
		t.Fatalf("document = %#v, want _id excluded", got.Documents[0])
	}
}

func TestMemoryStoreRejectsDuplicateID(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	doc := bson.M{"_id": "same", "name": "Ada"}
	if _, err := store.Insert(ctx, "app", "users", []bson.M{doc}); err != nil {
		t.Fatalf("first Insert returned error: %v", err)
	}

	_, err := store.Insert(ctx, "app", "users", []bson.M{doc})
	if err == nil {
		t.Fatal("second Insert returned nil error")
	}
}
