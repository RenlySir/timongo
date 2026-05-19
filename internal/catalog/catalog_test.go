package catalog

import (
	"strings"
	"testing"
)

func TestBootstrapStatementsContainRequiredSystemTables(t *testing.T) {
	joined := strings.Join(BootstrapStatements(), "\n")
	for _, want := range []string{
		"CREATE DATABASE IF NOT EXISTS `_timongo`",
		"CREATE TABLE IF NOT EXISTS `_timongo`.`databases`",
		"CREATE TABLE IF NOT EXISTS `_timongo`.`collections`",
		"CREATE TABLE IF NOT EXISTS `_timongo`.`indexes`",
		"CREATE TABLE IF NOT EXISTS `_timongo`.`sessions`",
		"CREATE TABLE IF NOT EXISTS `_timongo`.`retryable_writes`",
		"CREATE TABLE IF NOT EXISTS `_timongo`.`cursors`",
		"CREATE TABLE IF NOT EXISTS `_timongo`.`transactions`",
		"CREATE TABLE IF NOT EXISTS `_timongo`.`instances`",
		"CREATE TABLE IF NOT EXISTS `_timongo`.`schema_locks`",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("bootstrap statements missing %q:\n%s", want, joined)
		}
	}
}
