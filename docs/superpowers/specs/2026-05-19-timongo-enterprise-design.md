# timongo Enterprise Design

## Status

Accepted design baseline.

This document replaces the earlier MVP-oriented design direction with an enterprise-grade target architecture for timongo.

## Product Goal

timongo is a stateless MongoDB 6.0-compatible gateway for TiDB.

Applications connect with MongoDB drivers through a MongoDB wire protocol endpoint. timongo translates supported MongoDB commands, queries, transactions, indexes, authentication, and aggregation semantics into TiDB-backed execution while using TiKV, PD, and optionally TiFlash through the TiDB ecosystem.

The product must be deployable through TiUP Cluster with a `timongo_servers` topology section, similar to existing TiDB ecosystem components.

## Core Decisions

1. Reuse FerretDB's protocol, command framework, compatibility-test ideas, and other Apache-2.0-compatible components where practical.
2. Do not copy MongoDB server production code into timongo. MongoDB server behavior may be studied for compatibility, but its SSPL codebase is not the implementation base.
3. Use MongoDB 6.0 as the first enterprise compatibility baseline.
4. Keep timongo-server stateless. All durable metadata, cursor state, session state, retryable-write records, auth data, catalog data, and transaction result records live in TiDB.
5. Each timongo-server connects to exactly one configured tidb-server.
6. MongoDB client entry load balancing uses HAProxy, cloud NLB, or another MongoDB wire-compatible TCP load balancer. TiProxy remains a TiDB SQL-layer component and does not proxy MongoDB wire protocol.
7. TiUP Cluster support is a first-class part of the product, not an external install script.

## Non-Goals

1. Full MongoDB 7.x or 8.x compatibility in the first enterprise release.
2. MongoDB replica set or sharding internal protocol emulation.
3. Direct TiKV writes from timongo.
4. Forking MongoDB server as the production implementation.
5. JavaScript server-side execution.
6. Atlas-specific behavior.
7. Treating TiDB JSON semantics as a complete substitute for BSON semantics.

## Target Architecture

```text
MongoDB Drivers / Apps
  -> HAProxy / NLB on port 27017
  -> timongo-server A -> tidb-server A
  -> timongo-server B -> tidb-server B
  -> timongo-server C -> tidb-server C
  -> TiKV / PD / TiFlash through TiDB
```

timongo-server is a gateway, not a distributed storage engine. It uses TiDB SQL as the only durable write path and lets TiDB/TiKV/PD/TiFlash handle persistence, transactions, scheduling, and analytical acceleration.

### Main Modules

```text
cmd/
  timongo/
  timongoctl/
  timongo-dump/
  timongo-restore/

internal/
  wire/          # FerretDB wire reuse/adaptation
  command/       # MongoDB command registry, command semantics, auth checks
  bsonx/         # canonical BSON types, ordering, codecs
  planner/       # query/update/aggregation planning
  backend/tidb/  # TiDB SQL execution backend
  catalog/       # _timongo metadata and schema versioning
  index/         # MongoDB index encoding and maintenance
  session/       # lsid, retryable writes, transaction state
  cursor/        # stateless cursor persistence
  auth/          # SCRAM, users, roles
  observability/ # metrics, logs, audit
  tiup/          # topology specs and config rendering helpers

tiup/
  components/timongo/
  templates/timongo.toml.tmpl
  templates/timongo.service.tmpl
  templates/haproxy.cfg.tmpl
  examples/topology.yaml
```

## Data Model

timongo uses a mixed physical model:

1. BSON bytes for correctness.
2. JSON projection for TiDB pushdown where semantics are safe.
3. Dedicated index tables for MongoDB index behavior.

Each MongoDB collection maps to a TiDB document table:

```sql
CREATE TABLE tm_doc_<collection_id> (
  id_key VARBINARY(768) NOT NULL PRIMARY KEY,
  id_bson BLOB NOT NULL,
  doc_bson LONGBLOB NOT NULL,
  doc_json JSON NOT NULL,
  revision BIGINT NOT NULL,
  created_at TIMESTAMP(6) NOT NULL,
  updated_at TIMESTAMP(6) NOT NULL
);
```

Field meaning:

- `id_key` is the canonical encoded `_id` value used for primary-key lookup and ordering.
- `id_bson` stores the original `_id` value.
- `doc_bson` stores the canonical BSON document for lossless round trips.
- `doc_json` stores a query-assist representation for safe TiDB JSON pushdown.
- `revision` supports optimistic checks, diagnostics, future change-stream work, and repair tooling.

Canonical indexes use separate TiDB tables:

```sql
CREATE TABLE tm_idx_<index_id> (
  key_prefix VARBINARY(3072) NOT NULL,
  id_key VARBINARY(768) NOT NULL,
  multikey_pos VARBINARY(256) NOT NULL DEFAULT '',
  PRIMARY KEY (key_prefix, id_key, multikey_pos)
);
```

Generated columns, expression indexes, and TiDB JSON functions are allowed as optimization paths, but they are not the only correctness mechanism. MongoDB type ordering, collation, arrays, missing fields, null behavior, multikey indexes, sparse indexes, partial indexes, and unique constraints are controlled by timongo semantics.

## Write Path

1. Decode and validate BSON.
2. Apply command semantics and authorization.
3. Generate canonical `_id` encoding.
4. Generate canonical BSON, query JSON, and index entries.
5. Execute document and index mutations inside one TiDB transaction.
6. Store retryable-write result metadata when the request contains `lsid` and `txnNumber`.
7. Return MongoDB-compatible command replies and error labels.

## Read Path

1. Parse MongoDB filter, sort, projection, collation, and read options.
2. Select an execution plan:
   - index table lookup
   - safe TiDB JSON pushdown
   - TiDB generated-column or expression-index path
   - full scan with timongo semantic filtering
3. Fetch canonical BSON documents.
4. Re-check semantics that cannot be safely delegated to TiDB.
5. Apply projection and BSON-compatible result ordering.
6. Return the first batch and persist cursor state when more results remain.

## MongoDB 6.0 Compatibility Baseline

### P0 Production Requirements

- Wire protocol: `hello`, legacy `isMaster`, `OP_MSG`, driver metadata, connection lifecycle, optional compression.
- CRUD: `insert`, `find`, `update`, `delete`, `findAndModify`, `count`, `distinct`, `getMore`, `killCursors`.
- Query operators: comparison, logical operators, field existence, basic array matching, regex, projection, sort, skip, limit.
- Indexes: `_id`, single-field, compound, unique, sparse, partial, and basic TTL.
- Aggregation: `$match`, `$project`, `$sort`, `$limit`, `$skip`, `$group`, `$unwind`, and basic `$lookup`.
- Transactions: sessions, retryable writes, and multi-document transactions mapped to TiDB transactions.
- Auth: SCRAM-SHA-256, optional SCRAM-SHA-1 compatibility, users, roles, database-level and collection-level authorization.
- Admin commands: `buildInfo`, `serverStatus`, `listDatabases`, `listCollections`, `createIndexes`, `dropIndexes`, `create`, `drop`.
- Observability: Prometheus metrics, structured logs, slow query logs, audit logs.
- TiUP: deploy, scale-out, scale-in, reload, upgrade, display, destroy for `timongo_servers`.

### P1 Enterprise Enhancements

- More aggregation stages and expressions.
- Collation improvements.
- TiFlash aggregation acceleration.
- Change-stream design based on TiDB CDC or a timongo event log.
- Backup and restore tools with MongoDB namespace semantics.
- TLS, mTLS, audit policy controls, and certificate rotation.
- Compatibility certification against MongoDB drivers and FerretDB-derived tests.

### P2 Explicitly Out of Initial Scope

- MongoDB internal sharding protocol.
- MongoDB replica set election protocol.
- JavaScript execution.
- Atlas-specific APIs.
- Complete 7.x/8.x feature parity.

## Stateless Runtime Model

timongo-server persists all required state in TiDB system tables under `_timongo`:

```text
_timongo.databases
_timongo.collections
_timongo.indexes
_timongo.users
_timongo.roles
_timongo.sessions
_timongo.retryable_writes
_timongo.cursors
_timongo.transactions
_timongo.instances
_timongo.schema_locks
```

Allowed local state:

- catalog cache
- index metadata cache
- prepared query plans
- TiDB connection pool
- authentication context for the current TCP connection
- in-flight request data

Required durable state:

- catalog
- indexes
- users and roles
- sessions
- retryable-write result records
- cursor continuation records
- transaction state and committed result records
- schema version and migration locks

If a timongo-server fails, another timongo-server can rebuild durable state from TiDB. Uncommitted active transactions are allowed to roll back.

## Session and Retryable Writes

MongoDB `lsid` maps to `_timongo.sessions`. Session records include user identity, database, last-used timestamp, transaction number, expiration time, and retryable-write metadata.

Retryable writes use `(lsid, txnNumber, stmtId)` as the idempotency key. A successful write stores a compact result record in the same TiDB transaction as the business mutation. A repeated request returns the stored result without applying the mutation again.

## Cursor Design

Cursor records live in `_timongo.cursors` and contain:

- namespace
- plan digest
- filter, projection, sort, batch size
- continuation key
- temporary-result table name when needed
- user and session identity
- expiration time

For simple ordered scans, the continuation key contains the index key and `_id`. For complex sorts, groups, and lookups that cannot be resumed deterministically, timongo materializes results into TiDB temporary result tables and records the cursor location.

`getMore` can be handled by any timongo-server while the cursor record is valid.

## Transaction Design

timongo is stateless, but active TiDB transactions are connection-bound. Transaction behavior follows MongoDB driver transaction pinning expectations.

Flow:

1. Client sends `startTransaction` on a MongoDB TCP connection.
2. timongo starts a TiDB transaction on the bound tidb-server connection.
3. `_timongo.transactions` records `(lsid, txnNumber, state, owner_instance, owner_conn, lease_until)`.
4. Transaction commands must continue on the same TCP connection and timongo-server.
5. `commitTransaction` writes business data and result records, then commits TiDB transaction.
6. Commit retry can be answered by any timongo-server using `_timongo.transactions`.
7. If the timongo process or bound tidb-server fails before commit, the active transaction rolls back and later requests receive MongoDB-compatible transient transaction errors.

This preserves stateless component operation while respecting database transaction reality.

## Deployment Through TiUP

`timongo_servers` is added to TiUP Cluster topology support.

Example:

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
```

Rendered `timongo.toml`:

```toml
[mongo]
listen_addr = "0.0.0.0:27017"
compat_version = "6.0"

[status]
listen_addr = "0.0.0.0:28017"

[tidb]
host = "10.0.1.20"
port = 4000
database = "_timongo"
max_open_conns = 4096

[runtime]
stateless = true
cursor_ttl = "10m"
session_ttl = "30m"
transaction_timeout = "60s"
```

Required TiUP operations:

- `deploy`
- `start`
- `stop`
- `restart`
- `reload`
- `scale-out`
- `scale-in`
- `upgrade`
- `display`
- `destroy`

Upgrade flow:

1. Drain timongo-server from HAProxy/NLB.
2. Stop timongo-server.
3. Replace binary and config.
4. Start timongo-server.
5. Run readiness checks.
6. Add the node back to HAProxy/NLB.

Optional HAProxy management is supported by `timongo_lb_servers`:

```yaml
timongo_lb_servers:
  - host: 10.0.1.5
    port: 27017
    status_port: 28018
    balance: leastconn
    backend_role: timongo
```

Cloud NLB users can skip `timongo_lb_servers` and use the TiUP-generated timongo backend inventory.

## Health Checks

- `GET /health`: process is alive.
- `GET /ready`: bound tidb-server is reachable, `_timongo` system tables are accessible, schema version is compatible, and catalog lease is valid.
- `GET /metrics`: Prometheus metrics.

HAProxy or NLB must use `/ready`, not `/health`, so a failed bound tidb-server removes only the affected timongo-server from service.

## Enterprise Security

Authentication:

- SCRAM-SHA-256 by default.
- SCRAM-SHA-1 as compatibility option.
- Users and roles stored in `_timongo.users` and `_timongo.roles`.
- Built-in roles: `read`, `readWrite`, `dbAdmin`, `userAdmin`, `clusterMonitor`.

TLS:

- MongoDB client to timongo: TLS and mTLS.
- timongo to TiDB: TLS.
- TiUP distributes and reloads certificates.

Secret handling:

- No plaintext passwords in topology files.
- Support file references and environment references.
- Future KMS integration remains possible.

## Observability and Audit

Metrics:

- connection counts
- active requests
- command QPS
- command latency percentiles
- MongoDB error codes and labels
- TiDB SQL latency and retry counts
- transaction conflicts
- cursor/session/transaction counts
- cursor and session GC
- catalog cache hit rate
- index usage, back-table reads, full scans

Logs:

- JSON structured logs.
- Slow query logs with MongoDB command and translated SQL digest.
- Audit logs for auth, authorization failures, DDL, index changes, user management, transaction commit and rollback, and dangerous admin commands.

Trace fields:

- `trace_id`
- `lsid`
- `txnNumber`
- `user`
- `db`
- `collection`
- `command`
- `latency_ms`
- `affected_rows`
- `tidb_conn_id`

## Backup, Restore, and Repair

Physical backup uses TiDB BR, TiKV snapshots, and TiDB log backup.

timongo provides MongoDB semantic tools:

- `timongo-dump`: export by MongoDB namespace as BSON or Extended JSON.
- `timongo-restore`: import data and rebuild catalog/index entries.
- `timongo-check`: verify document table, index table, and catalog consistency.
- `timongo-repair-index`: rebuild index tables offline or with online throttling.

## Compatibility and Testing

Test layers:

1. Unit tests for BSON ordering, query matcher, update modifiers, index key encoding, planner rewrites, auth, and cursor/session state.
2. Compatibility tests derived from FerretDB-compatible frameworks and MongoDB driver behavior tests.
3. Differential tests against MongoDB 6.0, FerretDB, and timongo.
4. Integration tests on real TiDB, TiKV, PD, and TiFlash clusters.
5. Fault tests for timongo restart, bound tidb-server failure, LB drain, transaction rollback, cursor resume, and rolling upgrade.
6. Performance tests using YCSB-like and mongo-perf-like workloads.

Compatibility states:

- `supported`
- `partial`
- `planned`
- `unsupported`
- `intentionally-unsupported`

Every command, operator, aggregation stage, and index type must appear in a published compatibility matrix.

## Release Roadmap

### M0 Architecture Foundation

- Convert the current MVP into a FerretDB-reuse architecture.
- Split wire, command, planner, backend, catalog, index, cursor, session, auth, and observability modules.
- Remove memory backend from runtime paths.
- Create `_timongo` system schema.
- Add compatibility-test harness.

### M1 CRUD Production Preview

- MongoDB driver connection.
- SCRAM authentication foundation.
- CRUD commands.
- Basic query operators.
- Basic indexes.
- TiUP `timongo_servers` deploy and lifecycle operations.
- Prometheus metrics and structured logs.

### M2 Transactions and Aggregation

- Sessions.
- Retryable writes.
- Multi-document transactions.
- Stateless cursor persistence.
- Basic aggregation.
- Rolling upgrade path.

### M3 Enterprise GA

- Full role model for the supported command set.
- TLS and mTLS.
- Audit logs.
- Backup, restore, check, and repair tools.
- Compatibility certification report.
- HAProxy optional TiUP deployment.
- Performance and stability report.

### M4 Advanced Compatibility

- Change-stream design and implementation.
- TiFlash aggregation acceleration.
- More aggregation stages and expressions.
- More complete collation.
- MongoDB 7.x and 8.x compatibility expansion.

## Risks and Mitigations

| Risk | Impact | Mitigation |
| --- | --- | --- |
| BSON and TiDB JSON semantics differ | Incorrect query results | Preserve canonical BSON and re-check semantics in timongo |
| MongoDB compatibility surface is large | Scope creep | Publish compatibility matrix and staged roadmap |
| Multikey and collation indexes are complex | Query correctness and performance risk | Use canonical index tables, not only TiDB generated columns |
| Active transactions cannot migrate freely | Failover expectations | Follow MongoDB transaction pinning and return transient transaction errors |
| TiUP upstream changes are needed | Deployment integration risk | Maintain a timongo TiUP extension/fork first, then upstream if viable |
| HAProxy/NLB behavior differs by environment | Production routing risk | Define `/ready` semantics and provide reference HAProxy config |
| FerretDB internals may change | Reuse maintenance burden | Isolate FerretDB reuse behind timongo-owned interfaces |

## Acceptance Criteria

The enterprise architecture is ready for implementation planning when:

1. The repo contains this accepted design.
2. The current MVP is explicitly treated as prototype code.
3. The next implementation plan starts with M0 architecture foundation.
4. No runtime mode allows timongo-server to keep durable state outside TiDB.
5. `timongo_servers` topology support is part of the first planning phase.
6. Compatibility targets are measured against MongoDB 6.0.

