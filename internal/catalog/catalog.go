package catalog

import (
	"context"
	"database/sql"
)

// BootstrapStatements returns idempotent DDL for timongo system metadata.
func BootstrapStatements() []string {
	return []string{
		"CREATE DATABASE IF NOT EXISTS `_timongo`",
		"CREATE TABLE IF NOT EXISTS `_timongo`.`databases` (`name` VARBINARY(256) NOT NULL PRIMARY KEY, `created_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6))",
		"CREATE TABLE IF NOT EXISTS `_timongo`.`collections` (`id` BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, `database_name` VARBINARY(256) NOT NULL, `collection_name` VARBINARY(256) NOT NULL, `physical_table` VARCHAR(256) NOT NULL, `schema_version` BIGINT NOT NULL DEFAULT 1, UNIQUE KEY `uk_namespace` (`database_name`, `collection_name`))",
		"CREATE TABLE IF NOT EXISTS `_timongo`.`indexes` (`id` BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, `collection_id` BIGINT NOT NULL, `name` VARCHAR(128) NOT NULL, `definition_json` JSON NOT NULL, `state` VARCHAR(32) NOT NULL, UNIQUE KEY `uk_collection_index` (`collection_id`, `name`))",
		"CREATE TABLE IF NOT EXISTS `_timongo`.`users` (`user_name` VARBINARY(256) NOT NULL, `database_name` VARBINARY(256) NOT NULL, `credentials_json` JSON NOT NULL, `roles_json` JSON NOT NULL, PRIMARY KEY (`user_name`, `database_name`))",
		"CREATE TABLE IF NOT EXISTS `_timongo`.`roles` (`role_name` VARBINARY(256) NOT NULL, `database_name` VARBINARY(256) NOT NULL, `privileges_json` JSON NOT NULL, PRIMARY KEY (`role_name`, `database_name`))",
		"CREATE TABLE IF NOT EXISTS `_timongo`.`sessions` (`lsid` VARBINARY(256) NOT NULL PRIMARY KEY, `user_name` VARBINARY(256) NOT NULL, `last_used_at` TIMESTAMP(6) NOT NULL, `expires_at` TIMESTAMP(6) NOT NULL)",
		"CREATE TABLE IF NOT EXISTS `_timongo`.`retryable_writes` (`lsid` VARBINARY(256) NOT NULL, `txn_number` BIGINT NOT NULL, `stmt_id` INT NOT NULL, `result_bson` LONGBLOB NOT NULL, `created_at` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), PRIMARY KEY (`lsid`, `txn_number`, `stmt_id`))",
		"CREATE TABLE IF NOT EXISTS `_timongo`.`cursors` (`id` BIGINT NOT NULL PRIMARY KEY, `namespace` VARCHAR(512) NOT NULL, `plan_json` JSON NOT NULL, `continuation_key` VARBINARY(3072) NOT NULL, `expires_at` TIMESTAMP(6) NOT NULL)",
		"CREATE TABLE IF NOT EXISTS `_timongo`.`transactions` (`lsid` VARBINARY(256) NOT NULL, `txn_number` BIGINT NOT NULL, `state` VARCHAR(32) NOT NULL, `owner_instance` VARCHAR(128) NOT NULL, `lease_until` TIMESTAMP(6) NOT NULL, PRIMARY KEY (`lsid`, `txn_number`))",
		"CREATE TABLE IF NOT EXISTS `_timongo`.`instances` (`id` VARCHAR(128) NOT NULL PRIMARY KEY, `host` VARCHAR(256) NOT NULL, `mongo_port` INT NOT NULL, `tidb_host` VARCHAR(256) NOT NULL, `tidb_port` INT NOT NULL, `last_seen_at` TIMESTAMP(6) NOT NULL)",
		"CREATE TABLE IF NOT EXISTS `_timongo`.`schema_locks` (`name` VARCHAR(128) NOT NULL PRIMARY KEY, `owner_instance` VARCHAR(128) NOT NULL, `lease_until` TIMESTAMP(6) NOT NULL)",
	}
}

// Bootstrap creates timongo system metadata tables.
func Bootstrap(ctx context.Context, db *sql.DB) error {
	for _, stmt := range BootstrapStatements() {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
