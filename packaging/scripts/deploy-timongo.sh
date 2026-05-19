#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <cluster-name> <topology.yaml> [ssh-user]" >&2
  exit 2
fi

CLUSTER_NAME="$1"
TOPOLOGY="$2"
SSH_USER="${3:-tidb}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUNDLE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
WORK_DIR="${BUNDLE_DIR}/work/${CLUSTER_NAME}"
RENDER_DIR="${WORK_DIR}/timongo-rendered"

mkdir -p "${WORK_DIR}" "${RENDER_DIR}"

"${BUNDLE_DIR}/bin/timongo-server" tiup render-timongo \
  -input "${TOPOLOGY}" \
  -output "${RENDER_DIR}"

while IFS=$'\t' read -r host deploy_dir log_dir; do
  [[ -n "${host}" ]] || continue
  ssh ${TIMONGO_SSH_OPTS:-} "${SSH_USER}@${host}" "mkdir -p '${deploy_dir}/bin' '${deploy_dir}/conf' '${log_dir}'"
  scp ${TIMONGO_SSH_OPTS:-} "${BUNDLE_DIR}/bin/timongo-server" "${SSH_USER}@${host}:${deploy_dir}/bin/timongo"
  scp ${TIMONGO_SSH_OPTS:-} "${RENDER_DIR}/${host}/timongo.toml" "${SSH_USER}@${host}:${deploy_dir}/conf/timongo.toml"
  scp ${TIMONGO_SSH_OPTS:-} "${RENDER_DIR}/${host}/timongo.service" "${SSH_USER}@${host}:/tmp/timongo.service"
  ssh ${TIMONGO_SSH_OPTS:-} "${SSH_USER}@${host}" "chmod +x '${deploy_dir}/bin/timongo' && sudo mv /tmp/timongo.service /etc/systemd/system/timongo.service && sudo systemctl daemon-reload && sudo systemctl enable --now timongo.service"
done < "${RENDER_DIR}/hosts.tsv"

echo "timongo servers deployed for cluster ${CLUSTER_NAME}"
