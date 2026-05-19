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
