#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <cluster-name> <topology.yaml> [tiup-cluster-args...]" >&2
  exit 2
fi

CLUSTER_NAME="$1"
TOPOLOGY="$2"
shift 2

TIDB_VERSION="${TIDB_VERSION:-v8.5.6}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUNDLE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
TIMONGO_DEPLOY_ROOT="${TIMONGO_DEPLOY_ROOT:-${BUNDLE_DIR}/work/${CLUSTER_NAME}}"
TIDB_TOPOLOGY="${TIMONGO_DEPLOY_ROOT}/tidb-topology.yaml"

mkdir -p "${TIMONGO_DEPLOY_ROOT}"

"${BUNDLE_DIR}/bin/timongo-server" tiup split-topology \
  -input "${TOPOLOGY}" \
  -tidb-output "${TIDB_TOPOLOGY}" \
  -timongo-output "${TIMONGO_DEPLOY_ROOT}/timongo-topology.yaml"

tiup cluster deploy "${CLUSTER_NAME}" "${TIDB_VERSION}" "${TIDB_TOPOLOGY}" "$@"
