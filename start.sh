#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="${SCRIPT_DIR}"

SKIP_FRONTEND_BUILD="${SKIP_FRONTEND_BUILD:-0}"

if [ "${SKIP_FRONTEND_BUILD}" != "1" ]; then
  echo "Construyendo frontend..."
  (cd "${ROOT_DIR}/frontend" && npm run build)
fi

export DATA_DIR="${DATA_DIR:-${ROOT_DIR}/data}"
export PORT="${PORT:-8089}"
export LOG_LEVEL="${LOG_LEVEL:-info}"
export VERSION="${VERSION:-dev}"

mkdir -p "${DATA_DIR}"

cd "${ROOT_DIR}"
echo "Levantando p40la-ihost-automation en http://localhost:${PORT}"
go run ./cmd/server