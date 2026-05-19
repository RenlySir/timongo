package bsonutil

import (
	"encoding/base64"
	"fmt"
	"strconv"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// DocumentIDKey returns a stable key for a MongoDB _id value.
func DocumentIDKey(doc bson.M) (string, error) {
	id, ok := doc["_id"]
	if !ok {
		return "", fmt.Errorf("missing _id")
	}

	return ValueKey(id)
}

// ValueKey returns a stable string key for values used in equality lookups.
func ValueKey(v any) (string, error) {
	switch id := v.(type) {
	case bson.ObjectID:
		return "objectid:" + id.Hex(), nil
	case string:
		return "string:" + id, nil
	case int:
		return "int64:" + strconv.FormatInt(int64(id), 10), nil
	case int32:
		return "int32:" + strconv.FormatInt(int64(id), 10), nil
	case int64:
		return "int64:" + strconv.FormatInt(id, 10), nil
	case float64:
		return "double:" + strconv.FormatFloat(id, 'g', -1, 64), nil
	case []byte:
		return "binary:" + base64.StdEncoding.EncodeToString(id), nil
	default:
		return "", fmt.Errorf("unsupported _id type %T", v)
	}
}

// MarshalExtJSON converts a BSON document to canonical Extended JSON bytes.
func MarshalExtJSON(doc bson.M) ([]byte, error) {
	return bson.MarshalExtJSON(doc, true, false)
}

// UnmarshalExtJSON converts Extended JSON bytes back to a BSON map.
func UnmarshalExtJSON(raw []byte) (bson.M, error) {
	var doc bson.M
	if err := bson.UnmarshalExtJSON(raw, true, &doc); err != nil {
		return nil, err
	}

	normalized, _ := normalizeValue(doc).(bson.M)
	return normalized, nil
}

func normalizeValue(v any) any {
	switch value := v.(type) {
	case bson.D:
		res := make(bson.M, len(value))
		for _, elem := range value {
			res[elem.Key] = normalizeValue(elem.Value)
		}
		return res
	case bson.M:
		res := make(bson.M, len(value))
		for key, item := range value {
			res[key] = normalizeValue(item)
		}
		return res
	case bson.A:
		res := make(bson.A, 0, len(value))
		for _, item := range value {
			res = append(res, normalizeValue(item))
		}
		return res
	case []any:
		res := make(bson.A, 0, len(value))
		for _, item := range value {
			res = append(res, normalizeValue(item))
		}
		return res
	default:
		return value
	}
}
