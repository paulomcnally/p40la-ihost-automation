#!/usr/bin/env bash
#
# Crea (o actualiza) un usuario de prueba en la DB y hace login dejando
# las cookies listas en /tmp/cookies.txt.
#
# Uso:
#   ./scripts/create-test-user.sh
#
# Variables de entorno (todas opcionales):
#   DB_PATH    ruta a la base SQLite (default: <repo>/data/app.db)
#   EMAIL      email del usuario (default: test@test.com)
#   PASSWORD   password del usuario (default: test1234)
#   PORT       puerto del server (default: 8089)
#   COOKIE_JAR archivo de cookies (default: /tmp/cookies.txt)
#
# Requisitos: go, sqlite3, curl

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

DB_PATH="${DB_PATH:-${ROOT_DIR}/data/app.db}"
EMAIL="${EMAIL:-test@test.com}"
PASSWORD="${PASSWORD:-test1234}"
PORT="${PORT:-8089}"
BASE_URL="${BASE_URL:-http://localhost:${PORT}}"
COOKIE_JAR="${COOKIE_JAR:-/tmp/cookies.txt}"

command -v go >/dev/null 2>&1 || { echo "ERROR: go no está instalado" >&2; exit 1; }
command -v sqlite3 >/dev/null 2>&1 || { echo "ERROR: sqlite3 no está instalado" >&2; exit 1; }
command -v curl >/dev/null 2>&1 || { echo "ERROR: curl no está instalado" >&2; exit 1; }

if [[ ! -f "${DB_PATH}" ]]; then
  echo "ERROR: no existe la base de datos ${DB_PATH}" >&2
  exit 1
fi

HASH="$(go run "${SCRIPT_DIR}/genhash.go" "${PASSWORD}")"

if [[ "${EMAIL}" =~ [[:space:]] ]] || [[ "${PASSWORD}" =~ [[:space:]] ]]; then
  echo "ERROR: EMAIL y PASSWORD no pueden contener espacios" >&2
  exit 1
fi

sqlite3 "${DB_PATH}" <<SQL
.parameter set ?1 ${EMAIL}
.parameter set ?2 ${HASH}
INSERT INTO users (email, password_hash) VALUES (?1, ?2)
ON CONFLICT(email) DO UPDATE SET password_hash=?2, updated_at=CURRENT_TIMESTAMP;
SQL

if ! curl -sf -o /dev/null -c "${COOKIE_JAR}" "${BASE_URL}/api/login" \
  -X POST -H 'Content-Type: application/json' \
  -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}"; then
  echo "ERROR: login falló contra ${BASE_URL} (¿el server está corriendo?)" >&2
  exit 1
fi

echo "Usuario ${EMAIL} listo. Cookies en ${COOKIE_JAR}."
echo "Uso: curl -b ${COOKIE_JAR} ${BASE_URL}/api/..."