package tidb

import (
	"testing"

	"github.com/RenlySir/timongo/internal/backend"
)

func TestPhysicalTableNameIsStableAndSanitized(t *testing.T) {
	got := PhysicalTableName("app-db", "users.events")
	want := "tm_doc_app_db_users_events"
	if got != want {
		t.Fatalf("PhysicalTableName() = %q, want %q", got, want)
	}
}

func TestFindSQLUsesIDKeyForIDFilter(t *testing.T) {
	sql, args, err := BuildFindSQL("app", "users", backend.FindRequest{Filter: mapOf("_id", int32(1)), Limit: 10})
	if err != nil {
		t.Fatalf("BuildFindSQL returned error: %v", err)
	}

	wantSQL := "SELECT JSON_PRETTY(doc_json) FROM `tm_doc_app_users` WHERE `id_key` = ? LIMIT ?"
	if sql != wantSQL {
		t.Fatalf("sql = %q, want %q", sql, wantSQL)
	}

	if len(args) != 2 {
		t.Fatalf("len(args) = %d, want 2", len(args))
	}
	if args[0] != "int32:1" {
		t.Fatalf("args[0] = %v, want int32:1", args[0])
	}
	if args[1] != int64(10) {
		t.Fatalf("args[1] = %v, want 10", args[1])
	}
}

func TestFindSQLUsesJSONExtractForScalarFilter(t *testing.T) {
	sql, args, err := BuildFindSQL("app", "users", backend.FindRequest{Filter: mapOf("name", "Ada")})
	if err != nil {
		t.Fatalf("BuildFindSQL returned error: %v", err)
	}

	wantSQL := "SELECT JSON_PRETTY(doc_json) FROM `tm_doc_app_users`"
	if sql != wantSQL {
		t.Fatalf("sql = %q, want %q", sql, wantSQL)
	}

	if len(args) != 0 {
		t.Fatalf("args = %#v, want empty args", args)
	}
}

func TestFindSQLScansCollectionForPostFilteredComparison(t *testing.T) {
	sql, args, err := BuildFindSQL("app", "orders", backend.FindRequest{
		Filter: mapOf("total", mapOf("$gte", int32(20))),
		Limit:  1,
	})
	if err != nil {
		t.Fatalf("BuildFindSQL returned error: %v", err)
	}

	wantSQL := "SELECT JSON_PRETTY(doc_json) FROM `tm_doc_app_orders`"
	if sql != wantSQL {
		t.Fatalf("sql = %q, want %q", sql, wantSQL)
	}
	if len(args) != 0 {
		t.Fatalf("args = %#v, want empty args", args)
	}
}

func mapOf(k string, v any) map[string]any {
	return map[string]any{k: v}
}
