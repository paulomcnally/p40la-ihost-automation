#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-}"
IMAGE="paulomcnally/p40la-ihost-automation"
PLATFORMS="linux/amd64,linux/arm64,linux/arm/v7"

if [[ -z "${VERSION}" ]]; then
  echo "Uso: $0 <version>"
  echo "Ejemplo: $0 0.1.0"
  exit 1
fi

# Validar formato semver simple: MAJOR.MINOR.PATCH
if ! [[ "${VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Error: la versión debe seguir semver (ej. 0.1.0)"
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

# Verificar que el tag de Git exista localmente
if ! git tag -l "v${VERSION}" | grep -q "v${VERSION}"; then
  echo "Error: no existe el tag git v${VERSION}. Crea el tag primero."
  exit 1
fi

# Verificar que la imagen no exista ya en Docker Hub
echo "Verificando si ${IMAGE}:${VERSION} ya existe en Docker Hub..."
if docker manifest inspect "${IMAGE}:${VERSION}" > /dev/null 2>&1; then
  echo "Error: ${IMAGE}:${VERSION} ya existe en Docker Hub. No se permite sobrescribir."
  exit 1
fi

# Verificar que exista un builder de buildx
BUILDER="p40la-ihost-automation-builder"
if ! docker buildx inspect "${BUILDER}" > /dev/null 2>&1; then
  echo "Creando builder docker buildx: ${BUILDER}"
  docker buildx create --name "${BUILDER}" --use
else
  docker buildx use "${BUILDER}"
fi

echo "Construyendo y publicando ${IMAGE}:${VERSION} y ${IMAGE}:latest para ${PLATFORMS}..."
docker buildx build \
  --platform "${PLATFORMS}" \
  --tag "${IMAGE}:${VERSION}" \
  --tag "${IMAGE}:latest" \
  --push \
  .

echo "Publicación completada: ${IMAGE}:${VERSION}"