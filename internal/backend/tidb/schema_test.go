package tidb

import (
	"strings"
	"testing"
)

func TestPhysicalTableNameIsStableAndSafe(t *testing.T) {
	got := PhysicalTableName("shop-db", "orders.items")
	if got != "tm_doc_shop_db_orders_items" {
		t.Fatalf("PhysicalTableName = %q", got)
	}
}

func TestDocumentTableDDLUsesEnterpriseColumns(t *testing.T) {
	ddl := DocumentTableDDL("tm_doc_shop_orders")
	for _, want := range []string{
		"`id_key` VARBINARY(768) NOT NULL",
		"`id_bson` BLOB NOT NULL",
		"`doc_bson` LONGBLOB NOT NULL",
		"`doc_json` JSON NOT NULL",
		"`revision` BIGINT NOT NULL",
		"PRIMARY KEY (`id_key`)",
	} {
		if !strings.Contains(ddl, want) {
			t.Fatalf("DDL missing %q:\n%s", want, ddl)
		}
	}
}
