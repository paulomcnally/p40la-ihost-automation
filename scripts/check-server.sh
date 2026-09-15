#!/usr/bin/env bash
#
# Verifica que el server responde con {"status":"ok"} en /health.
# Con reintentos (hasta ~10s). Si falla, muestra el final del log.
#
# Uso:
#   ./scripts/check-server.sh
#
# Variables de entorno (opcionales):
#   PORT     puerto del server (default: 8089)
#   LOG_FILE ruta del log (default: /tmp/p40la-ihost-automation-server.log)
#
# Requisitos: curl
#
# Exit 0 si el server responde, exit 1 si no.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

PORT="${PORT:-8089}"
BASE_URL="${BASE_URL:-http://localhost:${PORT}}"
LOG_FILE="${LOG_FILE:-/tmp/p40la-ihost-automation-server.log}"

command -v curl >/dev/null 2>&1 || { echo "ERROR: curl no está instalado" >&2; exit 1; }

for i in $(seq 1 10); do
  RESP="$(curl -sf "${BASE_URL}/health" 2>/dev/null || true)"
  if [[ "${RESP}" == *'"status":"ok"'* ]]; then
    echo "OK: ${BASE_URL}/health responde ${RESP}"
    exit 0
  fi
  sleep 1
done

echo "ERROR: el server no responde en ${BASE_URL}/health" >&2
if [[ -f "${LOG_FILE}" ]]; then
  echo "--- últimas líneas de ${LOG_FILE} ---" >&2
  tail -n 30 "${LOG_FILE}" >&2
fi
exit 1