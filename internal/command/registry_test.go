package command

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestRegistryDispatchesRegisteredCommand(t *testing.T) {
	reg := NewRegistry()
	reg.Register("ping", HandlerFunc(func(ctx context.Context, cmd bson.M) (bson.M, error) {
		return bson.M{"ok": float64(1)}, nil
	}))

	res, err := reg.Handle(context.Background(), bson.M{"ping": int32(1)})
	if err != nil {
		t.Fatal(err)
	}
	if res["ok"] != float64(1) {
		t.Fatalf("ok = %v", res["ok"])
	}
}

func TestRegistryReturnsCommandNotFound(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Handle(context.Background(), bson.M{"unknown": int32(1)})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNamePrefersKnownCommandOverPayloadFields(t *testing.T) {
	name, value, err := Name(bson.M{
		"documents": bson.A{},
		"insert":    "users",
		"$db":       "app",
	})
	if err != nil {
		t.Fatal(err)
	}
	if name != "insert" {
		t.Fatalf("name = %q, want insert", name)
	}
	if value != "users" {
		t.Fatalf("value = %v, want users", value)
	}
}
