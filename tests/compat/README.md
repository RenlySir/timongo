# timongo Compatibility Harness

This directory contains the M0 scaffold for differential testing.

The target comparison set is:

1. MongoDB 6.0
2. FerretDB-compatible behavior where reusable under Apache-2.0 terms
3. timongo connected to TiDB

M0 only establishes the harness location and local services. M1 adds executable driver tests for CRUD and command compatibility.

The harness must not copy MongoDB server SSPL source code into this repository.
