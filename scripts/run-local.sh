#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

export DATA_DIR="${DATA_DIR:-${ROOT_DIR}/data}"
export PORT="${PORT:-8089}"
export LOG_LEVEL="${LOG_LEVEL:-info}"
export VERSION="${VERSION:-dev}"

mkdir -p "${DATA_DIR}"

cd "${ROOT_DIR}"
echo "Levantando p40la-ihost-automation en http://localhost:${PORT}"
go run ./cmd/server
