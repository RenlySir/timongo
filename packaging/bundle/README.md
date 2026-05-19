# timongo TiDB v8.5.6 Offline Bundle

This bundle contains a TiUP offline mirror for TiDB v8.5.6 plus timongo deployment assets.

## One-command Deploy

```bash
./scripts/deploy-all.sh timongo-test ./tiup/examples/topology.yaml --user tidb -p
```

## Step-by-step Deploy

Install the bundled TiUP mirror:

```bash
./scripts/install-local-mirror.sh
```

Deploy and start TiDB:

```bash
./scripts/deploy-tidb-cluster.sh timongo-test ./tiup/examples/topology.yaml --user tidb -p
tiup cluster start timongo-test
```

Deploy timongo:

```bash
./scripts/deploy-timongo.sh timongo-test ./tiup/examples/topology.yaml tidb
```

The topology file can contain normal TiUP Cluster sections such as `pd_servers`, `tidb_servers`,
`tikv_servers`, and `tiflash_servers`, plus timongo-specific sections:

```yaml
timongo_servers:
  - host: 10.0.1.10
    port: 27017
    status_port: 28017
    tidb_host: 10.0.1.20
    tidb_port: 4000
```

`deploy-tidb-cluster.sh` calls `timongo-server tiup split-topology` to strip the timongo-only
sections before calling `tiup cluster deploy`. `deploy-timongo.sh` renders timongo config and systemd units, copies the binary to each
`timongo_servers` host, and starts a stateless `timongo.service`.
