# timongo

timongo targets a stateless MongoDB 6.0-compatible gateway for TiDB.

## Enterprise Direction

The enterprise architecture is documented in:

- [timongo Enterprise Design](docs/superpowers/specs/2026-05-19-timongo-enterprise-design.md)
- [timongo Enterprise M0 Plan](docs/superpowers/plans/2026-05-19-timongo-enterprise-m0.md)

Current code is M0 architecture foundation work. It is not a full MongoDB replacement.

Runtime principles:

- `timongo serve` uses TiDB as the durable backend.
- memory storage is test-only.
- every timongo-server is configured to connect to one tidb-server.
- MongoDB client entry load balancing is done by HAProxy, cloud NLB, or another TCP load balancer.
- TiProxy remains a TiDB SQL-layer component and does not proxy MongoDB wire protocol.

## Source Study Notes

This repository also contains MongoDB, TiDB ecosystem, and FerretDB source-study notes.

This repository summarizes a staged learning plan for several database systems and MongoDB-compatible implementations:

- MongoDB server source: `mongodb/mongo`
- TiDB ecosystem: `pingcap/tidb`, `tikv/tikv`, `pingcap/tiflash`, `pingcap/tiup`
- FerretDB: `FerretDB/FerretDB`

Start here:

- [Database Source Study Summary](docs/database-source-study-summary.md)
- [timongo Feasibility Assessment](docs/timongo-feasibility-report.md)
- [timongo Product Design](docs/timongo-product-design.md)
- [MongoDB 6.0 Compatibility Matrix](docs/compatibility/mongodb-6.0-matrix.md)
- [timongo + TiDB v8.5.6 Packaging](docs/packaging-tidb-v8.5.6.md)

## Study Focus

The notes are organized around real execution paths rather than directory-by-directory reading:

1. how a request enters the server
2. how it is parsed and dispatched
3. how query planning and execution work
4. how storage, transactions, replication, and distributed execution are implemented
5. how compatibility is tested

## Repositories Covered

| System | Repository | Main language | Core role |
| --- | --- | --- | --- |
| MongoDB | `mongodb/mongo` | C++ | Document database server, query engine, replication, sharding, storage integration. |
| TiDB | `pingcap/tidb` | Go | MySQL-compatible SQL layer, planner, executor, DDL, distributed transaction client. |
| TiKV | `tikv/tikv` | Rust | Distributed transactional KV store, MVCC, Raftstore, coprocessor. |
| TiFlash | `pingcap/tiflash` | C++ | Columnar HTAP engine, Raft learner, MPP execution, DeltaMerge storage. |
| TiUP | `pingcap/tiup` | Go | TiDB component manager and cluster operations tool. |
| FerretDB | `FerretDB/FerretDB` | Go | MongoDB wire protocol proxy backed by PostgreSQL DocumentDB extension. |

## Learning Principle

Do not read these systems alphabetically. Pick one user-visible operation and trace it end to end.

Examples:

- `db.collection.find(...).explain()`
- `db.collection.insertOne(...)`
- `SELECT ... WHERE ... ORDER BY ... LIMIT ...`
- `ALTER TABLE ...`
- `tiup playground`
- FerretDB `find` through PostgreSQL DocumentDB extension

## M0 Implementation

This repository includes an early Go M0 implementation for `timongo`: a MongoDB wire protocol gateway foundation that handles a small command subset and stores runtime data through a TiDB/MySQL-compatible backend.

Supported commands in the current MVP:

- `hello` / `isMaster`
- `ping`
- `buildInfo`
- `insert`
- `find`

Current limits:

- no authentication
- no update/delete/aggregate yet
- TiUP topology support is a foundation scaffold, not a complete TiUP Cluster component yet
- no full MongoDB compatibility
- `find` supports empty filters, `_id` equality, and simple scalar equality

Build:

```bash
go build ./cmd/timongo
```

Build a TiDB v8.5.6 offline bundle with timongo:

```bash
./scripts/package-timongo-tidb.sh
```

For packaging smoke tests without downloading the multi-GB TiDB mirror:

```bash
./scripts/package-timongo-tidb.sh --skip-mirror
```

Run with TiDB backend:

```bash
go run ./cmd/timongo serve \
  -listen 127.0.0.1:27017 \
  -status-listen 127.0.0.1:28017 \
  -backend tidb \
  -tidb-dsn 'root@tcp(127.0.0.1:4000)/test?parseTime=true'
```

Basic smoke test:

```javascript
db.users.insertOne({_id: 1, name: "Ada"})
db.users.find({_id: 1})
```
