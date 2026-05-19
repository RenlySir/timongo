#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <cluster-name> <topology.yaml> [tiup-cluster-args...]" >&2
  exit 2
fi

CLUSTER_NAME="$1"
TOPOLOGY="$2"
shift 2

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

"${SCRIPT_DIR}/install-local-mirror.sh"
"${SCRIPT_DIR}/deploy-tidb-cluster.sh" "${CLUSTER_NAME}" "${TOPOLOGY}" "$@"
tiup cluster start "${CLUSTER_NAME}"
"${SCRIPT_DIR}/deploy-timongo.sh" "${CLUSTER_NAME}" "${TOPOLOGY}" "${TIMONGO_SSH_USER:-tidb}"
