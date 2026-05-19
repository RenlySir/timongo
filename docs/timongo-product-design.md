# timongo Product Design

## Product Definition

timongo is a MongoDB-compatible gateway for TiDB.

Users connect to timongo with MongoDB drivers, MongoDB Shell, or MongoDB-compatible tools. timongo translates supported MongoDB wire protocol commands into TiDB SQL and stores documents in TiDB. TiDB then uses TiKV, PD, and optionally TiFlash for storage, transactions, scheduling, and analytical acceleration.

Deployment target:

```text
application / MongoDB driver
  -> load balancer
  -> timongo
  -> TiDB
  -> PD / TiKV / TiFlash
```

Primary deployment method:

```bash
tiup cluster deploy <cluster-name> <version> topology.yaml
```

with `timongo_servers` as a TiUP-managed component.

## Goals

1. Let applications use a practical subset of MongoDB APIs while storing data in TiDB.
2. Use TiDB's distributed SQL, TiKV storage, PD scheduling, and TiFlash analytical capabilities.
3. Provide clear compatibility levels and differential tests against MongoDB.
4. Deploy and operate timongo through TiUP alongside TiDB ecosystem components.
5. Keep timongo stateless where possible so it can scale horizontally behind a load balancer.

## Non-Goals

1. Full MongoDB server parity in the first releases.
2. Direct TiKV access in MVP.
3. MongoDB replica set or sharding admin semantics.
4. Change stream parity.
5. Full aggregation expression parity.
6. Transparent support for arbitrary MongoDB workloads without compatibility review.

## Personas

### Application Developer

Wants to keep using MongoDB drivers and document-style APIs while gaining TiDB's distributed storage and operational model.

Needs:

- familiar MongoDB URI
- common CRUD commands
- basic indexes
- predictable compatibility matrix
- clear errors for unsupported features

### DBA / Platform Engineer

Wants to deploy and operate timongo like a TiDB component.

Needs:

- TiUP deployment
- logs and metrics
- health checks
- scale-out and upgrade
- backup/restore guidance
- compatibility and performance diagnostics

### Migration Engineer

Wants to evaluate whether a MongoDB workload can move to TiDB through timongo.

Needs:

- command/operator compatibility report
- schema/index recommendations
- query explain and translated SQL
- differential test tooling

## Product Editions

### Developer Preview

Purpose: validate architecture.

Features:

- local binary
- manual config
- hello/ping/buildInfo
- basic insert/find
- simple JSON table
- no TiUP integration required

### MVP

Purpose: usable for simple applications.

Features:

- CRUD
- basic query operators
- simple projections and sorting
- generated-column indexes
- cursor support
- TiUP private component deployment
- metrics/logging
- MongoDB compatibility tests

### Production Preview

Purpose: operate in a TiDB cluster.

Features:

- TiUP topology support
- HA timongo nodes behind load balancer
- centralized metadata
- shared or resumable cursor strategy
- TiFlash analytical option
- compatibility reporting
- upgrade/rollback

## Architecture

```text
MongoDB client
  |
  | MongoDB wire protocol
  v
timongo listener
  |
  v
wire protocol decoder
  |
  v
command router
  |
  v
command handlers
  |
  v
semantic planner / translator
  |
  +--> metadata manager
  +--> index manager
  +--> cursor manager
  +--> auth manager
  |
  v
TiDB SQL executor
  |
  v
TiDB server
  |
  +--> TiKV / PD
  +--> TiFlash
```

## Core Modules

### 1. Wire Protocol Module

Responsibilities:

- accept TCP/TLS connections
- read MongoDB wire protocol messages
- support OP_MSG
- support limited OP_QUERY for compatibility
- parse BSON command documents
- write MongoDB-compatible responses

Recommended implementation:

- use Go
- evaluate reusing `github.com/FerretDB/wire`
- keep protocol types separate from command semantics

Interfaces:

```text
Listener -> Conn -> Request -> Response
```

### 2. Command Router

Responsibilities:

- extract command name from BSON document
- check authentication and authorization
- dispatch to command handler
- generate MongoDB-compatible unsupported-command errors

Initial command registry:

| Command | MVP Status |
| --- | --- |
| `hello`, `isMaster` | supported |
| `ping` | supported |
| `buildInfo` | supported |
| `find` | supported |
| `getMore` | supported |
| `killCursors` | supported |
| `insert` | supported |
| `update` | supported subset |
| `delete` | supported |
| `aggregate` | supported subset |
| `count` | supported |
| `distinct` | supported subset |
| `create` | supported |
| `drop` | supported |
| `listDatabases` | supported |
| `listCollections` | supported |
| `createIndexes` | supported subset |
| `listIndexes` | supported |
| `dropIndexes` | supported subset |

Unsupported commands should return stable MongoDB-compatible errors, not generic internal errors.

### 3. Semantic Translator

Responsibilities:

- parse MongoDB command specs into internal ASTs
- validate supported semantics
- translate supported query/update/aggregation expressions into SQL
- choose generated-column indexes when available
- produce explain/debug data

Submodules:

```text
translator/query
translator/projection
translator/sort
translator/update
translator/aggregate
translator/index
translator/errors
```

Design rule:

Do not generate SQL directly from raw BSON everywhere. Convert BSON to typed internal nodes first. This avoids ad hoc string building and keeps compatibility testing focused.

Example internal query nodes:

```text
Eq(path, value)
Gt(path, value)
Gte(path, value)
Lt(path, value)
Lte(path, value)
In(path, values)
Exists(path, bool)
And(children)
Or(children)
Nor(children)
```

### 4. TiDB SQL Executor

Responsibilities:

- manage TiDB SQL connection pool
- execute generated SQL
- map TiDB errors to MongoDB errors
- manage transactions
- enforce timeouts
- collect metrics

MVP:

- use MySQL protocol driver for TiDB
- connection pool per timongo process
- one SQL transaction per MongoDB write command
- explicit context deadlines

### 5. Metadata Manager

Responsibilities:

- track MongoDB databases and collections
- map logical collection names to TiDB physical tables
- track index definitions
- track generated columns
- track compatibility options
- manage metadata schema version

Metadata schema:

```sql
CREATE TABLE timongo_meta_databases (
  name VARCHAR(128) PRIMARY KEY,
  created_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6)
);

CREATE TABLE timongo_meta_collections (
  database_name VARCHAR(128) NOT NULL,
  collection_name VARCHAR(128) NOT NULL,
  table_name VARCHAR(256) NOT NULL,
  options_json JSON,
  created_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (database_name, collection_name)
);

CREATE TABLE timongo_meta_indexes (
  database_name VARCHAR(128) NOT NULL,
  collection_name VARCHAR(128) NOT NULL,
  index_name VARCHAR(128) NOT NULL,
  key_spec JSON NOT NULL,
  unique_index BOOL NOT NULL DEFAULT FALSE,
  generated_columns JSON,
  state VARCHAR(32) NOT NULL,
  created_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (database_name, collection_name, index_name)
);
```

### 6. Document Storage

Default table per collection:

```sql
CREATE TABLE tm_<db>_<collection> (
  _id_key VARBINARY(512) NOT NULL,
  doc JSON NOT NULL,
  created_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6),
  updated_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (_id_key)
);
```

`_id_key` is a canonical binary encoding of MongoDB `_id`.

The JSON `doc` stores the user document. BSON types that JSON cannot represent exactly must use a canonical extended representation.

Type encoding policy:

| BSON Type | MVP Handling |
| --- | --- |
| string, bool, int32, int64, double | native JSON-compatible encoding with type caveats |
| ObjectId | extended JSON marker |
| Date | extended JSON marker |
| Decimal128 | extended JSON marker or string-backed typed marker |
| Binary | extended JSON marker |
| null | JSON null |
| array | JSON array with limited query/update semantics |
| regex | unsupported in MVP or stored marker only |
| MinKey/MaxKey | unsupported in MVP |

### 7. Index Manager

MongoDB index definitions map to TiDB generated columns plus indexes.

Example:

```javascript
db.users.createIndex({age: 1})
```

Generated SQL:

```sql
ALTER TABLE tm_app_users
  ADD COLUMN g_idx_age BIGINT
    AS (CAST(JSON_UNQUOTE(JSON_EXTRACT(doc, '$.age')) AS SIGNED)) STORED,
  ADD INDEX idx_age (g_idx_age);
```

Supported MVP index types:

- `_id` unique primary key
- single-field ascending/descending normal index
- compound indexes over scalar paths
- unique indexes over scalar paths, with documented null/missing semantics

Out of scope initially:

- text indexes
- geospatial indexes
- wildcard indexes
- hashed indexes
- partial indexes
- sparse indexes unless explicitly implemented with clear caveats

### 8. Cursor Manager

MVP:

- cursor state stored in timongo memory
- require load balancer sticky sessions
- cursor timeout and kill support

Production option:

- store cursor state in TiDB metadata tables
- or encode resumable cursor tokens

Cursor state:

```text
cursor_id
database
collection
sql
parameters
offset or resume token
batch size
created_at
last_access_at
owner node
```

Offset-based cursors are simple but can be inefficient and inconsistent under writes. Resume-token or keyset cursors are preferable for large result sets.

### 9. Authentication and Authorization

MVP options:

1. disabled auth for internal/test environments
2. static user/password config in timongo
3. timongo-managed users in TiDB metadata tables

Recommendation:

- MVP supports static user/password and optional no-auth mode.
- Production preview adds timongo-managed users and roles.

MongoDB SCRAM compatibility can be phased in later.

### 10. TiFlash Integration

TiFlash should be optional.

Collection option:

```javascript
db.runCommand({
  collMod: "orders",
  timongo: {
    tiflashReplica: 1
  }
})
```

Equivalent SQL:

```sql
ALTER TABLE tm_app_orders SET TIFLASH REPLICA 1;
```

Translator can route supported aggregation queries to TiFlash using TiDB hints where appropriate.

The first implementation should not expose this as MongoDB-native behavior. It should be a timongo extension.

### 11. Explain and Diagnostics

timongo should provide:

- MongoDB-compatible `explain` for supported commands
- translated SQL
- chosen indexes
- TiDB `EXPLAIN` or `EXPLAIN ANALYZE` output where safe
- compatibility warnings

Example extension:

```javascript
db.runCommand({
  explain: {
    find: "users",
    filter: {age: {$gte: 18}}
  },
  timongoVerbose: true
})
```

Returned diagnostics may include:

```json
{
  "timongo": {
    "sql": "...",
    "usedGeneratedColumns": ["g_idx_age"],
    "unsupportedFallbacks": [],
    "tidbPlan": [...]
  }
}
```

## Compatibility Levels

Define explicit compatibility levels:

| Level | Meaning |
| --- | --- |
| L0 | command recognized, returns compatible unsupported response when not implemented |
| L1 | basic happy-path behavior |
| L2 | common edge cases and error codes covered |
| L3 | differential tests against MongoDB for broad cases |
| L4 | production compatibility for documented workload class |

Every command/operator should have a compatibility level.

## MVP Command Compatibility Matrix

| Area | Supported in MVP | Notes |
| --- | --- | --- |
| handshake | yes | `hello`, `isMaster`, `buildInfo`, `ping` |
| CRUD | yes | common subset |
| query predicates | partial | scalar paths first |
| projection | partial | include/exclude without complex expressions |
| sort/skip/limit | yes | index recommended for sort |
| cursor | yes | sticky session initially |
| indexes | partial | generated columns for scalar paths |
| aggregation | partial | SQL-translatable subset |
| transactions | limited | command-level atomicity first |
| auth | limited | static or no-auth mode |
| change streams | no | out of scope |
| replica set commands | no | return compatible unsupported responses |
| sharding commands | no | TiDB handles distribution internally |

## Deployment Design

### TiUP Topology

Desired topology extension:

```yaml
timongo_servers:
  - host: 10.0.0.10
    port: 27017
    status_port: 27018
    deploy_dir: /tidb-deploy/timongo-27017
    log_dir: /tidb-deploy/timongo-27017/log
    config:
      tidb_endpoints:
        - 10.0.0.5:4000
      auth:
        enabled: true
      compatibility:
        level: mvp
```

### timongo Config

```yaml
server:
  listen_addr: 0.0.0.0:27017
  status_addr: 0.0.0.0:27018
  tls:
    enabled: false

tidb:
  endpoints:
    - 127.0.0.1:4000
  username: root
  password_file: /etc/timongo/tidb.password
  database: timongo
  max_open_conns: 100
  max_idle_conns: 20

compatibility:
  mode: strict
  allow_unsupported_commands: false
  default_batch_size: 101

cursor:
  backend: memory
  timeout: 10m
  require_sticky_sessions: true

log:
  level: info
  format: json

metrics:
  enabled: true
```

### Load Balancing

MVP:

- TCP load balancer
- sticky sessions enabled
- health check on status port

Production:

- avoid sticky sessions by centralizing cursor state
- expose readiness only after TiDB metadata schema is initialized

## Observability

Metrics:

- connections open
- commands total by command/status
- command latency histogram
- SQL latency histogram
- TiDB connection pool stats
- cursor count
- unsupported command/operator count
- translated query count by shape
- slow command log count

Logs:

- command name
- database and collection
- duration
- generated SQL hash
- error code
- request id
- client address

Tracing:

```text
wire receive
  -> command dispatch
  -> translation
  -> SQL execution
  -> response encode
```

## Error Handling

Principles:

- unsupported command returns MongoDB-compatible command error
- unsupported operator returns query/update parse error
- TiDB duplicate key maps to duplicate key write error
- TiDB timeout maps to MaxTimeMSExpired where appropriate
- internal errors are redacted by default

Error mapping table should be a first-class module:

```text
timongo/internal/mongoerrors
timongo/internal/tidberrors
```

## Testing Design

Test layers:

| Layer | Purpose |
| --- | --- |
| unit tests | parser, translator, SQL generation, error mapping |
| integration tests | timongo + TiDB |
| compatibility tests | same workload against timongo and MongoDB |
| TiUP tests | deploy/start/scale/upgrade |
| performance tests | representative CRUD and aggregation workloads |

Compatibility harness:

```text
test case
  -> MongoDB target
  -> timongo target
  -> normalize results
  -> compare output, errors, codes, and write results
```

This should follow FerretDB's approach: compatibility behavior is a product artifact, not an afterthought.

## Security Design

MVP:

- TLS support at timongo listener
- TLS support to TiDB where configured
- static user/password file
- redacted logs
- least-privilege TiDB user

Later:

- SCRAM authentication
- role-based auth
- audit log
- integration with external secret management

## Backup and Recovery

timongo document data lives in TiDB. Backup should use TiDB ecosystem tools:

- BR for TiDB/TiKV backup and restore
- TiFlash replicas can be rebuilt
- timongo metadata tables must be included in backup

timongo should document:

- which schemas/tables it creates
- how to restore metadata and collection tables
- how to rebuild generated indexes
- how to reapply TiFlash replica settings

## Roadmap

### Milestone 0: Architecture Prototype

Features:

- Go binary
- MongoDB wire listener
- hello/ping/buildInfo
- TiDB connection pool
- create one collection table
- insertOne
- find equality by `_id`

Success criteria:

- `mongosh` connects to timongo
- `db.collection.insertOne` succeeds
- `db.collection.findOne` returns the inserted document from TiDB

### Milestone 1: CRUD MVP

Features:

- create/drop/list databases and collections
- insertMany
- find with scalar predicates
- projection/sort/skip/limit
- getMore/killCursors
- update/delete subset
- createIndexes/listIndexes
- generated columns
- compatibility test harness

Success criteria:

- common CRUD integration tests pass
- known unsupported features return stable errors

### Milestone 2: Aggregation and TiFlash Preview

Features:

- `$match`, `$project`, `$group`, `$sort`, `$limit`, `$count`
- TiFlash replica option
- explain with translated SQL and TiDB plan

Success criteria:

- simple analytical workloads can use TiFlash through TiDB
- explain identifies SQL and backend choice

### Milestone 3: TiUP Deployment

Features:

- TiUP component package
- topology support
- systemd service
- health checks
- reload/upgrade/scale

Success criteria:

- `tiup cluster deploy` can deploy TiDB + timongo topology
- multiple timongo nodes can run behind load balancer

### Milestone 4: Production Preview

Features:

- shared cursor state
- auth improvements
- compatibility dashboard
- migration assessment tool
- performance tuning guide

Success criteria:

- selected real-world MongoDB workloads can be certified at compatibility level L3 or L4

## Open Decisions

1. Whether to reuse `FerretDB/wire` directly or implement a minimal internal protocol layer.
2. Whether MVP cursor state should require sticky sessions or use TiDB from the start.
3. Exact BSON extended encoding format.
4. Whether one table per collection is mandatory or configurable.
5. How much MongoDB auth compatibility is required for MVP.
6. Whether TiUP integration starts as private mirror only or targets upstream TiUP component conventions immediately.

## Recommended First Implementation Slice

Build this first:

```text
wire listener
  -> hello/ping
  -> insertOne
  -> findOne by _id
  -> TiDB JSON table
```

This slice validates the entire critical path without prematurely committing to the full compatibility surface.
