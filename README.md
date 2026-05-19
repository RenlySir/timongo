# timongo

MongoDB, TiDB ecosystem, and FerretDB source-study notes.

This repository summarizes a staged learning plan for several database systems and MongoDB-compatible implementations:

- MongoDB server source: `mongodb/mongo`
- TiDB ecosystem: `pingcap/tidb`, `tikv/tikv`, `pingcap/tiflash`, `pingcap/tiup`
- FerretDB: `FerretDB/FerretDB`

Start here:

- [Database Source Study Summary](docs/database-source-study-summary.md)
- [timongo Feasibility Assessment](docs/timongo-feasibility-report.md)
- [timongo Product Design](docs/timongo-product-design.md)

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

## MVP Implementation

This repository now includes an early Go MVP for `timongo`: a MongoDB wire protocol gateway that can handle a small command subset and store documents through either an in-memory backend or a TiDB/MySQL-compatible backend.

Supported commands in the current MVP:

- `hello` / `isMaster`
- `ping`
- `buildInfo`
- `insert`
- `find`

Current limits:

- no authentication
- no update/delete/aggregate yet
- no TiUP component packaging yet
- no full MongoDB compatibility
- `find` supports empty filters, `_id` equality, and simple scalar equality

Build:

```bash
go build ./cmd/timongo
```

Run with in-memory backend:

```bash
go run ./cmd/timongo serve -listen 127.0.0.1:27017 -backend memory
```

Run with TiDB backend:

```bash
go run ./cmd/timongo serve \
  -listen 127.0.0.1:27017 \
  -backend tidb \
  -tidb-dsn 'root:@tcp(127.0.0.1:4000)/timongo?parseTime=true'
```

Basic smoke test:

```javascript
db.users.insertOne({_id: 1, name: "Ada"})
db.users.find({_id: 1})
```
