#!/bin/bash
# Build script para p40la-ihost-automation
# Uso: ./scripts/build.sh [tag] [builder]

set -e

TAG="${1:-latest}"
BUILDER="${2:-p40la-ihost-automation-builder}"
IMAGE="paulomcnally/p40la-ihost-automation:${TAG}"

echo "=== Build Docker: ${IMAGE} ==="
echo "Builder: ${BUILDER}"
echo ""

# Verificar que el builder existe
if ! docker buildx inspect "${BUILDER}" > /dev/null 2>&1; then
    echo "Builder '${BUILDER}' no existe. Creando..."
    docker buildx create --name "${BUILDER}" --driver docker-container --use
    docker buildx inspect --bootstrap
fi

echo "=== Fase 1/3: Build amd64 (rápido) ==="
docker buildx build \
    --builder "${BUILDER}" \
    --platform linux/amd64 \
    -t "${IMAGE}-amd64" \
    --load \
    . 2>&1 | while IFS= read -r line; do
        echo "[amd64] $line"
    done

echo ""
echo "=== Fase 2/3: Build arm64 + armv7 (cross-compile, lento) ==="
echo "Esto tarda ~10-15 min por cross-compilation con QEMU..."
docker buildx build \
    --builder "${BUILDER}" \
    --platform linux/arm64,linux/arm/v7 \
    -t "${IMAGE}" \
    --push \
    . 2>&1 | while IFS= read -r line; do
        echo "[arm] $line"
    done

echo ""
echo "=== Fase 3/3: Crear manifest multi-arch ==="
docker buildx imagetools inspect "${IMAGE}"

echo ""
echo "=== Build completado: ${IMAGE} ==="