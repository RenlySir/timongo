package tidb

import (
	"fmt"
	"regexp"
	"strings"
)

var unsafeIdent = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// PhysicalTableName returns a TiDB-safe enterprise document table name.
func PhysicalTableName(dbName, collection string) string {
	base := "tm_doc_" + dbName + "_" + collection
	base = unsafeIdent.ReplaceAllString(base, "_")
	base = strings.Trim(base, "_")
	if base == "" || base == "tm_doc" {
		return "tm_doc_collection"
	}
	return strings.ToLower(base)
}

// DocumentTableDDL returns the enterprise document-table DDL for one collection.
func DocumentTableDDL(table string) string {
	return fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS `%s` ("+
			"`id_key` VARBINARY(768) NOT NULL,"+
			"`id_bson` BLOB NOT NULL,"+
			"`doc_bson` LONGBLOB NOT NULL,"+
			"`doc_json` JSON NOT NULL,"+
			"`revision` BIGINT NOT NULL,"+
			"`created_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),"+
			"`updated_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),"+
			"PRIMARY KEY (`id_key`)"+
			")",
		table,
	)
}
