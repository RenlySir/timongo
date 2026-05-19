#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUNDLE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
MIRROR_DIR="${1:-${BUNDLE_DIR}/tiup/mirror}"

if [[ ! -d "${MIRROR_DIR}" ]]; then
  echo "TiUP mirror directory not found: ${MIRROR_DIR}" >&2
  exit 1
fi

if ! command -v tiup >/dev/null 2>&1; then
  echo "tiup is required on PATH" >&2
  exit 1
fi

tiup mirror set "${MIRROR_DIR}"
echo "TiUP mirror set to ${MIRROR_DIR}"
