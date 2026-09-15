# p40la-ihost-automation

Add-on para SONOFF iHost (eWeLink CUBE). Repositorio base: el proyecto arranca con el cascote del dashboard y una pantalla de settings vacía. Los menús/módulos se van agregando incrementalmente.

---

## Releasing a new Docker version via tag

Para publicar una nueva imagen Docker multi-arquitectura (`linux/amd64`, `linux/arm/v7`, `linux/arm64`) en Docker Hub, seguí estos pasos desde una terminal:

### 1. Actualizar la versión en `docker-compose.yml`

```bash
# Editá docker-compose.yml y cambiá la variable VERSION, por ejemplo:
# VERSION=0.1.0
```

### 2. Commitear el cambio de versión

```bash
git add docker-compose.yml
git commit -m "bump version to 0.1.0"
git push origin main
```

### 3. Crear y pushear el tag

```bash
export VERSION=0.1.0
git tag -a v$VERSION -m "Release v$VERSION"
git push origin v$VERSION
```

> El workflow `docker-publish.yml` se dispara automáticamente con cualquier tag `v*`. Construye las 3 arquitecturas, crea el manifest multi-arch y publica:
> - `paulomcnally/p40la-ihost-automation:$VERSION`
> - `paulomcnally/p40la-ihost-automation:latest`

### 4. Verificar el release (opcional)

```bash
docker buildx imagetools inspect paulomcnally/p40la-ihost-automation:$VERSION
```

---

## Desarrollo local

> **Requisito (SPEC-015):** las implementaciones de plugins viven en el repo
> privado `github.com/paulomcnally/p40la-ihost-automation-plugins`. Para que el
> backend compile necesitás acceso a ese repo:
>
> ```bash
> go env -w GOPRIVATE=github.com/paulomcnally/p40la-ihost-automation-plugins
> gh auth setup-git    # o un PAT con scope repo via git credential helper
> ```

```bash
# Backend (puerto 8089 para no chocar con el proyecto p40la-ihost en 8088)
go run ./cmd/server

# Frontend
cd frontend
npm install
npm run dev
```

El servidor expone la API en `http://localhost:8089`.

---

## Estructura del proyecto

- `cmd/server/` — punto de entrada del backend Go
- `internal/` — API, servicios, storage y modelos
- `frontend/` — aplicación React + Tailwind + Vite
- `migrations/` — migraciones SQLite
- `scripts/` — utilidades de build, release y dev