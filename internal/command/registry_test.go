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

func TestNamePrefersKnownCommandsSeenInCompatTest(t *testing.T) {
	tests := []struct {
		name string
		cmd  bson.M
		want string
	}{
		{name: "create over validator", cmd: bson.M{"validator": bson.M{}, "create": "orders", "$db": "app"}, want: "create"},
		{name: "listCollections over nameOnly", cmd: bson.M{"nameOnly": true, "listCollections": int32(1), "$db": "app"}, want: "listCollections"},
		{name: "createIndexes over indexes", cmd: bson.M{"indexes": bson.A{}, "createIndexes": "orders", "$db": "app"}, want: "createIndexes"},
		{name: "aggregate over cursor and pipeline", cmd: bson.M{"cursor": bson.M{}, "pipeline": bson.A{}, "aggregate": "orders", "$db": "app"}, want: "aggregate"},
		{name: "insert over documents and ordered", cmd: bson.M{"ordered": true, "documents": bson.A{}, "insert": "orders", "$db": "app"}, want: "insert"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Name(tt.cmd)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("Name() = %q, want %q", got, tt.want)
			}
		})
	}
}
