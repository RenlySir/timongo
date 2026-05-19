# Database Source Study Summary

This document summarizes the learning notes for MongoDB, the TiDB ecosystem, and FerretDB.

The goal is not to memorize repository directories. The goal is to understand how each system handles real database operations from client request to storage, replication, distributed execution, and compatibility testing.

## Big Picture

| System | What to Learn First | Core Boundary |
| --- | --- | --- |
| MongoDB | `find`, write path, storage API, replication, sharding | A complete document database server implemented mostly in C++. |
| TiDB | SQL parsing, planning, executor, TiKV client, DDL | Stateless SQL layer over distributed KV and analytical engines. |
| TiKV | MVCC, Raftstore, coprocessor, RocksDB-backed engine | Distributed transactional key-value storage. |
| TiFlash | MPP, coprocessor, DeltaMerge, Raft learner | Columnar analytical replica for HTAP workloads. |
| TiUP | repository, playground, cluster topology, task graph | Component manager and deployment/orchestration tool. |
| FerretDB | MongoDB wire protocol, command handler, DocumentDB wrappers | MongoDB-compatible proxy backed by PostgreSQL DocumentDB extension. |

## MongoDB Source Study

Repository:

- https://github.com/mongodb/mongo

Core source areas:

| Path | Purpose |
| --- | --- |
| `src/mongo/db` | `mongod` server core. |
| `src/mongo/s` | `mongos` router and sharding logic. |
| `src/mongo/transport` | Network transport and session workflow. |
| `src/mongo/db/commands/query_cmd` | `find`, `aggregate`, `getMore`, write commands. |
| `src/mongo/db/query` | query canonicalization, planning, plan cache, plan executor. |
| `src/mongo/db/matcher` | filter AST and `MatchExpression`. |
| `src/mongo/db/pipeline` | aggregation pipeline and `DocumentSource` stages. |
| `src/mongo/db/exec` | classic and SBE execution engines. |
| `src/mongo/db/storage` | storage engine API. |
| `src/mongo/db/repl` | oplog, replication, write concern, elections. |

### MongoDB Key Concepts

| Concept | Meaning |
| --- | --- |
| `ServiceContext` | process-wide state. |
| `OperationContext` | per-operation state: locks, deadlines, recovery unit, interruptibility. |
| `Command` | server command abstraction. |
| `CanonicalQuery` | normalized representation of a find-like query. |
| `MatchExpression` | parsed query predicate tree. |
| `QuerySolution` | candidate or winning logical plan. |
| `PlanExecutor` | runtime object that pulls results from execution stages. |
| `RecordStore` | storage abstraction for collection records. |
| `SortedDataInterface` | storage abstraction for indexes. |
| `RecoveryUnit` | storage transaction and snapshot abstraction. |
| `WriteUnitOfWork` | RAII commit/rollback boundary for writes. |
| `OpObserver` | observes writes and records replication side effects such as oplog entries. |

### MongoDB Recommended First Trace

Trace:

```javascript
db.users.find({age: {$gte: 18}}, {name: 1}).sort({age: -1}).limit(10)
```

Read:

```text
src/mongo/transport/session_workflow.cpp
src/mongo/db/service_entry_point*
src/mongo/db/commands.cpp
src/mongo/db/commands/query_cmd/find_cmd.cpp
src/mongo/db/query/find_command.idl
src/mongo/db/query/parsed_find_command.*
src/mongo/db/query/canonical_query.*
src/mongo/db/matcher/expression*.*
src/mongo/db/query/get_executor.*
src/mongo/db/query/plan_executor*
src/mongo/db/exec/classic
src/mongo/db/exec/sbe
src/mongo/db/storage/record_store.h
src/mongo/db/storage/sorted_data_interface.h
```

What to understand:

- BSON command becomes an IDL-generated request.
- Filter becomes `MatchExpression`.
- Planner generates `QuerySolution`.
- Executor runs classic or SBE plan.
- Storage API provides record and index access.

### MongoDB Write and Replication Trace

Trace:

```javascript
db.users.insertOne({_id: 1, name: "Ada"})
db.users.updateOne({_id: 1}, {$set: {name: "Ada Lovelace"}})
db.users.deleteOne({_id: 1})
```

Read:

```text
src/mongo/db/commands/query_cmd/write_commands.cpp
src/mongo/db/collection_crud/collection_write_path.*
src/mongo/db/storage/write_unit_of_work.h
src/mongo/db/storage/recovery_unit.h
src/mongo/db/op_observer/op_observer_impl.cpp
src/mongo/db/repl/oplog.cpp
src/mongo/db/write_concern.*
```

What to understand:

- Writes run inside `WriteUnitOfWork`.
- Data and indexes are updated through storage abstractions.
- `OpObserver` records oplog entries.
- Write concern waits for replication progress.

## TiDB Ecosystem Source Study

Repositories:

- https://github.com/pingcap/tidb
- https://github.com/tikv/tikv
- https://github.com/pingcap/tiflash
- https://github.com/pingcap/tiup

Architecture:

```text
MySQL client
  -> TiDB SQL server
  -> parser / planner / executor
  -> TiKV transaction KV or TiFlash MPP
  -> PD for timestamp and region metadata
  -> Raft / MVCC / RocksDB or TiFlash DeltaMerge
```

### TiDB Core Areas

| Path | Purpose |
| --- | --- |
| `cmd/tidb-server` | TiDB server entry point. |
| `pkg/server` | MySQL protocol server. |
| `pkg/session` | SQL session and statement lifecycle. |
| `pkg/parser` | SQL parser and AST. |
| `pkg/planner` | logical and physical optimizer. |
| `pkg/executor` | SQL execution. |
| `pkg/distsql` | distributed SQL request construction. |
| `pkg/store` | storage backend integration. |
| `pkg/kv` | KV and transaction abstractions. |
| `pkg/tablecodec` | table and index row encoding. |
| `pkg/ddl` | schema change jobs. |
| `pkg/domain` | schema lease, DDL, statistics, InfoSchema coordination. |

### TiKV Core Areas

| Path | Purpose |
| --- | --- |
| `src/server` | gRPC services and raft server. |
| `src/storage` | transactional KV service. |
| `src/storage/mvcc` | MVCC reader/writer logic. |
| `src/storage/txn` | transaction command path. |
| `components/raftstore` | Region peer state machine and Raft. |
| `components/engine_traits` | storage engine abstraction. |
| `components/engine_rocks` | RocksDB engine implementation. |
| `components/tidb_query_executors` | TiDB pushed-down executors. |
| `components/tidb_query_expr` | pushed-down expression evaluation. |

### TiFlash Core Areas

| Path | Purpose |
| --- | --- |
| `dbms/src/Flash` | query serving layer. |
| `dbms/src/Flash/Coprocessor` | coprocessor request handling. |
| `dbms/src/Flash/Mpp` | MPP execution. |
| `dbms/src/Flash/Planner` | TiFlash query planning. |
| `dbms/src/Flash/Executor` | execution operators. |
| `dbms/src/Storages/KVStore` | Raft learner ingestion and Region metadata. |
| `dbms/src/Storages/DeltaMerge` | columnar DeltaMerge storage engine. |
| `dbms/src/TiDB` | TiDB metadata/protocol structures. |

### TiUP Core Areas

| Path | Purpose |
| --- | --- |
| `main.go` | TiUP entry point. |
| `cmd` | install, update, mirror, env, status commands. |
| `components/playground` | local playground cluster. |
| `components/cluster` | cluster operation CLI. |
| `pkg/repository` | component repository metadata and artifacts. |
| `pkg/cluster/spec` | topology specification. |
| `pkg/cluster/task` | deployment task graph. |
| `pkg/cluster/operation` | start, stop, scale, upgrade operations. |
| `pkg/cluster/executor` | SSH/local executor abstraction. |

### TiDB Recommended First Trace

Trace:

```sql
CREATE TABLE users (id BIGINT PRIMARY KEY, age INT, name VARCHAR(64), KEY idx_age(age));
SELECT id, name FROM users WHERE age >= 18 ORDER BY age LIMIT 10;
```

Read:

```text
pingcap/tidb/pkg/server/conn_stmt.go
pingcap/tidb/pkg/session/session.go
pingcap/tidb/pkg/parser
pingcap/tidb/pkg/planner/optimize.go
pingcap/tidb/pkg/planner/core
pingcap/tidb/pkg/statistics
pingcap/tidb/pkg/executor/compiler.go
pingcap/tidb/pkg/executor/builder.go
pingcap/tidb/pkg/executor/table_reader.go
pingcap/tidb/pkg/executor/index_lookup_reader.go
pingcap/tidb/pkg/distsql
tikv/tikv/src/server/service
tikv/tikv/src/coprocessor
tikv/tikv/components/tidb_query_executors
tikv/tikv/src/storage/mvcc
```

What to understand:

- TiDB parses SQL into AST.
- Planner chooses physical plan using statistics.
- Executor builds local and distributed executors.
- TiKV coprocessor runs pushed-down scans, filters, aggregations, TopN, and limits.
- TiKV reads MVCC data from Region storage.

### TiDB Transaction Trace

Trace:

```sql
BEGIN;
INSERT INTO users VALUES (1, 20, 'Ada');
UPDATE users SET age = 21 WHERE id = 1;
COMMIT;
```

Read:

```text
pingcap/tidb/pkg/executor/insert.go
pingcap/tidb/pkg/executor/update.go
pingcap/tidb/pkg/executor/write.go
pingcap/tidb/pkg/tablecodec
pingcap/tidb/pkg/sessiontxn
pingcap/tidb/pkg/store/tikv
tikv/tikv/src/storage/txn
tikv/tikv/src/storage/mvcc
tikv/tikv/components/raftstore/src/store
tikv/tikv/components/engine_rocks
```

What to understand:

- TiDB encodes rows and indexes into KV pairs.
- Transaction commit goes through TiKV prewrite/commit flow.
- TiKV handles locks, MVCC, conflict detection, and rollback.
- Raftstore replicates durable changes across Region peers.

### TiFlash HTAP Trace

Trace:

```sql
ALTER TABLE orders SET TIFLASH REPLICA 1;
SELECT user_id, SUM(amount) FROM orders GROUP BY user_id;
```

Read:

```text
pingcap/tidb/pkg/planner/core
pingcap/tidb/pkg/executor/mpp_gather.go
pingcap/tidb/pkg/executor/mppcoordmanager
pingcap/tiflash/dbms/src/Flash/FlashService.cpp
pingcap/tiflash/dbms/src/Flash/CoprocessorHandler.cpp
pingcap/tiflash/dbms/src/Flash/Mpp
pingcap/tiflash/dbms/src/Flash/Planner
pingcap/tiflash/dbms/src/Flash/Executor
pingcap/tiflash/dbms/src/Storages/DeltaMerge
pingcap/tiflash/dbms/src/Storages/KVStore
```

What to understand:

- TiDB optimizer can choose TiFlash for analytical plans.
- TiFlash receives Raft learner replication from TiKV.
- DeltaMerge provides columnar storage.
- MPP distributes analytical execution across TiFlash nodes.

### TiUP Trace

Trace:

```bash
tiup playground
tiup cluster deploy demo v8.5.0 topology.yaml
tiup cluster start demo
tiup cluster scale-out demo scale-out.yaml
```

Read:

```text
pingcap/tiup/main.go
pingcap/tiup/cmd
pingcap/tiup/components/playground
pingcap/tiup/components/cluster
pingcap/tiup/pkg/repository
pingcap/tiup/pkg/cluster/spec
pingcap/tiup/pkg/cluster/task
pingcap/tiup/pkg/cluster/operation
pingcap/tiup/pkg/cluster/executor
```

What to understand:

- TiUP resolves component versions from repository metadata.
- Playground starts a local set of TiDB components.
- Cluster mode converts topology YAML into task graphs.
- Executor performs local or SSH-based operations.

## FerretDB Source Study

Repository:

- https://github.com/FerretDB/FerretDB

FerretDB v2 architecture:

```text
MongoDB client / driver
  -> MongoDB wire protocol BSON messages
  -> FerretDB Go server
  -> command handler
  -> PostgreSQL connection pool
  -> PostgreSQL with DocumentDB extension
```

FerretDB is not a MongoDB storage engine reimplementation in Go. It is a MongoDB-compatible proxy that delegates document semantics to PostgreSQL with the DocumentDB extension.

### FerretDB Core Areas

| Path | Purpose |
| --- | --- |
| `cmd/ferretdb` | binary entry point and CLI flags. |
| `ferretdb` | embeddable Go package. |
| `internal/clientconn` | MongoDB wire protocol server. |
| `internal/handler` | MongoDB command handlers. |
| `internal/handler/middleware` | request/response middleware and operation modes. |
| `internal/handler/session` | sessions and cursor tracking. |
| `internal/documentdb` | PostgreSQL DocumentDB integration. |
| `internal/documentdb/documentdb_api` | generated wrappers for DocumentDB procedures. |
| `internal/mongoerrors` | MongoDB-compatible error mapping. |
| `integration` | integration and compatibility tests. |
| `Taskfile.yml` | development/test workflow. |

### FerretDB Key Concepts

| Concept | Meaning |
| --- | --- |
| `clientconn.Listener` | accepts MongoDB protocol connections. |
| `clientconn.Conn` | handles one client connection. |
| `handler.Handler` | shared command dispatcher. |
| `middleware.Request` | parsed request with wire body and command document. |
| `documentdb.Pool` | pgx pool plus cursor registry. |
| `documentdb_api.*` | generated wrappers for PostgreSQL stored procedures. |
| `session.Registry` | logical session and cursor tracking. |
| `mongoerrors` | maps errors into MongoDB-compatible responses. |

### FerretDB Recommended First Trace

Trace:

```javascript
db.users.find({age: {$gte: 18}}, {name: 1}).sort({age: -1}).limit(10)
```

Read:

```text
internal/clientconn/listener.go
internal/clientconn/conn.go
internal/handler/handler.go
internal/handler/commands.go
internal/handler/msg_find.go
internal/handler/session
internal/documentdb/pool_cursors.go
internal/documentdb/documentdb_api/documentdb_api.go
```

What to understand:

- `clientconn` reads a MongoDB wire protocol message.
- `handler.Handle` dispatches by command name.
- `msgFind` extracts `$db`, updates session state, and calls `Pool.Find`.
- `Pool.Find` calls generated `documentdb_api.FindCursorFirstPage`.
- The generated wrapper executes a PostgreSQL function.
- FerretDB returns BSON response through middleware.

### FerretDB Write Trace

Trace:

```javascript
db.users.insertOne({_id: 1, age: 20, name: "Ada"})
db.users.updateOne({_id: 1}, {$set: {age: 21}})
db.users.deleteOne({_id: 1})
```

Read:

```text
internal/handler/msg_insert.go
internal/handler/msg_update.go
internal/handler/msg_delete.go
internal/documentdb/documentdb_api/documentdb_api.go
internal/mongoerrors
integration/insert_*
integration/update_*
integration/delete_*
```

What to understand:

- Write commands parse OP_MSG sections using `FerretDB/wire`.
- Command specs and document sequences are passed to generated DocumentDB wrappers.
- Errors are mapped into MongoDB-compatible write responses.
- Compatibility tests compare behavior with MongoDB.

## Cross-System Comparison

| Dimension | MongoDB | TiDB Ecosystem | FerretDB |
| --- | --- | --- | --- |
| Client protocol | MongoDB wire protocol | MySQL protocol | MongoDB wire protocol |
| Query language | MQL / aggregation | SQL | MQL / aggregation subset via DocumentDB |
| Server role | full database server | distributed SQL layer plus storage engines | proxy/compatibility layer |
| Storage | WiredTiger through storage API | TiKV row KV, TiFlash columnar | PostgreSQL DocumentDB extension |
| Transaction model | document and multi-document transactions | Percolator-style distributed transactions over TiKV | delegated to PostgreSQL/DocumentDB backend behavior |
| Replication | oplog and replica sets | Raft Regions in TiKV, TiFlash learners | backend-dependent |
| Distributed query | sharding, mongos | TiDB distributed SQL, TiKV coprocessor, TiFlash MPP | not a distributed database engine itself |
| Testing focus | C++ unit + JS integration | Go/Rust/C++ unit/integration + cluster tests | MongoDB compatibility integration tests |

## Suggested Study Order

1. FerretDB first if your goal is MongoDB compatibility and protocol translation. It has a smaller Go codebase and a clear proxy architecture.
2. MongoDB next if your goal is document database internals: query planner, execution engine, storage API, oplog, sharding.
3. TiDB/TiKV next if your goal is distributed SQL and transaction systems: SQL planning, distributed KV, MVCC, Raft.
4. TiFlash after TiDB/TiKV if your goal is HTAP and columnar MPP execution.
5. TiUP anytime you need deployment and operations perspective.

## Practical Capstones

### Capstone 1: MongoDB `find`

Deliverable:

- sequence diagram from wire request to storage cursor
- file/function list
- output from `explain("executionStats")`

### Capstone 2: TiDB indexed query

Deliverable:

- `EXPLAIN ANALYZE` output
- physical plan mapping to TiDB executor code
- TiKV pushed-down executor mapping

### Capstone 3: TiDB transaction

Deliverable:

- KV encoding example for row and index
- prewrite/commit sequence
- TiKV MVCC state diagram

### Capstone 4: TiFlash analytical query

Deliverable:

- planner choice between TiKV and TiFlash
- MPP task flow
- DeltaMerge read path summary

### Capstone 5: FerretDB `find`

Deliverable:

- wire protocol to handler dispatch diagram
- generated `documentdb_api` wrapper call
- cursor lifecycle explanation
- compatibility test mapping

## Official References

MongoDB:

- https://github.com/mongodb/mongo
- https://www.mongodb.com/docs/

TiDB ecosystem:

- https://github.com/pingcap/tidb
- https://github.com/tikv/tikv
- https://github.com/pingcap/tiflash
- https://github.com/pingcap/tiup
- https://docs.pingcap.com/tidb/stable/tidb-architecture
- https://docs.pingcap.com/tidb/stable/tiflash-overview

FerretDB:

- https://github.com/FerretDB/FerretDB
- https://docs.ferretdb.io/
- https://docs.ferretdb.io/migration/compatibility/
- https://github.com/documentdb/documentdb
- https://github.com/FerretDB/wire
