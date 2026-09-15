# AGENTS.md

Guía de contexto para agentes de IA que trabajen en este repositorio.

## Proyecto

`p40la-ihost-automation` — add-on para SONOFF iHost (eWeLink CUBE). La app es un
SPA (React + Vite + Tailwind) con backend en Go (API + SQLite) servido todo por un
mismo binario.

Estado actual: el proyecto arranca como **cascote base**:
- Autenticación completa (Setup inicial + Login + Logout).
- Dashboard (pantalla placeholder vacía).
- Settings vacío.

Los módulos/menús se agregan incrementalmente a pedido del usuario.

## Comandos

- Backend (puerto por defecto `8089`, para no chocar con el proyecto `p40la-ihost` que usa `8088`):
  ```bash
  go run ./cmd/server
  ```
- **Requisito previo (SPEC-015)**: los plugins viven en el repo privado
  `github.com/paulomcnally/p40la-ihost-automation-plugins`. Para compilar hace falta:
  ```bash
  go env -w GOPRIVATE=github.com/paulomcnally/p40la-ihost-automation-plugins
  gh auth setup-git   # o PAT con scope repo via git credential helper
  ```
  Si `go build` falla con "no required module provides package ...-plugins", es
  por falta de credenciales o `GOPRIVATE`, no por el código.
- Frontend:
  ```bash
  cd frontend && npm install && npm run dev
  ```
- Datos de demo (crea usuario test + app/cuentas de ejemplo, idempotente):
  ```bash
  ./scripts/create-test-user.sh && ./scripts/seed-apps.sh
  ```
  `seed-apps.sh` exige `PLUGIN_SOURCE` por entorno (los dominios no viven en este repo).
- Build de producción del frontend (vuelca a `../public`):
  ```bash
  cd frontend && npm run build
  ```
- Verificación:
  ```bash
  go build ./...     # compila el backend
  npm run build      # typecheck + build del frontend
  ```
- Docker:
  ```bash
  gh auth token > .gh_token    # requisito (SPEC-015): el build usa este token para bajar el módulo privado
  docker compose up --build     # build + levantar
  ```

## Puertos

- API/servidor: `8089` (variable `PORT`). **No usar 8088** (lo usa el proyecto hermano `p40la-ihost`).
- Vite dev proxy `/api` y `/health` → `http://localhost:8089`.

## Cookie de sesión

- La cookie de sesión usa un nombre propio por proyecto para evitar colisiones en el navegador:
  `p40la_ihost_automation_session` (configurable vía env `COOKIE_NAME`).
- El proyecto hermano `p40la-ihost` usa la cookie `session`; si se cambia el nombre, las sesiones
  activas con el nombre anterior se invalidan una sola vez.

## Estructura

- `cmd/server/main.go` — entrypoint del backend Go (registra plugins vía `pluginall.RegisterAll` del módulo privado).
- `internal/` — `api` (handlers/routes), `config`, `db` (SQLite + migraciones), `models`, `services`, `storage`; `plugins` es solo un re-export (aliases) del contrato del módulo privado.
- `frontend/` — SPA React: `src/pages`, `src/components`, `src/stores`, `src/api`, `src/i18n`.
- `migrations/` — archivos `.up.sql` / `.down.sql` numerados.
- `scripts/` — build, release, dev y test.
- `public/` — output del build de Vite (gitignored, lo genera `npm run build`).

## Plugins de integración (módulo privado) — REGLAS OBLIGATORIAS

Las implementaciones de plugins (contrato + integraciones + dominios) viven en
el repo **privado** `github.com/paulomcnally/p40la-ihost-automation-plugins`
(SPEC-015). Este repo público **no debe contener** dominios ni know-how de
proveedores.

### Al agregar un plugin nuevo (o tocar uno existente)

1. El trabajo se hace en el **repo privado**, no en este. El contrato está en
   `plugins/` (módulo privado); este repo solo re-exporta aliases en
   `internal/plugins/plugins.go`.
2. **Todo literal de dominio/URL de proveedor DEBE ir cifrado** con
   `secret.Must`, nunca en claro (aplica a código de producción y a scripts;
   en tests del repo privado se permite el valor claro, pero es preferible
   `secret.Must`):
   ```bash
   # en el repo privado
   go run ./cmd/gensecret 'https://proveedor.ejemplo/ruta'   # emite el blob
   ```
   La llave vive partida en `secret/key.go` del repo privado; no tocarla.
3. La nueva implementación se registra en `all/register.go` del repo privado.
   **No** se agrega ningún import de implementación en `cmd/server/main.go`
   (este repo usa solo `pluginall.RegisterAll`).
4. Al terminar: tag semver en el repo privado (`git tag vX.Y.Z`) y actualizar
   `go.mod`/`go.sum` de este repo (`go get ...-plugins@vX.Y.Z`). Nunca usar
   pseudo-versiones de rama.
5. `scripts/tigo-auth.sh` y otros scripts que toquen dominios/clientes viven en
   el repo privado; aquí solo `seed-apps.sh` con `PLUGIN_SOURCE` obligatoria.

### Build, CI y secretos

- Local: `go env -w GOPRIVATE=github.com/paulomcnally/p40la-ihost-automation-plugins`
  + `gh auth setup-git` (o PAT con scope `repo`).
- Docker (`Dockerfile`): el token se pasa con `--mount=type=secret,id=gh_token`
  en `go mod download`. **Nunca** en `ARG`/`ENV` (quedaría en capas/historial).
- CI (`docker-publish.yml`): `secrets.PLUGINS_TOKEN` (PAT con lectura del repo
  privado) via `secrets:` del `docker/build-push-action`.
- **Prohibido** `go mod vendor` en este repo: copiaría el código privado al
  repo público.
- El stage builder de Go necesita `git` (lo agrega el Dockerfile) porque los
  módulos `GOPRIVATE` se bajan por VCS directo.

### Verificación tras tocar plugins

```bash
go build ./... && go test ./...
go build -o /tmp/server-check ./cmd/server && strings /tmp/server-check | rg -i 'proveedor'   # sin salidas en claro
rg -i 'proveedor' -g '!docs/specs/**' .   # sin salidas (docs/specs conserva dominios por decisión, ADR-004)
```

## Backend: agregar un módulo

Para agregar un nuevo menú/módulo (ej. hogares, servicios, deudas), el patrón es:
1. Nueva migración SQLite (`migrations/NNNN_*.up.sql`).
2. Modelo en `internal/models/`.
3. Storage en `internal/storage/`.
4. Servicio en `internal/services/`.
5. Handlers en `internal/api/` y registrarlos en `routes.go` (con `authMiddleware`).
6. Frontend: página en `src/pages/`, ruta en `src/App.tsx`, entrada en `src/components/Sidebar.tsx`, endpoints en `src/api/index.ts`, traducciones en `frontend/public/i18n/{es,en}.json` (fuente de verdad; `npm run build` las sincroniza a `src/i18n`).

## Specs

Cada feature nueva se documenta con una spec en `docs/specs/` (formato `SPEC-XXX`),
gestionada por la skill `spec-manager` (comandos `/spec create|list|status|show`).
Regla de oro: **no escribir código significativo sin una spec existente**. El tracker
(`docs/specs/README.md`) es la fuente del contador de IDs y estados.
Regla de commits: **cuando una spec pasa a `released`, el agente DEBE hacer commit**
(código + spec + tracker README) en la misma sesión, sin esperar a que el usuario lo pida.

## Convenciones

- Idioma de código/comentarios/mensajes: **español**.
- Backend: handlers finos, servicios con la lógica de negocio, storage solo SQL.
- Frontend: stores de Zustand, estilos con Tailwind, `min-h-[44px]` en controles táctiles.
- i18n: la fuente de verdad es `frontend/public/i18n/*.json` (es + en). `npm run build` copia a `src/i18n`.
- No commitear el output del build (`public/`) ni `data/`.