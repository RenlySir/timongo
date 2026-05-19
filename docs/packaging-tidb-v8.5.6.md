# timongo + TiDB v8.5.6 Packaging

This document describes the packaging path for a timongo distribution that includes TiDB v8.5.6 install assets and timongo deployment assets.

## Package Shape

The generated tarball is:

```text
dist/timongo-tidb-v8.5.6-linux-amd64.tar.gz
```

It expands to:

```text
timongo-tidb-v8.5.6-linux-amd64/
  README.md
  bin/timongo-server
  manifest/timongo-bundle.json
  scripts/install-local-mirror.sh
  scripts/deploy-all.sh
  scripts/deploy-tidb-cluster.sh
  scripts/deploy-timongo.sh
  tiup/mirror/
  tiup/components/timongo/timongo-component.tar.gz
  tiup/examples/topology.yaml
  tiup/templates/
```

`tiup/mirror` is created by `tiup mirror clone <mirror-dir> v8.5.6 --os=linux --arch=amd64`.
TiUP resolves the exact versions of non-TiDB monitoring/tooling components that belong to the v8.5.6 package.

## Build

Smoke-test packaging without downloading the multi-GB TiDB mirror:

```bash
./scripts/package-timongo-tidb.sh --skip-mirror
```

Production packaging:

```bash
./scripts/package-timongo-tidb.sh
```

The production command downloads the TiDB v8.5.6 offline mirror and then creates the tarball.

## Deploy

On the deploy workstation:

```bash
tar -xzf timongo-tidb-v8.5.6-linux-amd64.tar.gz
cd timongo-tidb-v8.5.6-linux-amd64
./scripts/deploy-all.sh timongo-prod ./tiup/examples/topology.yaml --user tidb -p
```

For step-by-step control:

```bash
./scripts/install-local-mirror.sh
./scripts/deploy-tidb-cluster.sh timongo-prod ./tiup/examples/topology.yaml --user tidb -p
tiup cluster start timongo-prod
./scripts/deploy-timongo.sh timongo-prod ./tiup/examples/topology.yaml tidb
```

The topology file can contain normal TiUP Cluster sections plus:

```yaml
timongo_servers:
  - host: 10.0.1.10
    port: 27017
    status_port: 28017
    deploy_dir: /tidb-deploy/timongo-27017
    log_dir: /tidb-deploy/timongo-27017/log
    tidb_host: 10.0.1.20
    tidb_port: 4000
```

`timongo-server tiup split-topology` removes `timongo_servers` before calling `tiup cluster deploy`.
`timongo-server tiup render-timongo` renders `timongo.toml` and `timongo.service`, preserving the one-to-one binding between one timongo-server and one tidb-server.

## Current Boundary

This package provides one-command TiDB deployment plus one-command timongo deployment from the same offline bundle. It does not patch the upstream `tiup cluster` component to natively understand `timongo_servers`; that would require maintaining a TiUP Cluster fork or upstreaming a new component type.
