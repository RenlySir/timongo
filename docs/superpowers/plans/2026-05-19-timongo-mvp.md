# timongo MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a minimal Go MongoDB-compatible gateway that handles handshake, ping, buildInfo, insert, and find over MongoDB wire protocol and stores documents in TiDB/MySQL-compatible SQL tables.

**Architecture:** The MVP uses a stateless TCP server with OP_MSG decoding from `github.com/FerretDB/wire`. Command handlers translate a small MongoDB command subset into calls to a storage interface. The TiDB storage implementation stores one JSON document table per collection and supports `_id`-based insert plus simple find filters.

**Tech Stack:** Go, FerretDB wire/BSON package, MySQL/TiDB driver, unit tests, optional Docker/TiDB integration later.

---

## Scope

Implement:

- `timongo serve`
- MongoDB OP_MSG listener
- `hello`, `isMaster`, `ping`, `buildInfo`
- `insert`
- `find` with empty filter, `_id` equality, simple scalar equality
- storage interface
- SQL/TiDB storage implementation
- in-memory storage for tests
- unit tests

Do not implement:

- full MongoDB compatibility
- authentication
- update/delete/aggregate
- TiUP component packaging
- TiFlash routing
- shared cursor state

## Files

- Create: `go.mod`
- Create: `cmd/timongo/main.go`
- Create: `internal/config/config.go`
- Create: `internal/bsonutil/bsonutil.go`
- Create: `internal/bsonutil/bsonutil_test.go`
- Create: `internal/mongoerrors/errors.go`
- Create: `internal/storage/storage.go`
- Create: `internal/storage/memory.go`
- Create: `internal/storage/memory_test.go`
- Create: `internal/storage/tidb.go`
- Create: `internal/wire/server.go`
- Create: `internal/wire/server_test.go`
- Create: `internal/handler/handler.go`
- Create: `internal/handler/handler_test.go`
- Modify: `README.md`

## Tasks

### Task 1: BSON Utility

- [x] Write failing tests for `_id` extraction and JSON normalization.
- [x] Implement BSON utility helpers.
- [x] Run tests.

### Task 2: Storage Interface

- [x] Write failing tests for in-memory insert/find.
- [x] Implement storage interface and in-memory store.
- [x] Run tests.

### Task 3: Command Handler

- [x] Write failing tests for hello, ping, buildInfo, insert, find.
- [x] Implement handler.
- [x] Run tests.

### Task 4: Wire Server

- [x] Write failing tests for command dispatch without real network complexity.
- [x] Implement OP_MSG server loop.
- [x] Run tests.

### Task 5: TiDB Storage

- [x] Write SQL-building tests for table names and query generation.
- [x] Implement TiDB storage.
- [x] Run tests.

### Task 6: CLI and Docs

- [x] Implement `timongo serve`.
- [x] Update README with build/run commands and MVP scope.
- [x] Run `go test ./...`.
- [ ] Commit and push.
