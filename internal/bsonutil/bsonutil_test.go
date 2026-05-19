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

func TestUnmarshalExtJSONNormalizesNestedDocuments(t *testing.T) {
	raw := []byte(`{
		"validator": {
			"$jsonSchema": {
				"required": ["lines"],
				"properties": {
					"lines": {"bsonType": "array"}
				}
			}
		},
		"lines": [
			{"sku": "SKU-1", "qty": {"$numberInt": "2"}}
		]
	}`)

	got, err := UnmarshalExtJSON(raw)
	if err != nil {
		t.Fatalf("UnmarshalExtJSON returned error: %v", err)
	}
	validator, ok := got["validator"].(bson.M)
	if !ok {
		t.Fatalf("validator has type %T, want bson.M", got["validator"])
	}
	schema, ok := validator["$jsonSchema"].(bson.M)
	if !ok {
		t.Fatalf("$jsonSchema has type %T, want bson.M", validator["$jsonSchema"])
	}
	properties, ok := schema["properties"].(bson.M)
	if !ok {
		t.Fatalf("properties has type %T, want bson.M", schema["properties"])
	}
	lines := got["lines"].(bson.A)
	if _, ok := lines[0].(bson.M); !ok {
		t.Fatalf("line item has type %T, want bson.M", lines[0])
	}
	if _, ok := properties["lines"].(bson.M); !ok {
		t.Fatalf("property has type %T, want bson.M", properties["lines"])
	}
}
