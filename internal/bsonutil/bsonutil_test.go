package bsonutil

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestDocumentIDKeyPreservesObjectID(t *testing.T) {
	id := bson.NewObjectID()
	key, err := DocumentIDKey(bson.M{"_id": id, "name": "Ada"})
	if err != nil {
		t.Fatalf("DocumentIDKey returned error: %v", err)
	}

	if key != "objectid:"+id.Hex() {
		t.Fatalf("DocumentIDKey() = %q, want objectid:%s", key, id.Hex())
	}
}

func TestDocumentIDKeyRejectsMissingID(t *testing.T) {
	_, err := DocumentIDKey(bson.M{"name": "Ada"})
	if err == nil {
		t.Fatal("DocumentIDKey returned nil error for missing _id")
	}
}

func TestJSONRoundTripPreservesDocumentFields(t *testing.T) {
	doc := bson.M{"_id": int32(1), "name": "Ada", "active": true}

	raw, err := MarshalExtJSON(doc)
	if err != nil {
		t.Fatalf("MarshalExtJSON returned error: %v", err)
	}

	got, err := UnmarshalExtJSON(raw)
	if err != nil {
		t.Fatalf("UnmarshalExtJSON returned error: %v", err)
	}

	if got["name"] != "Ada" {
		t.Fatalf("name = %v, want Ada", got["name"])
	}

	if got["active"] != true {
		t.Fatalf("active = %v, want true", got["active"])
	}
}
