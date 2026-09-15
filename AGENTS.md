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
- Frontend:
  ```bash
  cd frontend && npm install && npm run dev
  ```
- Datos de demo (crea usuario test + app/cuentas de ejemplo, idempotente):
  ```bash
  ./scripts/create-test-user.sh && ./scripts/seed-apps.sh
  ```
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

- `cmd/server/main.go` — entrypoint del backend Go.
- `internal/` — `api` (handlers/routes), `config`, `db` (SQLite + migraciones), `models`, `services`, `storage`.
- `frontend/` — SPA React: `src/pages`, `src/components`, `src/stores`, `src/api`, `src/i18n`.
- `migrations/` — archivos `.up.sql` / `.down.sql` numerados.
- `scripts/` — build, release, dev y test.
- `public/` — output del build de Vite (gitignored, lo genera `npm run build`).

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