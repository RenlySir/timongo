# MongoDB 6.0 Compatibility Matrix

Last local run:

- Date: 2026-05-19
- Tool: `/Users/lan/Documents/mongo/target/mongo-usage-collector.jar compat-test`
- Target: `mongodb://127.0.0.1:27019/admin?directConnection=true`
- Backend: TiDB on `127.0.0.1:34000`
- Result: `total=182, passed=165, failed=14, skipped=3`
- Report: `tests/compat/out-timongo/compat-test-report.json` (ignored generated output)

Status values:

- `supported`: implemented and covered by automated compatibility tests
- `partial`: implemented with documented limitations
- `planned`: accepted scope for a later milestone
- `unsupported`: not supported
- `intentionally-unsupported`: deliberately outside timongo scope

## Wire Protocol

| Feature | Status | Milestone | Notes |
| --- | --- | --- | --- |
| OP_MSG | partial | M0 | Command path, document sequences, MinKey/MaxKey decode covered |
| OP_QUERY handshake | partial | M0 | Legacy handshake path covered |
| Compression | planned | M1 | Negotiation and codecs not implemented |
| Sessions | planned | M1 | Needed for driver sessions and transactions |
| Cursors/getMore | planned | M2 | Current covered tests return firstBatch only |

## Commands

| Command | Status | Milestone | Notes |
| --- | --- | --- | --- |
| hello | partial | M0 | Basic driver handshake |
| isMaster / ismaster | partial | M0 | Legacy alias |
| ping | supported | M0 | Covered by compat-test |
| buildInfo | partial | M0 | timongo metadata only |
| serverStatus | partial | M0 | timongo metadata only |
| create/listCollections/drop/dropDatabase | partial | M0 | Collection metadata and validators are persisted |
| createIndexes/listIndexes/dropIndexes | partial | M0 | Metadata only; no physical secondary index enforcement yet |
| insert | partial | M0 | `_id`, Extended JSON persistence, and JSON schema subset covered |
| find | partial | M0 | Filter, sort, skip, limit, projection, arrays, dot paths covered for common subset |
| count/distinct | partial | M0 | Common filters covered |
| update/delete | partial | M0 | Common update operators covered |
| findAndModify | partial | M0 | findOneAndUpdate/findOneAndDelete/findOneAndReplace subset covered |
| aggregate | partial | M0 | `$match`, `$project`, `$addFields`, `$set`, `$unset`, `$limit`, `$skip`, `$sort`, `$count`, `$group`, `$sortByCount`, `$unwind`, `$replaceRoot`, `$sample`, `$planCacheStats` covered |
| Admin/stat commands | partial | M0 | `connectionStatus`, `dbStats`, `collStats`, `hostInfo`, `getCmdLineOpts`, `getParameter`, `currentOp`, `top`, `profile`, `listCommands`, `usersInfo`, `rolesInfo`, `getDefaultRWConcern`, `validate`, `planCacheClear` return compatibility stubs |
| getMore | planned | M2 | Requires cursor persistence |
| killCursors | planned | M2 | Requires cursor persistence |

## Remaining Compat-Test Failures

| Category | Test IDs | Status | Notes |
| --- | --- | --- | --- |
| Advanced aggregation | `aggregation-lookup`, `aggregation-facet`, `aggregation-bucket`, `aggregation-union-with`, `aggregation-graph-lookup`, `aggregation-out`, `aggregation-merge`, `aggregation-set-window-fields` | planned | Requires cross-collection pipeline execution and write stages |
| Transactions | `transaction-commit`, `transaction-abort`, `transaction-read-your-writes` | planned | Requires session catalog, transaction pinning to one TiDB connection, and commit/abort semantics |
| Change streams | `change-stream-open`, `change-stream-pipeline`, `change-stream-full-document` | planned | Requires change event log or TiCDC-style source and resumable cursors |

## Explicit Non-Goals

| Feature | Status | Milestone | Notes |
| --- | --- | --- | --- |
| MongoDB replica set election protocol | intentionally-unsupported | none | TiDB provides HA at storage and SQL layers |
| MongoDB sharding internals | intentionally-unsupported | none | TiDB handles distribution |
| server-side JavaScript | intentionally-unsupported | none | Disabled for safety |
