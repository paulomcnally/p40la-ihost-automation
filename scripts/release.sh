#!/usr/bin/env bash
#
# Script de release automático para p40la-ihost-automation en Docker Hub.
# Uso:
#   ./scripts/release.sh              # bump automático de patch
#   ./scripts/release.sh 0.5.0        # versión manual
#
# Requisitos: bash, curl, git, gh
# Opcionales: jq o python3 (para parsear JSON de Docker Hub)
#

set -euo pipefail

readonly REPO="paulomcnally/p40la-ihost-automation"
readonly DOCKER_HUB_API="https://hub.docker.com/v2/repositories/${REPO}/tags/?page_size=50"
readonly WORKFLOW_NAME="Build & Push Docker Image"
readonly GITHUB_REPO="paulomcnally/p40la-ihost-automation"

# -----------------------------------------------------------------------------
# Helpers
# -----------------------------------------------------------------------------

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

error() {
  printf '[%s] ERROR: %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" >&2
  exit 1
}

# Extrae la última versión semver pura (X.Y.Z) desde la respuesta JSON de Docker Hub.
parse_latest_version() {
  local response="$1"
  local versions

  if command -v jq >/dev/null 2>&1; then
    versions=$(echo "$response" | jq -r '.results[].name')
  elif command -v python3 >/dev/null 2>&1; then
    versions=$(echo "$response" | python3 -c '
import json, sys
data = json.load(sys.stdin)
for r in data.get("results", []):
    print(r.get("name", ""))
')
  else
    # Fallback portable a grep (funciona en GNU y BSD)
    versions=$(echo "$response" | grep -o '"name":"[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*"' | grep -o '[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*')
  fi

  echo "$versions" | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -1 || true
}

# Indica si una versión ya existe en el JSON de Docker Hub.
version_exists_in_docker_hub() {
  local response="$1"
  local version="$2"

  if command -v jq >/dev/null 2>&1; then
    echo "$response" | jq -e --arg v "$version" '.results[] | select(.name == $v)' >/dev/null 2>&1
  elif command -v python3 >/dev/null 2>&1; then
    echo "$response" | python3 -c "
import json, sys
data = json.load(sys.stdin)
for r in data.get('results', []):
    if r.get('name') == '${version}':
        sys.exit(0)
sys.exit(1)
" >/dev/null 2>&1
  else
    echo "$response" | grep -q "\"name\":\"${version}\""
  fi
}

# -----------------------------------------------------------------------------
# Validaciones de entorno
# -----------------------------------------------------------------------------

log "Validando herramientas necesarias..."
command -v curl >/dev/null 2>&1 || error "curl no está instalado"
command -v git >/dev/null 2>&1 || error "git no está instalado"
command -v gh >/dev/null 2>&1 || error "gh no está instalado. Instalalo desde: https://cli.github.com/"

log "Validando autenticación con gh..."
gh auth status >/dev/null 2>&1 || error "gh no está autenticado. Ejecutá: gh auth login"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

CURRENT_BRANCH=$(git branch --show-current)
[[ "${CURRENT_BRANCH}" == "main" ]] || error "No estás en la rama main. Rama actual: ${CURRENT_BRANCH}"

log "Validando que no haya cambios sin commitear..."
if ! git diff --quiet || ! git diff --cached --quiet; then
  error "El working tree tiene cambios sin commitear. Commitea o stash antes de continuar."
fi

# -----------------------------------------------------------------------------
# Validar que no haya ramas de spec con código sin mergear a main
# -----------------------------------------------------------------------------
# La imagen se construye desde el tag generado sobre main. Si una feature
# branch tiene commits que main no contiene, ese código NO entraría en la
# imagen y el usuario vería una release sin los cambios esperados.
# (Precedente: SPEC-043 liberado en v0.4.12 sin su implementación.)

log "Validando que no haya ramas de feature con código sin mergear a main..."
UNMERGED_BRANCHES=$(git branch --no-merged main --format='%(refname:short)' | grep -v '^main$' || true)
if [[ -n "${UNMERGED_BRANCHES}" ]]; then
  error "Hay ramas de feature con código que main NO incluye:
$(echo "${UNMERGED_BRANCHES}" | sed 's/^/  - /')

La imagen se construye desde el tag sobre main, por lo que ese código NO se incluiría.
Mergeá cada rama a main (o liberá su spec) ANTES de correr release.sh."
fi

# -----------------------------------------------------------------------------
# Consultar Docker Hub
# -----------------------------------------------------------------------------

log "Consultando tags en Docker Hub: ${REPO}..."
RESPONSE=$(curl -fsSL "${DOCKER_HUB_API}" 2>/dev/null) || error "No se pudo consultar Docker Hub API en ${DOCKER_HUB_API}"

LATEST_VERSION=$(parse_latest_version "$RESPONSE")

if [[ -n "${LATEST_VERSION}" ]]; then
  log "Última versión publicada en Docker Hub: ${LATEST_VERSION}"
else
  log "No hay versiones publicadas en Docker Hub (primera release)."
fi

# -----------------------------------------------------------------------------
# Calcular nueva versión
# -----------------------------------------------------------------------------

if [[ $# -ge 1 ]]; then
  NEW_VERSION="$1"
  if ! [[ "${NEW_VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    error "Versión inválida: ${NEW_VERSION}. Debe seguir semver (X.Y.Z)."
  fi
  log "Usando versión manual: ${NEW_VERSION}"
elif [[ -n "${LATEST_VERSION}" ]]; then
  IFS='.' read -r MAJOR MINOR PATCH <<< "${LATEST_VERSION}"
  NEW_PATCH=$((PATCH + 1))
  NEW_VERSION="${MAJOR}.${MINOR}.${NEW_PATCH}"
  log "Calculando bump automático de patch: ${LATEST_VERSION} → ${NEW_VERSION}"
else
  INITIAL_VERSION=$(jq -r '.version' frontend/package.json 2>/dev/null || echo "0.1.0")
  NEW_VERSION="${INITIAL_VERSION}"
  log "Primera release: usando versión inicial ${NEW_VERSION}"
fi

# -----------------------------------------------------------------------------
# Validar que la versión no exista
# -----------------------------------------------------------------------------

log "Verificando que v${NEW_VERSION} no exista como tag git..."
if git tag -l "v${NEW_VERSION}" | grep -q "v${NEW_VERSION}"; then
  error "El tag v${NEW_VERSION} ya existe localmente"
fi

log "Sincronizando tags con origin..."
git fetch --tags origin >/dev/null 2>&1 || error "No se pudo hacer fetch de tags desde origin"

if git ls-remote --tags origin "refs/tags/v${NEW_VERSION}" | grep -q "refs/tags/v${NEW_VERSION}"; then
  error "El tag v${NEW_VERSION} ya existe en origin"
fi

log "Verificando que ${NEW_VERSION} no exista en Docker Hub..."
if version_exists_in_docker_hub "$RESPONSE" "${NEW_VERSION}"; then
  error "La versión ${NEW_VERSION} ya existe en Docker Hub"
fi

# -----------------------------------------------------------------------------
# Actualizar archivos
# -----------------------------------------------------------------------------

log "Actualizando docker-compose.yml (VERSION=${NEW_VERSION})..."
sed -i.bak "s/\(- VERSION=\).*/\1${NEW_VERSION}/" docker-compose.yml
rm -f docker-compose.yml.bak

log "Actualizando frontend/package.json (version=${NEW_VERSION})..."
if command -v jq >/dev/null 2>&1; then
  TMP_PKG=$(mktemp)
  jq --arg v "${NEW_VERSION}" '.version = $v' frontend/package.json > "${TMP_PKG}"
  mv "${TMP_PKG}" frontend/package.json
else
  sed -i.bak "s/\"version\": \"[^\"]*\"/\"version\": \"${NEW_VERSION}\"/" frontend/package.json
  rm -f frontend/package.json.bak
fi

if git diff --quiet; then
  error "No se detectaron cambios después de actualizar la versión"
fi

# -----------------------------------------------------------------------------
# Commit y tag
# -----------------------------------------------------------------------------

log "Creando commit de bump..."
git add docker-compose.yml frontend/package.json
git commit -m "bump version to ${NEW_VERSION}"

log "Creando tag anotado v${NEW_VERSION}..."
git tag -a "v${NEW_VERSION}" -m "Release v${NEW_VERSION}"

# -----------------------------------------------------------------------------
# Push
# -----------------------------------------------------------------------------

log "Pusheando commit a origin/main..."
git push origin main

log "Pusheando tag v${NEW_VERSION}..."
git push origin "v${NEW_VERSION}"

# -----------------------------------------------------------------------------
# Obtener URL del action con gh
# -----------------------------------------------------------------------------

log "Esperando que GitHub Actions registre el workflow disparado por el tag..."
sleep 5

RUN_URL=""
if command -v jq >/dev/null 2>&1; then
  RUN_URL=$(gh run list \
    --workflow="${WORKFLOW_NAME}" \
    --event tag \
    --json url,headBranch \
    --limit 20 2>/dev/null \
    | jq -r --arg tag "v${NEW_VERSION}" '[.[] | select(.headBranch == $tag)][0].url // empty' || true)
fi

if [[ -z "${RUN_URL}" ]]; then
  log "Primer intento fallido, reintentando en 5 segundos..."
  sleep 5
  if command -v jq >/dev/null 2>&1; then
    RUN_URL=$(gh run list \
      --workflow="${WORKFLOW_NAME}" \
      --event tag \
      --json url,headBranch \
      --limit 20 2>/dev/null \
      | jq -r --arg tag "v${NEW_VERSION}" '[.[] | select(.headBranch == $tag)][0].url // empty' || true)
  fi
fi

# -----------------------------------------------------------------------------
# Resumen
# -----------------------------------------------------------------------------

echo ""
log "=============================================="
log "Release v${NEW_VERSION} iniciado correctamente"
log "Docker Hub: https://hub.docker.com/r/${REPO}/tags"

if [[ -n "${RUN_URL}" && "${RUN_URL}" != "null" ]]; then
  log "GitHub Action: ${RUN_URL}"
else
  log "GitHub Actions: https://github.com/${GITHUB_REPO}/actions"
  log "(No se pudo obtener la URL exacta todavía; revisá manualmente en unos segundos)"
fi

log "=============================================="
echo ""
