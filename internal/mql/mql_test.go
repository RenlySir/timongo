package mql

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMatchesComparisonAndLogicalOperators(t *testing.T) {
	doc := bson.M{"status": "paid", "total": float64(75), "tags": bson.A{"compat", "order"}}

	tests := []bson.M{
		{"status": bson.M{"$eq": "paid"}},
		{"status": bson.M{"$ne": "cancelled"}},
		{"total": bson.M{"$gt": float64(50)}},
		{"total": bson.M{"$gte": float64(75)}},
		{"total": bson.M{"$lt": float64(100)}},
		{"total": bson.M{"$lte": float64(75)}},
		{"status": bson.M{"$in": bson.A{"paid", "new"}}},
		{"missing": bson.M{"$exists": false}},
		{"status": bson.M{"$regex": "^pa"}},
		{"$and": bson.A{bson.M{"status": "paid"}, bson.M{"total": bson.M{"$gte": float64(70)}}}},
		{"$or": bson.A{bson.M{"status": "new"}, bson.M{"status": "paid"}}},
	}

	for _, filter := range tests {
		if !Matches(doc, filter) {
			t.Fatalf("Matches(%#v) = false, want true", filter)
		}
	}
}

func TestMatchesArrayElemMatchAndDotPath(t *testing.T) {
	doc := bson.M{
		"lines": bson.A{
			bson.M{"sku": "SKU-1", "qty": int32(2)},
			bson.M{"sku": "SKU-2", "qty": int32(1)},
		},
		"tags": bson.A{"compat", "order"},
	}

	tests := []bson.M{
		{"lines": bson.M{"$elemMatch": bson.M{"sku": "SKU-1", "qty": bson.M{"$gte": int32(1)}}}},
		{"lines.sku": "SKU-1"},
		{"tags": bson.M{"$all": bson.A{"compat", "order"}}},
		{"lines": bson.M{"$size": int32(2)}},
	}
	for _, filter := range tests {
		if !Matches(doc, filter) {
			t.Fatalf("Matches(%#v) = false, want true", filter)
		}
	}
}

func TestApplyFindOptionsSortSkipLimitProjection(t *testing.T) {
	docs := []bson.M{
		{"_id": int32(1), "orderNo": "A", "total": int32(20), "createdAt": int32(1)},
		{"_id": int32(2), "orderNo": "B", "total": int32(40), "createdAt": int32(3)},
		{"_id": int32(3), "orderNo": "C", "total": int32(60), "createdAt": int32(2)},
	}

	got := ApplyFindOptions(docs, FindOptions{
		Sort:       bson.M{"createdAt": int32(-1)},
		Skip:       1,
		Limit:      1,
		Projection: bson.M{"orderNo": int32(1), "total": int32(1), "_id": int32(0)},
	})
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0]["orderNo"] != "C" || got[0]["total"] != int32(60) {
		t.Fatalf("got[0] = %#v, want order C total 60", got[0])
	}
	if _, ok := got[0]["_id"]; ok {
		t.Fatalf("got[0] = %#v, want _id excluded", got[0])
	}
}

func TestApplyPipelineGroupAccumulators(t *testing.T) {
	docs := []bson.M{
		{"_id": int32(1), "status": "paid", "total": float64(10)},
		{"_id": int32(2), "status": "paid", "total": float64(30)},
		{"_id": int32(3), "status": "new", "total": float64(5)},
	}

	got, err := ApplyPipeline(docs, bson.A{
		bson.M{"$match": bson.M{"status": "paid"}},
		bson.M{"$group": bson.M{
			"_id":     "$status",
			"count":   bson.M{"$sum": int32(1)},
			"revenue": bson.M{"$sum": "$total"},
			"avg":     bson.M{"$avg": "$total"},
			"min":     bson.M{"$min": "$total"},
			"max":     bson.M{"$max": "$total"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0]["_id"] != "paid" {
		t.Fatalf("_id = %v, want paid", got[0]["_id"])
	}
	if got[0]["count"] != float64(2) {
		t.Fatalf("count = %v, want 2", got[0]["count"])
	}
	if got[0]["revenue"] != float64(40) {
		t.Fatalf("revenue = %v, want 40", got[0]["revenue"])
	}
}

func TestApplyPipelineCommonLightweightStages(t *testing.T) {
	docs := []bson.M{
		{"_id": int32(1), "status": "paid", "region": "cn", "lines": bson.A{bson.M{"sku": "S1"}}},
		{"_id": int32(2), "status": "paid", "region": "cn", "lines": bson.A{bson.M{"sku": "S2"}}},
		{"_id": int32(3), "status": "new", "region": "us", "lines": bson.A{bson.M{"sku": "S3"}}},
	}

	got, err := ApplyPipeline(docs, bson.A{
		bson.M{"$addFields": bson.M{"compatFlag": true}},
		bson.M{"$unset": "region"},
		bson.M{"$unwind": "$lines"},
		bson.M{"$group": bson.M{
			"_id":    "$status",
			"orders": bson.M{"$push": "$lines.sku"},
			"flags":  bson.M{"$addToSet": "$compatFlag"},
		}},
		bson.M{"$sortByCount": "$_id"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("len(got) = 0, want rows")
	}
}

func TestApplyUpdatePreservesIDForReplacementAndSupportsCommonOperators(t *testing.T) {
	tests := []struct {
		name   string
		doc    bson.M
		update bson.M
		assert func(t *testing.T, doc bson.M)
	}{
		{
			name:   "mul",
			doc:    bson.M{"_id": int32(1), "score": int32(5)},
			update: bson.M{"$mul": bson.M{"score": int32(2)}},
			assert: func(t *testing.T, doc bson.M) {
				if doc["score"] != float64(10) {
					t.Fatalf("score = %v, want 10", doc["score"])
				}
			},
		},
		{
			name:   "min",
			doc:    bson.M{"_id": int32(1), "score": int32(5)},
			update: bson.M{"$min": bson.M{"score": int32(3)}},
			assert: func(t *testing.T, doc bson.M) {
				if doc["score"] != int32(3) {
					t.Fatalf("score = %v, want 3", doc["score"])
				}
			},
		},
		{
			name:   "max",
			doc:    bson.M{"_id": int32(1), "score": int32(5)},
			update: bson.M{"$max": bson.M{"score": int32(12)}},
			assert: func(t *testing.T, doc bson.M) {
				if doc["score"] != int32(12) {
					t.Fatalf("score = %v, want 12", doc["score"])
				}
			},
		},
		{
			name:   "rename",
			doc:    bson.M{"_id": int32(1), "renameSource": "value"},
			update: bson.M{"$rename": bson.M{"renameSource": "renameTarget"}},
			assert: func(t *testing.T, doc bson.M) {
				if doc["renameTarget"] != "value" {
					t.Fatalf("renameTarget = %v, want value", doc["renameTarget"])
				}
			},
		},
		{
			name:   "currentDate",
			doc:    bson.M{"_id": int32(1)},
			update: bson.M{"$currentDate": bson.M{"touchedAt": true}},
			assert: func(t *testing.T, doc bson.M) {
				if _, ok := doc["touchedAt"].(bson.DateTime); !ok {
					t.Fatalf("touchedAt has type %T, want bson.DateTime", doc["touchedAt"])
				}
			},
		},
		{
			name:   "addToSet",
			doc:    bson.M{"_id": int32(1), "tags": bson.A{"keep-me"}},
			update: bson.M{"$addToSet": bson.M{"tags": "deduped"}},
			assert: func(t *testing.T, doc bson.M) {
				if len(doc["tags"].(bson.A)) != 2 {
					t.Fatalf("tags = %#v, want 2 values", doc["tags"])
				}
			},
		},
		{
			name:   "push",
			doc:    bson.M{"_id": int32(1), "scores": bson.A{int32(1), int32(2)}},
			update: bson.M{"$push": bson.M{"scores": bson.M{"$each": bson.A{int32(5), int32(3)}, "$sort": int32(-1), "$slice": int32(3)}}},
			assert: func(t *testing.T, doc bson.M) {
				if len(doc["scores"].(bson.A)) != 3 {
					t.Fatalf("scores = %#v, want 3 values", doc["scores"])
				}
			},
		},
		{
			name:   "pull",
			doc:    bson.M{"_id": int32(1), "tags": bson.A{"remove-me", "keep-me"}},
			update: bson.M{"$pull": bson.M{"tags": "remove-me"}},
			assert: func(t *testing.T, doc bson.M) {
				if len(doc["tags"].(bson.A)) != 1 {
					t.Fatalf("tags = %#v, want 1 value", doc["tags"])
				}
			},
		},
		{
			name:   "pop",
			doc:    bson.M{"_id": int32(1), "scores": bson.A{int32(1), int32(2)}},
			update: bson.M{"$pop": bson.M{"scores": int32(1)}},
			assert: func(t *testing.T, doc bson.M) {
				if len(doc["scores"].(bson.A)) != 1 {
					t.Fatalf("scores = %#v, want 1 value", doc["scores"])
				}
			},
		},
		{
			name:   "bit",
			doc:    bson.M{"_id": int32(1), "flags": int32(1)},
			update: bson.M{"$bit": bson.M{"flags": bson.M{"or": int32(2)}}},
			assert: func(t *testing.T, doc bson.M) {
				if doc["flags"] != int32(3) {
					t.Fatalf("flags = %v, want 3", doc["flags"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ApplyUpdate(tt.doc, tt.update); err != nil {
				t.Fatal(err)
			}
			tt.assert(t, tt.doc)
		})
	}

	doc := bson.M{"_id": int32(1), "score": int32(5)}
	if err := ApplyUpdate(doc, bson.M{"kind": "replacement"}); err != nil {
		t.Fatal(err)
	}
	if doc["_id"] != int32(1) || doc["kind"] != "replacement" {
		t.Fatalf("replacement doc = %#v, want preserved _id and kind", doc)
	}
}

func TestValidateJSONSchemaRequiredEnumAndTypes(t *testing.T) {
	validator := bson.M{"$jsonSchema": bson.M{
		"required": bson.A{"orderNo", "status", "lines"},
		"properties": bson.M{
			"orderNo": bson.M{"bsonType": "string"},
			"status":  bson.M{"enum": bson.A{"new", "paid"}},
			"lines":   bson.M{"bsonType": "array"},
		},
	}}

	if err := ValidateJSONSchema(validator, bson.M{"orderNo": "A", "status": "paid", "lines": bson.A{}}); err != nil {
		t.Fatalf("ValidateJSONSchema returned error: %v", err)
	}
	if err := ValidateJSONSchema(validator, bson.M{"orderNo": "A"}); err == nil {
		t.Fatal("ValidateJSONSchema accepted missing required fields")
	}
	if err := ValidateJSONSchema(validator, bson.M{"orderNo": "A", "status": "bad", "lines": bson.A{}}); err == nil {
		t.Fatal("ValidateJSONSchema accepted invalid enum")
	}
	if err := ValidateJSONSchema(validator, bson.M{"orderNo": "A", "status": "paid", "lines": "not-array"}); err == nil {
		t.Fatal("ValidateJSONSchema accepted invalid array type")
	}
}
