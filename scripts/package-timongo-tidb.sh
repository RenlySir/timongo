#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TIDB_VERSION="${TIDB_VERSION:-v8.5.6}"
TIMONGO_VERSION="${TIMONGO_VERSION:-dev}"
TARGET_OS="${TARGET_OS:-linux}"
TARGET_ARCH="${TARGET_ARCH:-amd64}"
OUTPUT_DIR="${OUTPUT_DIR:-${ROOT_DIR}/dist}"
SKIP_MIRROR=0
DRY_RUN=0
JOBS="${JOBS:-4}"

usage() {
  cat <<'USAGE'
Usage: scripts/package-timongo-tidb.sh [options]

Options:
  --tidb-version <version>    TiDB version to package. Default: v8.5.6
  --timongo-version <version> timongo component version. Default: dev
  --os <os>                   Target OS for TiUP mirror and binary. Default: linux
  --arch <arch>               Target architecture. Default: amd64
  --output <dir>              Output directory. Default: ./dist
  --skip-mirror               Do not download TiDB TiUP mirror, useful for CI smoke tests.
  --dry-run                   Print planned paths and commands without writing package files.
  -h, --help                  Show this help.

The full v8.5.6 mirror is large. Use --skip-mirror only for validating the timongo package path;
production bundles must include the generated tiup/mirror directory.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --tidb-version)
      TIDB_VERSION="$2"
      shift 2
      ;;
    --timongo-version)
      TIMONGO_VERSION="$2"
      shift 2
      ;;
    --os)
      TARGET_OS="$2"
      shift 2
      ;;
    --arch)
      TARGET_ARCH="$2"
      shift 2
      ;;
    --output)
      OUTPUT_DIR="$2"
      shift 2
      ;;
    --skip-mirror)
      SKIP_MIRROR=1
      shift
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ "${TIDB_VERSION}" != "v8.5.6" ]]; then
  echo "warning: this package profile is validated for TiDB v8.5.6, got ${TIDB_VERSION}" >&2
fi

BUNDLE_NAME="timongo-tidb-${TIDB_VERSION}-${TARGET_OS}-${TARGET_ARCH}"
BUNDLE_DIR="${OUTPUT_DIR}/${BUNDLE_NAME}"
MIRROR_DIR="${BUNDLE_DIR}/tiup/mirror"
TIMONGO_COMPONENT_DIR="${OUTPUT_DIR}/timongo-component-${TIMONGO_VERSION}-${TARGET_OS}-${TARGET_ARCH}"
TIMONGO_COMPONENT_TAR="${BUNDLE_DIR}/tiup/components/timongo/timongo-component.tar.gz"
PACKAGE_TAR="${OUTPUT_DIR}/${BUNDLE_NAME}.tar.gz"

mirror_clone_cmd=(
  tiup mirror clone "${MIRROR_DIR}" "${TIDB_VERSION}"
  "--os=${TARGET_OS}" "--arch=${TARGET_ARCH}" "--jobs=${JOBS}"
)

if [[ "${DRY_RUN}" -eq 1 ]]; then
  echo "bundle: ${BUNDLE_DIR}"
  echo "archive: ${PACKAGE_TAR}"
  echo "mirror clone: ${mirror_clone_cmd[*]}"
  echo "timongo build: GOOS=${TARGET_OS} GOARCH=${TARGET_ARCH} go build ./cmd/timongo"
  exit 0
fi

rm -rf "${BUNDLE_DIR}" "${TIMONGO_COMPONENT_DIR}"
mkdir -p \
  "${BUNDLE_DIR}/bin" \
  "${BUNDLE_DIR}/manifest" \
  "${BUNDLE_DIR}/scripts" \
  "${BUNDLE_DIR}/tiup/components/timongo" \
  "${BUNDLE_DIR}/tiup/examples" \
  "${BUNDLE_DIR}/tiup/templates" \
  "${OUTPUT_DIR}"

GOOS="${TARGET_OS}" GOARCH="${TARGET_ARCH}" CGO_ENABLED=0 \
  go build -trimpath -ldflags "-s -w -X main.version=${TIMONGO_VERSION}" \
  -o "${BUNDLE_DIR}/bin/timongo-server" ./cmd/timongo

mkdir -p "${TIMONGO_COMPONENT_DIR}"
cp "${BUNDLE_DIR}/bin/timongo-server" "${TIMONGO_COMPONENT_DIR}/timongo-server"
tar -C "${TIMONGO_COMPONENT_DIR}" -czf "${TIMONGO_COMPONENT_TAR}" timongo-server

cp "${ROOT_DIR}/packaging/bundle/README.md" "${BUNDLE_DIR}/README.md"
cp "${ROOT_DIR}/packaging/scripts/"*.sh "${BUNDLE_DIR}/scripts/"
cp "${ROOT_DIR}/tiup/templates/"*.tmpl "${BUNDLE_DIR}/tiup/templates/"
cp "${ROOT_DIR}/tiup/examples/topology.yaml" "${BUNDLE_DIR}/tiup/examples/topology.yaml"
chmod +x "${BUNDLE_DIR}/scripts/"*.sh

cat > "${BUNDLE_DIR}/manifest/timongo-bundle.json" <<EOF
{
  "tidb_version": "${TIDB_VERSION}",
  "timongo_version": "${TIMONGO_VERSION}",
  "os": "${TARGET_OS}",
  "arch": "${TARGET_ARCH}",
  "bundle": "${BUNDLE_NAME}",
  "contains_tiup_mirror": $([[ "${SKIP_MIRROR}" -eq 0 ]] && echo true || echo false),
  "components": [
    "tidb-community-server-${TIDB_VERSION}",
    "tidb-community-toolkit-${TIDB_VERSION}",
    "timongo"
  ]
}
EOF

if [[ "${SKIP_MIRROR}" -eq 0 ]]; then
  if ! command -v tiup >/dev/null 2>&1; then
    echo "tiup is required to build a production offline mirror; rerun with --skip-mirror for smoke tests" >&2
    exit 1
  fi
  "${mirror_clone_cmd[@]}"
else
  mkdir -p "${MIRROR_DIR}"
  cat > "${MIRROR_DIR}/MIRROR_NOT_INCLUDED.txt" <<EOF
This smoke-test bundle was built with --skip-mirror.
Rebuild without --skip-mirror to include the TiDB ${TIDB_VERSION} TiUP offline mirror.
EOF
fi

tar -C "${OUTPUT_DIR}" -czf "${PACKAGE_TAR}" "${BUNDLE_NAME}"
rm -rf "${TIMONGO_COMPONENT_DIR}"

echo "${PACKAGE_TAR}"
