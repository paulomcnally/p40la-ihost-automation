#!/usr/bin/env bash
#
# Siembra datos de demo de apps y cuentas en la base SQLite (idempotente).
# Útil para no tener que crear cuentas a mano cada vez que se prueba.
# Registra también el plugin en el catálogo y lo asocia a las cuentas.
#
# Uso:
#   ./scripts/seed-apps.sh
#
# Variables de entorno (todas opcionales):
#   DB_PATH        ruta a la base SQLite (default: <repo>/data/app.db)
#   APP_NAME       nombre de la app demo (default: "Claro Nicaragua")
#   CREDENTIALS    JSON con las credenciales (default: usuario demo claro)
#   IDENTIFIERS    lista de identificadores (default: "0000000")
#   PLUGIN_NAME    plugin a registrar/asociar (default: "claro.nicaragua")
#   PLUGIN_VERSION versión del plugin (default: "1.2.0")
#   PLUGIN_SOURCE  fuente del plugin (default: "https://<DOMINIO_CLARO_API>")
#
# Ejemplo DISNORTE-DISSUR (credenciales por entorno, nunca hardcodeadas):
#   PLUGIN_NAME=disnorte.dissur.nicaragua PLUGIN_VERSION=1.0.0 \
#   PLUGIN_SOURCE=https://<DOMINIO_DISNORTE> \
#   APP_NAME="DISNORTE-DISSUR Nicaragua" IDENTIFIERS=0000000 \
#   CREDENTIALS='{"pin":"<PIN>"}' ./scripts/seed-apps.sh
#
# Ejemplo ASSA (credenciales por entorno, nunca hardcodeadas):
#   PLUGIN_NAME=assa.nicaragua PLUGIN_VERSION=1.0.0 \
#   PLUGIN_SOURCE=https://<DOMINIO_ASSA> \
#   APP_NAME="ASSA Nicaragua" IDENTIFIERS=<POLIZA> \
#   CREDENTIALS='{"username":"<USUARIO>","password":"<CLAVE>"}' ./scripts/seed-apps.sh
#
# Requisitos: sqlite3

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

DB_PATH="${DB_PATH:-${ROOT_DIR}/data/app.db}"
APP_NAME="${APP_NAME:-Claro Nicaragua}"
if [[ -z "${CREDENTIALS:-}" ]]; then
  CREDENTIALS='{"username":"demo@claro.ni","password":"demo1234"}'
fi
IDENTIFIERS="${IDENTIFIERS:-0000000}"
PLUGIN_NAME="${PLUGIN_NAME:-claro.nicaragua}"
PLUGIN_VERSION="${PLUGIN_VERSION:-1.2.0}"
PLUGIN_SOURCE="${PLUGIN_SOURCE:-https://<DOMINIO_CLARO_API>}"
PLUGIN_DESCRIPTION="${PLUGIN_DESCRIPTION:-}"
if [[ -z "${PLUGIN_DESCRIPTION}" ]]; then
  case "${PLUGIN_NAME}" in
    claro.nicaragua)
      PLUGIN_DESCRIPTION="Consulta de facturas de Mi Claro Nicaragua" ;;
    tigo.nicaragua)
      PLUGIN_DESCRIPTION="Consulta de saldo y facturas de Mi Cuenta Tigo Nicaragua" ;;
    disnorte.dissur.nicaragua)
      PLUGIN_DESCRIPTION="Consulta de recibos de energía eléctrica de DISNORTE-DISSUR Nicaragua" ;;
    enacal.nicaragua)
      PLUGIN_DESCRIPTION="Consulta de facturas de agua potable de ENACAL Nicaragua" ;;
    assa.nicaragua)
      PLUGIN_DESCRIPTION="Consulta de pólizas y facturas de ASSA Nicaragua" ;;
    *)
      PLUGIN_DESCRIPTION="Consulta de facturas" ;;
  esac
fi

command -v sqlite3 >/dev/null 2>&1 || { echo "ERROR: sqlite3 no está instalado" >&2; exit 1; }

if [[ ! -f "${DB_PATH}" ]]; then
  echo "ERROR: no existe la base de datos ${DB_PATH}" >&2
  exit 1
fi

# Verifica que CREDENTIALS sea JSON válido
if ! python3 -c "import json,sys; json.loads(sys.argv[1])" "${CREDENTIALS}" 2>/dev/null; then
  echo "ERROR: CREDENTIALS no es un JSON válido" >&2
  exit 1
fi

# Registra el plugin en el catálogo (idempotente)
sqlite3 "${DB_PATH}" "INSERT INTO plugins (name, version, source, description, status)
  VALUES ('${PLUGIN_NAME}', '${PLUGIN_VERSION}', '${PLUGIN_SOURCE}', '${PLUGIN_DESCRIPTION}', 'active')
  ON CONFLICT(name) DO UPDATE SET
    version = excluded.version,
    source = excluded.source,
    description = excluded.description,
    status = excluded.status,
    updated_at = CURRENT_TIMESTAMP;"
echo "Plugin asegurado: ${PLUGIN_NAME} v${PLUGIN_VERSION}"

# Inserta la app si no existe y obtiene su id
APP_ID="$(sqlite3 "${DB_PATH}" "SELECT id FROM apps WHERE name='${APP_NAME}' LIMIT 1;")"
if [[ -z "${APP_ID}" ]]; then
  sqlite3 "${DB_PATH}" "INSERT INTO apps (name) VALUES ('${APP_NAME}');"
  APP_ID="$(sqlite3 "${DB_PATH}" "SELECT id FROM apps WHERE name='${APP_NAME}' LIMIT 1;")"
  echo "App creada: ${APP_NAME} (id ${APP_ID})"
else
  echo "App ya existía: ${APP_NAME} (id ${APP_ID})"
fi

# Inserta cada cuenta (INSERT OR IGNORE respeta UNIQUE(app_id, identifier))
# y le asocia el plugin.
COUNT=0
for IDENTIFIER in ${IDENTIFIERS}; do
  sqlite3 "${DB_PATH}" "INSERT OR IGNORE INTO accounts (app_id, identifier, credentials) VALUES (${APP_ID}, '${IDENTIFIER}', '${CREDENTIALS}');"
  sqlite3 "${DB_PATH}" "UPDATE accounts SET plugin_name='${PLUGIN_NAME}', updated_at=CURRENT_TIMESTAMP WHERE app_id=${APP_ID} AND identifier='${IDENTIFIER}';"
  COUNT=$((COUNT + 1))
done

echo "Cuentas aseguradas (${COUNT}): ${IDENTIFIERS} (plugin ${PLUGIN_NAME})"
echo "Seed de apps completado."