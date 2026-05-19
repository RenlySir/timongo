# timongo Enterprise M0 Architecture Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert the current timongo MVP into the M0 enterprise architecture foundation defined in `docs/superpowers/specs/2026-05-19-timongo-enterprise-design.md`.

**Architecture:** M0 keeps the current FerretDB wire dependency and working command path, but introduces enterprise boundaries: stateless TiDB-only runtime, explicit backend contracts, `_timongo` catalog bootstrap, status readiness checks, TiUP topology rendering, and a compatibility-matrix test scaffold. M0 does not implement the full MongoDB 6.0 surface; it makes the repository ready for M1/M2 feature work without preserving MVP-only runtime assumptions.

**Tech Stack:** Go, `github.com/FerretDB/wire`, MongoDB Go Driver BSON package, `github.com/go-sql-driver/mysql`, TiDB/MySQL SQL, YAML topology files, TOML-style rendered config.

---

## Scope Check

The enterprise specification covers several subsystems. This plan implements only M0:

- runtime must be stateless and TiDB-backed
- memory backend must be unavailable to `timongo serve`
- FerretDB wire reuse remains isolated behind `internal/wire`
- command handling gets a registry boundary
- TiDB backend gets catalog bootstrap and document-table schema helpers
- status endpoints expose process and TiDB readiness
- TiUP topology parsing/rendering exists for `timongo_servers`
- compatibility matrix and differential-test directories exist

Full MongoDB CRUD breadth, indexes, auth, transactions, aggregation, TLS, audit, HAProxy orchestration, and TiFlash acceleration are later plans.

## File Structure

Create or modify these files:

- Modify `cmd/timongo/main.go`: reject non-TiDB runtime, wire status server, call catalog bootstrap.
- Keep `cmd/timongo/main_test.go`: stateless runtime tests already started; extend if needed.
- Modify `internal/config/config.go`: define stateless defaults, status address, compatibility version, TiDB config.
- Keep `internal/config/config_test.go`: default config tests already started; extend if needed.
- Create `internal/backend/backend.go`: stable backend contract used by command handlers.
- Create `internal/backend/memory/memory.go`: test-only in-memory backend.
- Create `internal/backend/tidb/store.go`: TiDB-backed store implementation.
- Create `internal/backend/tidb/schema.go`: document table DDL and namespace helpers.
- Create `internal/backend/tidb/schema_test.go`: DDL and namespace tests.
- Modify or remove `internal/storage/*.go`: replace MVP storage package with backend packages. Use `git mv` when moving code.
- Create `internal/catalog/catalog.go`: `_timongo` system schema bootstrap.
- Create `internal/catalog/catalog_test.go`: catalog bootstrap statement tests.
- Create `internal/command/registry.go`: command registry boundary.
- Create `internal/command/registry_test.go`: command routing tests.
- Modify `internal/handler/handler.go`: delegate command-name dispatch through `internal/command`.
- Create `internal/status/server.go`: `/health`, `/ready`, `/metrics` HTTP status server.
- Create `internal/status/server_test.go`: health and readiness tests.
- Create `internal/tiup/topology.go`: parse and validate `timongo_servers` topology.
- Create `internal/tiup/topology_test.go`: topology validation tests.
- Create `tiup/templates/timongo.toml.tmpl`: rendered timongo config template.
- Create `tiup/templates/timongo.service.tmpl`: systemd unit template.
- Create `tiup/templates/haproxy.cfg.tmpl`: optional HAProxy template.
- Create `tiup/examples/topology.yaml`: sample topology.
- Create `docs/compatibility/mongodb-6.0-matrix.md`: initial compatibility matrix.
- Create `tests/compat/README.md`: compatibility-test harness documentation.
- Modify `README.md`: describe enterprise M0 direction and TiDB-only runtime.

## Task 1: Lock Runtime To Stateless TiDB

**Files:**
- Modify: `internal/config/config.go`
- Modify: `cmd/timongo/main.go`
- Test: `internal/config/config_test.go`
- Test: `cmd/timongo/main_test.go`

- [ ] **Step 1: Write the failing config test**

Use this exact test in `internal/config/config_test.go`:

```go
package config

import "testing"

func TestDefaultConfigUsesStatelessTiDBBackend(t *testing.T) {
	cfg := Default()
	if cfg.Backend != BackendTiDB {
		t.Fatalf("Backend = %q, want %q", cfg.Backend, BackendTiDB)
	}
	if cfg.ListenAddr == "" {
		t.Fatal("ListenAddr is empty")
	}
	if cfg.StatusAddr == "" {
		t.Fatal("StatusAddr is empty")
	}
	if cfg.CompatVersion != "6.0" {
		t.Fatalf("CompatVersion = %q, want 6.0", cfg.CompatVersion)
	}
}
```

- [ ] **Step 2: Write the failing runtime tests**

Use this exact test content in `cmd/timongo/main_test.go`:

```go
package main

import (
	"strings"
	"testing"

	"github.com/RenlySir/timongo/internal/config"
)

func TestBuildStoreRejectsMemoryBackend(t *testing.T) {
	_, _, err := buildStore(config.Config{Backend: "memory"})
	if err == nil {
		t.Fatal("buildStore returned nil error for memory backend")
	}
	if !strings.Contains(err.Error(), "stateless") {
		t.Fatalf("error = %q, want mention of stateless runtime", err.Error())
	}
}

func TestBuildStoreRequiresTiDBDSN(t *testing.T) {
	_, _, err := buildStore(config.Config{Backend: config.BackendTiDB})
	if err == nil {
		t.Fatal("buildStore returned nil error without TiDB DSN")
	}
}
```

- [ ] **Step 3: Run tests to verify failure**

Run:

```bash
go test ./cmd/timongo ./internal/config
```

Expected before implementation: failure because `StatusAddr` and `CompatVersion` are not present or because memory backend is still accepted.

- [ ] **Step 4: Implement config defaults**

Replace `internal/config/config.go` with:

```go
package config

import "time"

const (
	// BackendTiDB is the only runtime backend. It keeps timongo stateless by storing all data in TiDB.
	BackendTiDB = "tidb"
)

// Config contains timongo runtime settings.
type Config struct {
	ListenAddr    string
	StatusAddr    string
	Backend       string
	TiDBDSN       string
	CompatVersion string
	Timeout       time.Duration
}

// Default returns production-safe local defaults.
func Default() Config {
	return Config{
		ListenAddr:    "127.0.0.1:27017",
		StatusAddr:    "127.0.0.1:28017",
		Backend:       BackendTiDB,
		CompatVersion: "6.0",
		Timeout:       30 * time.Second,
	}
}
```

- [ ] **Step 5: Reject memory backend in runtime**

In `cmd/timongo/main.go`, change the backend flag help text:

```go
fs.StringVar(&cfg.Backend, "backend", cfg.Backend, "storage backend: tidb")
fs.StringVar(&cfg.StatusAddr, "status-listen", cfg.StatusAddr, "HTTP health and metrics listen address")
```

Replace `buildStore` with:

```go
func buildStore(cfg config.Config) (storage.Store, func(), error) {
	switch cfg.Backend {
	case config.BackendTiDB:
		if cfg.TiDBDSN == "" {
			return nil, nil, fmt.Errorf("-tidb-dsn is required when -backend=tidb")
		}
		store, err := storage.NewTiDBStore(cfg.TiDBDSN)
		if err != nil {
			return nil, nil, err
		}
		return store, func() { _ = store.Close() }, nil
	case "memory":
		return nil, nil, fmt.Errorf("memory backend is test-only; timongo runtime must be stateless and use tidb")
	default:
		return nil, nil, fmt.Errorf("unsupported backend %q", cfg.Backend)
	}
}
```

- [ ] **Step 6: Run tests to verify pass**

Run:

```bash
go test ./cmd/timongo ./internal/config
```

Expected: pass.

- [ ] **Step 7: Commit**

Run:

```bash
git add cmd/timongo/main.go cmd/timongo/main_test.go internal/config/config.go internal/config/config_test.go
git commit -m "feat: enforce stateless tidb runtime"
```

## Task 2: Introduce Backend Package Boundary

**Files:**
- Create: `internal/backend/backend.go`
- Create: `internal/backend/memory/memory.go`
- Create: `internal/backend/memory/memory_test.go`
- Modify: `internal/handler/handler.go`
- Modify: `internal/handler/handler_test.go`
- Modify: `cmd/timongo/main.go`

- [ ] **Step 1: Create backend contract**

Create `internal/backend/backend.go`:

```go
package backend

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Store is the durable backend used by MongoDB command handlers.
type Store interface {
	Insert(ctx context.Context, db, collection string, docs []bson.M) (InsertResult, error)
	Find(ctx context.Context, db, collection string, req FindRequest) (FindResult, error)
}

// InsertResult describes an insert command result.
type InsertResult struct {
	Inserted int
}

// FindRequest describes the supported M0 find subset.
type FindRequest struct {
	Filter bson.M
	Limit  int64
}

// FindResult describes a find command result.
type FindResult struct {
	Documents []bson.M
}
```

- [ ] **Step 2: Move memory backend into test-only package**

Use `git mv internal/storage/memory.go internal/backend/memory/memory.go` and edit the moved file so its package is `memory` and imports `internal/backend`.

The constructor must be:

```go
package memory

import (
	"context"
	"sync"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/RenlySir/timongo/internal/backend"
)

type Store struct {
	mu   sync.RWMutex
	data map[string][]bson.M
}

func NewStore() *Store {
	return &Store{data: make(map[string][]bson.M)}
}

func (s *Store) Insert(ctx context.Context, db, collection string, docs []bson.M) (backend.InsertResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := db + "." + collection
	for _, doc := range docs {
		clone := bson.M{}
		for k, v := range doc {
			clone[k] = v
		}
		s.data[key] = append(s.data[key], clone)
	}

	return backend.InsertResult{Inserted: len(docs)}, nil
}

func (s *Store) Find(ctx context.Context, db, collection string, req backend.FindRequest) (backend.FindResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := db + "." + collection
	var out []bson.M
	for _, doc := range s.data[key] {
		if !matchesFilter(doc, req.Filter) {
			continue
		}
		clone := bson.M{}
		for k, v := range doc {
			clone[k] = v
		}
		out = append(out, clone)
		if req.Limit > 0 && int64(len(out)) >= req.Limit {
			break
		}
	}

	return backend.FindResult{Documents: out}, nil
}

func matchesFilter(doc bson.M, filter bson.M) bool {
	for k, want := range filter {
		if got, ok := doc[k]; !ok || got != want {
			return false
		}
	}
	return true
}
```

- [ ] **Step 3: Update handler imports**

In `internal/handler/handler.go`, replace the storage import with:

```go
"github.com/RenlySir/timongo/internal/backend"
```

Change the handler field and constructor:

```go
type Handler struct {
	store backend.Store
}

func New(store backend.Store) *Handler {
	return &Handler{store: store}
}
```

Change the find call:

```go
res, err := h.store.Find(ctx, db, coll, backend.FindRequest{Filter: filter, Limit: limit})
```

- [ ] **Step 4: Update tests to use memory backend package**

In `internal/handler/handler_test.go`, import:

```go
"github.com/RenlySir/timongo/internal/backend/memory"
```

Construct the handler with:

```go
h := New(memory.NewStore())
```

- [ ] **Step 5: Run tests**

Run:

```bash
go test ./internal/backend/... ./internal/handler
```

Expected: pass.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/backend internal/handler cmd/timongo internal/storage
git commit -m "refactor: introduce backend contract"
```

## Task 3: Move TiDB Backend And Add Enterprise Document Schema Helpers

**Files:**
- Create: `internal/backend/tidb/store.go`
- Create: `internal/backend/tidb/schema.go`
- Create: `internal/backend/tidb/schema_test.go`
- Modify: `cmd/timongo/main.go`
- Remove after move: `internal/storage/tidb.go`
- Remove after move: `internal/storage/tidb_test.go`

- [ ] **Step 1: Write schema tests**

Create `internal/backend/tidb/schema_test.go`:

```go
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
```

- [ ] **Step 2: Run schema tests to verify failure**

Run:

```bash
go test ./internal/backend/tidb
```

Expected before implementation: failure because package or functions do not exist.

- [ ] **Step 3: Implement schema helpers**

Create `internal/backend/tidb/schema.go`:

```go
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
```

- [ ] **Step 4: Move TiDB store**

Use `git mv internal/storage/tidb.go internal/backend/tidb/store.go` and edit package/imports:

```go
package tidb

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/RenlySir/timongo/internal/backend"
	"github.com/RenlySir/timongo/internal/bsonutil"
)
```

The constructor remains:

```go
func NewStore(dsn string) (*Store, error)
```

The store type is:

```go
type Store struct {
	db *sql.DB
}
```

Use `backend.InsertResult`, `backend.FindRequest`, and `backend.FindResult` in method signatures.

- [ ] **Step 5: Use enterprise columns in TiDB store**

In `internal/backend/tidb/store.go`, use `DocumentTableDDL(table)` in `ensureCollection`.

For M0, store Extended JSON in both `doc_bson` and `doc_json` until canonical BSON storage is implemented in M1. Make the limitation explicit in a code comment:

```go
// M0 stores Extended JSON in doc_bson as a compatibility scaffold.
// M1 replaces this with canonical BSON bytes.
```

The insert statement in M0 should be:

```go
stmt := fmt.Sprintf(
	"INSERT INTO `%s` (`id_key`, `id_bson`, `doc_bson`, `doc_json`, `revision`) VALUES (?, ?, ?, CAST(? AS JSON), 1)",
	table,
)
```

The find select in M0 should read from `doc_json`:

```go
query := fmt.Sprintf("SELECT JSON_PRETTY(doc_json) FROM `%s`", table)
```

- [ ] **Step 6: Update main to use TiDB backend package**

In `cmd/timongo/main.go`, import:

```go
"github.com/RenlySir/timongo/internal/backend"
tidbbackend "github.com/RenlySir/timongo/internal/backend/tidb"
```

Change `buildStore` return type:

```go
func buildStore(cfg config.Config) (backend.Store, func(), error)
```

Create store with:

```go
store, err := tidbbackend.NewStore(cfg.TiDBDSN)
```

- [ ] **Step 7: Run tests**

Run:

```bash
go test ./internal/backend/... ./cmd/timongo ./internal/handler
```

Expected: pass.

- [ ] **Step 8: Commit**

Run:

```bash
git add internal/backend cmd/timongo internal/handler internal/storage
git commit -m "refactor: move tidb backend behind enterprise schema"
```

## Task 4: Add `_timongo` Catalog Bootstrap

**Files:**
- Create: `internal/catalog/catalog.go`
- Create: `internal/catalog/catalog_test.go`
- Modify: `internal/backend/tidb/store.go`

- [ ] **Step 1: Write catalog tests**

Create `internal/catalog/catalog_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify failure**

Run:

```bash
go test ./internal/catalog
```

Expected before implementation: failure because package or functions do not exist.

- [ ] **Step 3: Implement catalog bootstrap statements**

Create `internal/catalog/catalog.go`:

```go
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
```

- [ ] **Step 4: Call catalog bootstrap from TiDB store**

In `internal/backend/tidb/store.go`, import:

```go
"github.com/RenlySir/timongo/internal/catalog"
```

In `NewStore`, after `sql.Open`, ping and bootstrap:

```go
if err := db.Ping(); err != nil {
	_ = db.Close()
	return nil, err
}
if err := catalog.Bootstrap(context.Background(), db); err != nil {
	_ = db.Close()
	return nil, err
}
```

- [ ] **Step 5: Run tests**

Run:

```bash
go test ./internal/catalog ./internal/backend/tidb
```

Expected: pass.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/catalog internal/backend/tidb
git commit -m "feat: add timongo catalog bootstrap"
```

## Task 5: Add Command Registry Boundary

**Files:**
- Create: `internal/command/registry.go`
- Create: `internal/command/registry_test.go`
- Modify: `internal/handler/handler.go`

- [ ] **Step 1: Write registry tests**

Create `internal/command/registry_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/command
```

Expected before implementation: failure because package does not exist.

- [ ] **Step 3: Implement registry**

Create `internal/command/registry.go`:

```go
package command

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/RenlySir/timongo/internal/mongoerrors"
)

type Handler interface {
	Handle(ctx context.Context, cmd bson.M) (bson.M, error)
}

type HandlerFunc func(ctx context.Context, cmd bson.M) (bson.M, error)

func (f HandlerFunc) Handle(ctx context.Context, cmd bson.M) (bson.M, error) {
	return f(ctx, cmd)
}

type Registry struct {
	handlers map[string]Handler
}

func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

func (r *Registry) Register(name string, h Handler) {
	r.handlers[name] = h
}

func (r *Registry) Handle(ctx context.Context, cmd bson.M) (bson.M, error) {
	name, _, err := Name(cmd)
	if err != nil {
		return nil, err
	}
	h, ok := r.handlers[name]
	if !ok {
		return nil, mongoerrors.New(mongoerrors.CodeCommandNotFound, "CommandNotFound", "no such command: %s", name)
	}
	return h.Handle(ctx, cmd)
}

func Name(cmd bson.M) (string, any, error) {
	for key, value := range cmd {
		if len(key) > 0 && key[0] != '$' {
			return key, value, nil
		}
	}
	return "", nil, mongoerrors.New(mongoerrors.CodeBadValue, "BadValue", "empty command")
}
```

- [ ] **Step 4: Adapt handler gradually**

Keep existing handler methods, but replace the hard-coded `commandName` helper with `command.Name(cmd)` so command-name parsing is centralized.

In `internal/handler/handler.go`, import:

```go
"github.com/RenlySir/timongo/internal/command"
```

Change:

```go
name, value, err := commandName(cmd)
```

to:

```go
name, value, err := command.Name(cmd)
```

Remove the old `commandName` function from `internal/handler/handler.go`.

- [ ] **Step 5: Run tests**

Run:

```bash
go test ./internal/command ./internal/handler ./internal/wire
```

Expected: pass.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/command internal/handler internal/wire
git commit -m "refactor: add command registry boundary"
```

## Task 6: Add Status Health And Readiness Server

**Files:**
- Create: `internal/status/server.go`
- Create: `internal/status/server_test.go`
- Modify: `cmd/timongo/main.go`

- [ ] **Step 1: Write status tests**

Create `internal/status/server_test.go`:

```go
package status

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthReturnsOK(t *testing.T) {
	h := NewHandler(func() error { return nil })
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestReadyReturnsServiceUnavailableWhenCheckFails(t *testing.T) {
	h := NewHandler(func() error { return errNotReadyForTest{} })
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
}

type errNotReadyForTest struct{}

func (errNotReadyForTest) Error() string {
	return "not ready"
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/status
```

Expected before implementation: failure because package does not exist.

- [ ] **Step 3: Implement status handler**

Create `internal/status/server.go`:

```go
package status

import (
	"context"
	"fmt"
	"net/http"
)

type ReadyCheck func() error

func NewHandler(ready ReadyCheck) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := ready(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, `{"ok":false,"error":%q}`, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("# timongo metrics are initialized in M1\n"))
	})
	return mux
}

func ListenAndServe(ctx context.Context, addr string, handler http.Handler) error {
	srv := &http.Server{Addr: addr, Handler: handler}
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()
	err := srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
```

- [ ] **Step 4: Add a readiness method to TiDB store**

In `internal/backend/tidb/store.go`, add:

```go
func (s *Store) Ready() error {
	return s.db.Ping()
}
```

- [ ] **Step 5: Start status server from main**

In `cmd/timongo/main.go`, import:

```go
"github.com/RenlySir/timongo/internal/status"
```

After creating the store and context, start:

```go
ready := func() error { return nil }
if r, ok := store.(interface{ Ready() error }); ok {
	ready = r.Ready
}
go func() {
	if err := status.ListenAndServe(ctx, cfg.StatusAddr, status.NewHandler(ready)); err != nil {
		log.Printf("status server stopped: %v", err)
	}
}()
```

- [ ] **Step 6: Run tests**

Run:

```bash
go test ./internal/status ./cmd/timongo ./internal/backend/...
```

Expected: pass.

- [ ] **Step 7: Commit**

Run:

```bash
git add internal/status internal/backend/tidb cmd/timongo internal/config
git commit -m "feat: add status readiness endpoints"
```

## Task 7: Add TiUP Topology Parser And Templates

**Files:**
- Create: `internal/tiup/topology.go`
- Create: `internal/tiup/topology_test.go`
- Create: `tiup/templates/timongo.toml.tmpl`
- Create: `tiup/templates/timongo.service.tmpl`
- Create: `tiup/templates/haproxy.cfg.tmpl`
- Create: `tiup/examples/topology.yaml`
- Modify: `go.mod`

- [ ] **Step 1: Add YAML dependency**

Run:

```bash
go get gopkg.in/yaml.v3
```

Expected: `go.mod` and `go.sum` update.

- [ ] **Step 2: Write topology tests**

Create `internal/tiup/topology_test.go`:

```go
package tiup

import "testing"

func TestParseTopologyRequiresTimongoServerBinding(t *testing.T) {
	raw := []byte(`
timongo_servers:
  - host: 10.0.1.10
    port: 27017
    status_port: 28017
`)
	_, err := ParseTopology(raw)
	if err == nil {
		t.Fatal("expected missing tidb binding error")
	}
}

func TestParseTopologyAcceptsBoundTimongoServer(t *testing.T) {
	raw := []byte(`
timongo_servers:
  - host: 10.0.1.10
    port: 27017
    status_port: 28017
    tidb_host: 10.0.1.20
    tidb_port: 4000
    config:
      compat_version: "6.0"
      stateless: true
`)
	topo, err := ParseTopology(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(topo.TimongoServers) != 1 {
		t.Fatalf("timongo servers = %d", len(topo.TimongoServers))
	}
	if topo.TimongoServers[0].TiDBHost != "10.0.1.20" {
		t.Fatalf("TiDBHost = %q", topo.TimongoServers[0].TiDBHost)
	}
}
```

- [ ] **Step 3: Implement topology parsing**

Create `internal/tiup/topology.go`:

```go
package tiup

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type Topology struct {
	TimongoServers   []TimongoServer   `yaml:"timongo_servers"`
	TimongoLBServers []TimongoLBServer `yaml:"timongo_lb_servers"`
}

type TimongoServer struct {
	Host       string        `yaml:"host"`
	Port       int           `yaml:"port"`
	StatusPort int           `yaml:"status_port"`
	DeployDir  string        `yaml:"deploy_dir"`
	LogDir     string        `yaml:"log_dir"`
	TiDBHost   string        `yaml:"tidb_host"`
	TiDBPort   int           `yaml:"tidb_port"`
	Config     TimongoConfig `yaml:"config"`
}

type TimongoConfig struct {
	CompatVersion      string `yaml:"compat_version"`
	Stateless          bool   `yaml:"stateless"`
	CursorTTL          string `yaml:"cursor_ttl"`
	SessionTTL         string `yaml:"session_ttl"`
	TransactionTimeout string `yaml:"transaction_timeout"`
	MaxConnections     int    `yaml:"max_connections"`
	TiDBMaxOpenConns   int    `yaml:"tidb_max_open_conns"`
}

type TimongoLBServer struct {
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	StatusPort  int    `yaml:"status_port"`
	Balance     string `yaml:"balance"`
	BackendRole string `yaml:"backend_role"`
}

func ParseTopology(raw []byte) (*Topology, error) {
	var topo Topology
	if err := yaml.Unmarshal(raw, &topo); err != nil {
		return nil, err
	}
	for i, server := range topo.TimongoServers {
		if server.Host == "" {
			return nil, fmt.Errorf("timongo_servers[%d].host is required", i)
		}
		if server.Port == 0 {
			return nil, fmt.Errorf("timongo_servers[%d].port is required", i)
		}
		if server.StatusPort == 0 {
			return nil, fmt.Errorf("timongo_servers[%d].status_port is required", i)
		}
		if server.TiDBHost == "" {
			return nil, fmt.Errorf("timongo_servers[%d].tidb_host is required", i)
		}
		if server.TiDBPort == 0 {
			return nil, fmt.Errorf("timongo_servers[%d].tidb_port is required", i)
		}
	}
	return &topo, nil
}
```

- [ ] **Step 4: Add templates**

Create `tiup/templates/timongo.toml.tmpl`:

```toml
[mongo]
listen_addr = "0.0.0.0:{{ .Port }}"
compat_version = "{{ .Config.CompatVersion }}"

[status]
listen_addr = "0.0.0.0:{{ .StatusPort }}"

[tidb]
host = "{{ .TiDBHost }}"
port = {{ .TiDBPort }}
database = "_timongo"
max_open_conns = {{ .Config.TiDBMaxOpenConns }}

[runtime]
stateless = true
cursor_ttl = "{{ .Config.CursorTTL }}"
session_ttl = "{{ .Config.SessionTTL }}"
transaction_timeout = "{{ .Config.TransactionTimeout }}"
```

Create `tiup/templates/timongo.service.tmpl`:

```ini
[Unit]
Description=timongo-server
After=network-online.target
Wants=network-online.target

[Service]
User={{ .User }}
ExecStart={{ .DeployDir }}/bin/timongo serve -config {{ .DeployDir }}/conf/timongo.toml
Restart=always
RestartSec=5
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
```

Create `tiup/templates/haproxy.cfg.tmpl`:

```text
frontend timongo_mongo
    bind *:{{ .Port }}
    mode tcp
    default_backend timongo_servers

backend timongo_servers
    mode tcp
    balance {{ .Balance }}
{{ range .Backends }}
    server {{ .Name }} {{ .Host }}:{{ .Port }} check port {{ .StatusPort }}
{{ end }}
```

- [ ] **Step 5: Add example topology**

Create `tiup/examples/topology.yaml`:

```yaml
global:
  user: tidb
  deploy_dir: /tidb-deploy
  data_dir: /tidb-data

timongo_servers:
  - host: 10.0.1.10
    port: 27017
    status_port: 28017
    deploy_dir: /tidb-deploy/timongo-27017
    log_dir: /tidb-deploy/timongo-27017/log
    tidb_host: 10.0.1.20
    tidb_port: 4000
    config:
      compat_version: "6.0"
      stateless: true
      cursor_ttl: "10m"
      session_ttl: "30m"
      transaction_timeout: "60s"
      max_connections: 20000
      tidb_max_open_conns: 4096

timongo_lb_servers:
  - host: 10.0.1.5
    port: 27017
    status_port: 28018
    balance: leastconn
    backend_role: timongo
```

- [ ] **Step 6: Run tests**

Run:

```bash
go test ./internal/tiup
```

Expected: pass.

- [ ] **Step 7: Commit**

Run:

```bash
git add go.mod go.sum internal/tiup tiup
git commit -m "feat: add timongo tiup topology foundation"
```

## Task 8: Add Compatibility Matrix And Test Harness Scaffold

**Files:**
- Create: `docs/compatibility/mongodb-6.0-matrix.md`
- Create: `tests/compat/README.md`
- Create: `tests/compat/docker-compose.yml`

- [ ] **Step 1: Create compatibility matrix**

Create `docs/compatibility/mongodb-6.0-matrix.md`:

```markdown
# MongoDB 6.0 Compatibility Matrix

Status values:

- `supported`: implemented and covered by automated compatibility tests
- `partial`: implemented with documented limitations
- `planned`: accepted scope for a later milestone
- `unsupported`: not supported
- `intentionally-unsupported`: deliberately outside timongo scope

## Wire Protocol

| Feature | Status | Milestone | Notes |
| --- | --- | --- | --- |
| OP_MSG | partial | M0 | Basic command request and response path exists |
| OP_QUERY handshake | partial | M0 | Legacy handshake only |
| Compression | planned | M1 | Negotiation and codecs not implemented |

## Commands

| Command | Status | Milestone | Notes |
| --- | --- | --- | --- |
| hello | partial | M0 | Basic driver handshake |
| isMaster / ismaster | partial | M0 | Legacy alias |
| ping | partial | M0 | Basic response |
| buildInfo | partial | M0 | timongo metadata only |
| insert | partial | M0 | M0 schema scaffold, M1 correctness work remains |
| find | partial | M0 | Basic equality subset |
| update | planned | M1 | Not implemented |
| delete | planned | M1 | Not implemented |
| getMore | planned | M2 | Requires cursor persistence |
| killCursors | planned | M2 | Requires cursor persistence |

## Explicit Non-Goals

| Feature | Status | Milestone | Notes |
| --- | --- | --- | --- |
| MongoDB replica set election protocol | intentionally-unsupported | none | TiDB provides HA at storage and SQL layers |
| MongoDB sharding internals | intentionally-unsupported | none | TiDB handles distribution |
| server-side JavaScript | intentionally-unsupported | none | Disabled for safety |
```

- [ ] **Step 2: Create compatibility harness README**

Create `tests/compat/README.md`:

```markdown
# timongo Compatibility Harness

This directory contains the M0 scaffold for differential testing.

The target comparison set is:

1. MongoDB 6.0
2. FerretDB-compatible behavior where reusable under Apache-2.0 terms
3. timongo connected to TiDB

M0 only establishes the harness location and local services. M1 adds executable driver tests for CRUD and command compatibility.

The harness must not copy MongoDB server SSPL source code into this repository.
```

- [ ] **Step 3: Create local service compose file**

Create `tests/compat/docker-compose.yml`:

```yaml
services:
  mongodb60:
    image: mongo:6.0
    ports:
      - "37017:27017"

  tidb:
    image: pingcap/tidb:latest
    command:
      - --store=unistore
    ports:
      - "34000:4000"
```

- [ ] **Step 4: Verify files**

Run:

```bash
test -s docs/compatibility/mongodb-6.0-matrix.md
test -s tests/compat/README.md
test -s tests/compat/docker-compose.yml
```

Expected: all commands exit 0.

- [ ] **Step 5: Commit**

Run:

```bash
git add docs/compatibility tests/compat
git commit -m "docs: add mongodb compatibility matrix scaffold"
```

## Task 9: Update README For Enterprise M0

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update README positioning**

Add this section near the top of `README.md` after the project title:

```markdown
## Enterprise Direction

timongo targets a stateless MongoDB 6.0-compatible gateway for TiDB.

The enterprise architecture is documented in:

- `docs/superpowers/specs/2026-05-19-timongo-enterprise-design.md`
- `docs/superpowers/plans/2026-05-19-timongo-enterprise-m0.md`

Current code is M0 architecture foundation work. It is not a full MongoDB replacement.

Runtime principles:

- `timongo serve` uses TiDB as the durable backend.
- memory storage is test-only.
- every timongo-server is configured to connect to one tidb-server.
- MongoDB client entry load balancing is done by HAProxy, cloud NLB, or another TCP load balancer.
- TiProxy remains a TiDB SQL-layer component and does not proxy MongoDB wire protocol.
```

- [ ] **Step 2: Replace memory backend examples**

Remove any README command that starts `timongo serve -backend memory`.

Use this local command instead:

```bash
go run ./cmd/timongo serve \
  -listen 127.0.0.1:27017 \
  -status-listen 127.0.0.1:28017 \
  -backend tidb \
  -tidb-dsn 'root@tcp(127.0.0.1:4000)/test?parseTime=true'
```

- [ ] **Step 3: Verify README does not promote memory runtime**

Run:

```bash
rg -n "backend memory|memory backend|test-only" README.md
```

Expected: no `backend memory` match; `memory storage is test-only` may remain.

- [ ] **Step 4: Commit**

Run:

```bash
git add README.md
git commit -m "docs: document enterprise m0 runtime direction"
```

## Task 10: Full Verification And Push

**Files:**
- All files changed by Tasks 1-9.

- [ ] **Step 1: Run full test suite**

Run:

```bash
go test ./...
```

Expected: pass.

- [ ] **Step 2: Run documentation sanity checks**

Run:

```bash
rg -n "TBD|TODO|FIXME" README.md docs tests tiup internal cmd
```

Expected: no matches.

Run:

```bash
rg -n "backend memory|MongoDB Driver -> TiProxy|copy MongoDB server" README.md docs tests tiup internal cmd
```

Expected: no match for `backend memory` or `MongoDB Driver -> TiProxy`; any MongoDB server mention must say not to copy SSPL production code.

- [ ] **Step 3: Inspect git status**

Run:

```bash
git status --short
```

Expected: clean working tree.

- [ ] **Step 4: Push**

Run:

```bash
git push origin main
```

Expected: push succeeds.

## Self-Review Checklist

- Spec coverage: M0 maps to the accepted enterprise design by introducing stateless runtime, TiDB-only serving, backend boundaries, catalog bootstrap, readiness, TiUP topology, compatibility matrix, and test scaffold.
- Known gaps by design: auth, full CRUD, indexes, sessions, retryable writes, cursor persistence, transactions, aggregation, TLS, audit, and HAProxy lifecycle are not implemented in M0.
- Type consistency: `backend.Store`, `backend.InsertResult`, `backend.FindRequest`, and `backend.FindResult` replace MVP `storage` types before command and TiDB backend changes depend on them.
- Deployment consistency: MongoDB entry load balancing is HAProxy/NLB; TiProxy is not used for MongoDB wire protocol.
- Stateless consistency: `timongo serve` rejects memory backend and requires TiDB DSN.

