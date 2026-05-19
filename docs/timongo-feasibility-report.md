# timongo Feasibility Assessment

## Executive Summary

timongo is feasible as a MongoDB-compatible access layer in front of a TiDB cluster, but only if the product is scoped as a staged compatibility project rather than a full MongoDB replacement on day one.

The recommended architecture is a stateless Go proxy similar in shape to FerretDB:

```text
MongoDB driver / mongosh / application
  -> load balancer
  -> timongo nodes
  -> TiDB SQL endpoint
  -> TiKV / PD / TiFlash
```

timongo should accept the MongoDB wire protocol, translate supported MongoDB commands into TiDB SQL, store documents primarily as JSON in TiDB tables, use TiDB generated columns or expression indexes for hot JSON paths, and optionally use TiFlash replicas for analytical aggregation workloads.

The MVP is viable for CRUD, basic query predicates, indexes on selected JSON paths, simple aggregation, authentication passthrough or proxy-level authentication, and TiUP-based deployment. Full MongoDB parity is not viable in the short term because MongoDB semantics cover a very large surface area: BSON type ordering, update operators, aggregation stages, collation, indexes, transactions, change streams, TTL, geospatial, text search, and replica/sharding admin commands.

## Target Positioning

timongo should be positioned as:

- a MongoDB-compatible protocol gateway for TiDB
- a migration and compatibility layer for applications that use a practical subset of MongoDB
- a way to use TiDB/TiKV/TiFlash as the durable and scalable backend for document-style workloads
- a TiUP-deployable component that runs next to TiDB, TiKV, PD, and TiFlash

timongo should not initially be positioned as:

- a drop-in replacement for every MongoDB deployment
- a MongoDB server implementation
- a replacement for TiDB SQL
- a replacement for FerretDB's PostgreSQL DocumentDB approach

## Reference Architecture

FerretDB proves that a MongoDB-compatible proxy is a workable pattern:

```text
MongoDB driver
  -> FerretDB wire protocol server
  -> command handler
  -> PostgreSQL connection pool
  -> DocumentDB extension functions
```

timongo can follow the same boundary but replace the backend with TiDB:

```text
MongoDB driver
  -> timongo wire protocol server
  -> command router
  -> semantic planner / translator
  -> TiDB SQL connection pool
  -> TiDB / TiKV / PD / TiFlash
```

The hard part moves from FerretDB's DocumentDB extension to timongo's own MongoDB-to-TiDB semantic translation layer. That is the central feasibility risk.

## Component Fit

### TiDB

TiDB is a strong fit for:

- SQL execution
- distributed transactions
- JSON document storage
- generated columns and indexes over extracted JSON paths
- SQL-based metadata tables
- horizontal scale via TiKV
- analytical routing to TiFlash

TiDB is a weaker fit for:

- exact BSON type system compatibility
- arbitrary MongoDB index semantics over nested arrays
- MongoDB collation behavior
- MongoDB aggregation semantics in full
- MongoDB oplog/change stream semantics
- MongoDB-specific administrative commands

### TiKV

TiKV is valuable as the durable distributed storage layer, but timongo should not talk to TiKV directly in the MVP.

Direct TiKV access would require timongo to reimplement too much of TiDB's SQL, transaction, table encoding, schema, and planner logic. The MVP should access TiDB through SQL and let TiDB manage TiKV.

Direct TiKV access can be revisited only for specialized performance paths after the SQL-backed compatibility layer is proven.

### PD

PD is required by TiDB/TiKV for timestamp allocation, metadata, placement, and scheduling. timongo does not need to integrate with PD directly in the MVP.

Possible later integrations:

- expose cluster health in timongo diagnostics
- route read preferences or analytical hints based on TiDB/TiFlash health
- collect region/hotspot metadata for observability

### TiFlash

TiFlash is valuable for analytical MongoDB aggregation workloads:

- `$match` + `$group`
- `$sort`
- `$project`
- large collection scans
- reporting-style queries

The MVP should not require TiFlash for correctness. It should treat TiFlash as an optional acceleration layer. Collections can have a timongo option that creates or recommends TiFlash replicas for analytical use.

### TiUP

TiUP is a strong fit for deployment. timongo should be packaged as a TiUP component and integrated into a TiUP cluster topology so users can deploy:

```text
pd
tikv
tidb
tiflash
timongo
```

Initial delivery can use a private TiUP mirror or documented manual component install. A mature release should support standard TiUP component metadata, binary artifacts, systemd service templates, health checks, scale-out, scale-in, upgrade, and reload.

## Data Model Feasibility

Recommended physical schema:

```sql
CREATE DATABASE timongo;

CREATE TABLE timongo_documents (
  database_name VARCHAR(128) NOT NULL,
  collection_name VARCHAR(128) NOT NULL,
  id VARBINARY(256) NOT NULL,
  doc JSON NOT NULL,
  created_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6),
  updated_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (database_name, collection_name, id)
);
```

For production, separate physical tables per MongoDB collection may be better:

```text
tm_<database>_<collection>
  _id_key
  doc JSON
  generated columns for indexed paths
```

The MVP should support one table per collection because:

- TiDB statistics are cleaner
- indexes are collection-local
- generated columns can be tailored per collection
- TiFlash replicas can be configured per collection table
- drop/rename collection maps naturally to SQL DDL

Metadata tables should track:

- databases
- collections
- indexes
- generated column mappings
- timongo compatibility options
- schema version
- user/auth metadata if proxy-level auth is implemented

## Query Translation Feasibility

### MVP Query Operators

Feasible:

- equality
- `$gt`, `$gte`, `$lt`, `$lte`
- `$ne`
- `$in`, `$nin`
- `$and`, `$or`, `$nor`
- `$exists`
- simple dot-path field access
- simple projection include/exclude
- sort
- skip
- limit
- count

Potentially feasible after MVP:

- `$regex` with caveats
- array matching subset
- `$elemMatch` subset
- collation subset
- partial indexes
- TTL through scheduled jobs

High-risk:

- exact MongoDB BSON comparison order
- full array semantics
- full collation behavior
- geospatial indexes
- text search parity
- wildcard indexes

### SQL Translation Shape

Example MongoDB query:

```javascript
db.users.find(
  {age: {$gte: 18}, status: "active"},
  {name: 1, age: 1}
).sort({age: -1}).limit(10)
```

Possible SQL:

```sql
SELECT JSON_OBJECT(
  '_id', JSON_EXTRACT(doc, '$._id'),
  'name', JSON_EXTRACT(doc, '$.name'),
  'age', JSON_EXTRACT(doc, '$.age')
) AS doc
FROM tm_app_users
WHERE JSON_EXTRACT(doc, '$.age') >= CAST(? AS JSON)
  AND JSON_EXTRACT(doc, '$.status') = CAST(? AS JSON)
ORDER BY JSON_EXTRACT(doc, '$.age') DESC
LIMIT 10;
```

For indexed fields, timongo should use generated columns:

```sql
ALTER TABLE tm_app_users
  ADD COLUMN g_age BIGINT AS (CAST(JSON_UNQUOTE(JSON_EXTRACT(doc, '$.age')) AS SIGNED)) STORED,
  ADD INDEX idx_age (g_age);
```

The planner should prefer generated columns over raw JSON expressions when an index exists.

## Write Translation Feasibility

Feasible MVP operations:

- `insertOne`
- `insertMany`
- `deleteOne`
- `deleteMany`
- `updateOne`
- `updateMany`
- `replaceOne`
- basic upsert

MVP update operators:

- `$set`
- `$unset`
- `$inc`
- `$currentDate` subset
- replacement document updates

Later:

- `$push`, `$pull`, `$addToSet`
- `$rename`
- `$min`, `$max`, `$mul`
- positional updates
- array filters

High-risk:

- full MongoDB update operator parity
- positional array updates
- concurrent update semantics matching MongoDB exactly

TiDB transactions are a strong foundation for write atomicity. Single-document writes can map to one SQL transaction. Multi-document writes can use TiDB transactions but must be carefully matched to MongoDB's acknowledged write behavior.

## Aggregation Feasibility

MVP aggregation stages:

- `$match`
- `$project`
- `$limit`
- `$skip`
- `$sort`
- `$count`
- `$group` with limited accumulators

MVP accumulators:

- `$sum`
- `$avg`
- `$min`
- `$max`
- `$count`

Later:

- `$lookup` mapped to SQL joins for timongo-managed collections
- `$unwind`
- `$addFields`
- `$set`
- `$facet`

High-risk:

- full expression language
- pipeline optimizer parity
- memory/disk spill semantics
- exact error messages and edge cases

TiFlash can accelerate aggregation, but only after translation correctness is stable.

## Protocol Feasibility

timongo should reuse proven Go libraries where possible:

- MongoDB wire protocol and BSON handling from `FerretDB/wire`, if licensing and API stability are acceptable
- official MongoDB Go driver only for tests/client behavior, not server protocol

Required protocol support:

- OP_MSG
- limited OP_QUERY for older client compatibility
- hello/isMaster
- buildInfo
- ping
- find
- getMore
- killCursors
- insert/update/delete
- aggregate
- count/distinct
- listDatabases/listCollections/listIndexes
- create/drop collection
- createIndexes/dropIndexes

MVP can return compatible "not supported" errors for admin, replication, and sharding commands.

## Deployment Feasibility

Target topology:

```yaml
global:
  user: tidb
  ssh_port: 22
  deploy_dir: /tidb-deploy
  data_dir: /tidb-data

pd_servers:
  - host: 10.0.0.1

tikv_servers:
  - host: 10.0.0.2
  - host: 10.0.0.3
  - host: 10.0.0.4

tidb_servers:
  - host: 10.0.0.5

tiflash_servers:
  - host: 10.0.0.6

timongo_servers:
  - host: 10.0.0.7
    port: 27017
    tidb_endpoint: 10.0.0.5:4000
```

TiUP can manage custom components if timongo provides:

- binary artifacts
- component manifest metadata
- systemd unit template
- config template
- health check
- scale and upgrade hooks

If upstream TiUP integration is not immediately available, use a private mirror or a wrapper script for MVP.

## Operational Feasibility

timongo should be stateless except for local logs and config. State should live in TiDB metadata tables.

Horizontal scaling:

- multiple timongo nodes behind L4 load balancer
- connection pooling from each timongo to TiDB
- no sticky session required for most operations if cursor state is stored centrally

Cursor state options:

1. local memory only
2. TiDB-backed cursor state
3. encoded cursor tokens

MVP recommendation: local cursor state plus load balancer stickiness. Production recommendation: centrally stored or resumable cursor state to avoid sticky sessions.

## Major Risks

| Risk | Severity | Mitigation |
| --- | --- | --- |
| MongoDB semantic surface is huge | High | Publish explicit compatibility matrix and implement staged command/operator subsets. |
| BSON vs JSON type mismatch | High | Define internal type encoding for ObjectId, Date, Binary, Decimal128, MinKey, MaxKey, regex. |
| Array query/update semantics | High | Start with limited array support and add compatibility tests before implementation. |
| Index semantics mismatch | High | Use generated columns for indexed paths; document unsupported index types. |
| Aggregation complexity | High | Start with SQL-translatable subset; use TiFlash only for supported analytical paths. |
| Cursor behavior behind load balancer | Medium | Use sticky sessions for MVP; design shared cursor state for production. |
| TiUP custom component integration | Medium | Start with private mirror/manual component install; upstream integration later. |
| Error message/code compatibility | Medium | Build MongoDB differential tests like FerretDB. |
| Performance unpredictability for JSON scans | Medium | Require explicit indexes for hot paths; add query planner warnings. |

## MVP Scope

### In Scope

- MongoDB wire protocol listener
- stateless timongo proxy process
- TiDB SQL backend
- database and collection metadata
- document insert/find/update/delete
- one table per collection
- JSON document storage
- `_id` primary key
- basic indexes using generated columns
- basic aggregation
- cursor support
- simple auth mode
- compatibility test suite against MongoDB
- TiUP deployment using custom/private component packaging
- Prometheus metrics and structured logs

### Out of Scope

- change streams
- MongoDB replica set semantics
- MongoDB sharding admin API
- full aggregation pipeline
- geospatial index parity
- text search parity
- GridFS optimization
- full BSON type ordering parity
- full transaction API parity
- direct TiKV access

## Recommended Product Path

### Phase 0: Prototype

Goal: prove wire protocol to TiDB SQL path.

- implement hello/ping/buildInfo
- implement insert/find for one collection
- store document JSON in TiDB
- support `_id`
- run mongosh against timongo

### Phase 1: CRUD MVP

Goal: support practical document CRUD.

- create/drop/list databases and collections
- insert/update/delete/find/getMore
- basic operators
- projection/sort/skip/limit
- generated-column indexes
- MongoDB differential tests

### Phase 2: Aggregation and TiFlash

Goal: support reporting workloads.

- `$match`, `$project`, `$group`, `$sort`, `$limit`
- TiFlash replica management option
- planner hinting for analytical paths
- explain output for translated SQL

### Phase 3: TiUP Productization

Goal: deploy as a TiDB ecosystem component.

- TiUP component package
- topology support
- systemd service
- config reload
- health check
- upgrade/rollback
- scale-out/scale-in

### Phase 4: Compatibility Expansion

Goal: expand MongoDB surface based on real applications.

- more operators
- more update semantics
- more aggregation stages
- auth/RBAC improvements
- compatibility dashboard
- migration tooling

## Feasibility Verdict

Build timongo if the product goal is:

- "MongoDB-compatible access to TiDB for common application workloads"
- "a migration bridge from MongoDB-style applications to TiDB"
- "a document API over TiDB with explicit compatibility levels"

Do not build it if the product goal is:

- "full MongoDB compatibility"
- "transparent replacement for arbitrary MongoDB clusters"
- "MongoDB change streams, sharding, and admin semantics over TiDB"

Recommended next step: build a Phase 0 prototype with three commands: `hello`, `insert`, and `find`. That prototype will validate the hardest architectural path: MongoDB wire protocol to BSON parsing to TiDB SQL generation to BSON response.
