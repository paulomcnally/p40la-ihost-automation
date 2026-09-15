---
title: "Cascote base: autenticación, dashboard y settings vacío"
id: "SPEC-001"
status: "released"
author: "paulomcnally"
created: "2026-09-11"
updated: "2026-09-11"
github_issue: 1
---

# Cascote base: autenticación, dashboard y settings vacío

**ID**: SPEC-001  
**Estado**: released  
**Autor**: paulomcnally  
**Creado**: 2026-09-11  
**Actualizado**: 2026-09-11

---

## 1. Resumen Ejecutivo

Este proyecto (`p40la-ihost-automation`) nace como una réplica recortada de `p40la-ihost`
(add-on para SONOFF iHost / eWeLink CUBE). A diferencia del proyecto original —que acumula
módulos como casas, servicios, facturas, autos, deudas y pensión alimenticia—, este
repositorio arranca con el **cascote base**: la autenticación completa, un dashboard
placeholder y una pantalla de settings vacía. Los módulos/menús se van agregando
incrementalmente a pedido del usuario.

La decisión de recortar es deliberada: cada menú nuevo se incorpora bajo demanda con su
propia spec, siguiendo el patrón documentado en `AGENTS.md` (migración → modelo → storage →
servicio → handlers → frontend). Esta spec es la **spec base** que documenta el estado
inicial ya existente en `main`, para dejar un punto de partida claro antes de las
próximas features.

El resultado esperado es un punto de partida estable y verificado: compila con `go build`,
typecheckea con `npm run build`, y la API responde correctamente (setup → login → logout).

## 2. Requerimientos

### 2.1 Requerimientos Funcionales (P0 - Obligatorios)

1. **REQ-001**: Autenticación completa: Setup inicial (crear único usuario admin), Login, Logout y validación de sesión por cookie.
2. **REQ-002**: Dashboard placeholder con sidebar, header y contenido vacío listo para futuros menús.
3. **REQ-003**: Pantalla de Settings vacía (solo título + estado vacío).
4. **REQ-004**: i18n es/en con mecanismo de diccionarios servidos (`/i18n/*.json`) y bundle de respaldo.
5. **REQ-005**: Backend Go con API HTTP + SQLite (migraciones versionadas).

### 2.2 Requerimientos Funcionales (P1 - Importantes)

1. **REQ-006**: Build de imagen Docker multi-arquitectura publicada como `paulomcnally/p40la-ihost-automation`.
2. **REQ-007**: Puerto por defecto `8089` para no chocar con el proyecto hermano `p40la-ihost` (usa `8088`).

### 2.3 Requerimientos Funcionales (P2 - Deseables)

1. **REQ-008**: Scripts de soporte (release, push docker, test user, check server).

### 2.4 Requerimientos No Funcionales

- **Rendimiento**: API ligera apta para iHost (bajo consumo de RAM/CPU).
- **Seguridad**: Passwords hasheados con bcrypt; cookie de sesión firmada con HMAC-SHA256 (HttpOnly, SameSite=Strict).
- **Almacenamiento**: SQLite con WAL; única migración `0001` (tablas `users` y `settings`).
- **Disponibilidad**: Healthcheck en `/health` (usado por el `HEALTHCHECK` del Dockerfile).
- **iHost**: Dependencias Go minimizadas (`golang.org/x/crypto`, `modernc.org/sqlite`); sin runtime pesado en el contenedor (distroless).

## 3. Investigación y Decisiones Técnicas

### 3.1 Contexto investigado

- Se tomó como base el repositorio `p40la-ihost` (proyecto hermano), replicando su arquitectura y convenciones.
- Se conservó el patrón de capas del backend: `api` (handlers/routes), `services` (lógica de negocio), `storage` (solo SQL), `models`.
- Frontend: React 19 + Vite 6 + Tailwind 3 + Zustand, idéntico al proyecto original.

### 3.2 Opciones evaluadas

| Opción | Pros | Contras | Decisión |
|--------|------|---------|----------|
| Replicar todo `p40la-ihost` completo | Menor esfuerzo inicial | Trae todos los módulos/tablas no deseados | ❌ Rechazada |
| Replicar solo el cascote base | Módulos se agregan bajo demanda | Más iteraciones futuras | ✅ Seleccionada |
| Mantener módulo Go `p40la-ihost` | Cero renombres | Confunde repos e imágenes | ❌ Rechazada |

### 3.3 Decisiones arquitectónicas (ADRs)

**ADR-001**: Proyecto recortado con crecimiento incremental
- **Contexto**: El usuario quiere un proyecto nuevo sin los módulos del original, agregándolos de a uno.
- **Decisión**: El repo arranca solo con auth + dashboard placeholder + settings vacío. Cada módulo nuevo se pide explícitamente.
- **Consecuencias**: Base pequeña y verificable; los módulos se agregan con su propia migración/spec.

**ADR-002**: Puerto por defecto 8089
- **Contexto**: El proyecto hermano `p40la-ihost` ocupa el 8088 y suele correr en paralelo.
- **Decisión**: `PORT=8089` por defecto (config, Dockerfile, docker-compose, proxy de Vite y scripts).
- **Consecuencias**: Sin choques de puerto; requiere recordar que el dev proxy apunta a 8089.

**ADR-003**: Imagen Docker con nombre propio del repo
- **Contexto**: El repositorio es `paulomcnally/p40la-ihost-automation`.
- **Decisión**: Imagen `paulomcnally/p40la-ihost-automation` en docker-compose, workflow `docker-publish.yml`, `scripts/build.sh` y `scripts/push-dockerhub.sh`.
- **Consecuencias**: Releases publican a un namespace de Docker Hub distinto al del proyecto original.

## 4. Diseño Técnico

### 4.1 Diagrama de arquitectura

```
[React SPA (Vite)] --(HTTP /api)--> [Go API (net/http)]
        |                                 |
        v                                 v
[ /i18n/*.json ]                  [ SQLite (WAL) + migraciones ]
```

### 4.2 Componentes

#### 4.2.1 Backend Go (`cmd/server`, `internal/`)
- **Responsabilidad**: API REST, autenticación, configuración y servir el SPA estático.
- **Interfaz**: Rutas públicas (`/health`, `/api/setup-status`, `/api/setup`, `/api/login`, `/api/logout`) y autenticadas (`/api/me`, `/api/settings`, `/api/settings/language`).
- **Dependencias**: `golang.org/x/crypto/bcrypt`, `modernc.org/sqlite`.
- **Ubicación**: `cmd/server/main.go`, `internal/{api,config,db,models,services,storage}`.

#### 4.2.2 Frontend React (`frontend/`)
- **Responsabilidad**: SPA con rutas `/login`, setup inicial y layout con sidebar (Dashboard + Settings).
- **Interfaz**: `src/App.tsx` (rutas + AuthGuard), `src/components/DashboardLayout.tsx`, `src/components/Sidebar.tsx`.
- **Dependencias**: react, react-router-dom, zustand.
- **Ubicación**: `frontend/src/**`.

### 4.3 Modelo de datos

```
users
- id: INTEGER PK AUTOINCREMENT
- email: TEXT UNIQUE NOT NULL
- password_hash: TEXT NOT NULL
- created_at / updated_at: DATETIME

settings
- key: TEXT PK
- value: TEXT NOT NULL
```

Migración única: `migrations/0001_create_users_and_settings.{up,down}.sql`.

### 4.4 APIs / Contratos

#### Endpoint: `GET /api/setup-status`

**Response 200**:
```json
{ "setup_completed": false }
```

#### Endpoint: `POST /api/setup`

**Request**:
```json
{ "email": "admin@example.com", "password": "test1234", "password_confirm": "test1234" }
```

**Response 201**:
```json
{ "user_id": 1, "email": "admin@example.com" }
```

#### Endpoint: `POST /api/login`

**Request**:
```json
{ "email": "admin@example.com", "password": "test1234", "remember": false }
```

**Response 200**:
```json
{ "email": "admin@example.com" }
```

#### Endpoint: `GET /api/me` (autenticado)

**Response 200**:
```json
{ "email": "admin@example.com" }
```

#### Endpoint: `GET /api/settings` (autenticado)

**Response 200**:
```json
{ "language": "es" }
```

#### Endpoint: `POST /api/settings/language` (autenticado)

**Request**:
```json
{ "language": "en" }
```

**Response 200**:
```json
{ "language": "en" }
```

**Response Error** (todos los endpoints):
```json
{ "error": "codigo", "message": "descripción" }
```

### 4.5 Dependencias

- **Internas**: Ninguna (no hay módulos de negocio todavía).
- **Externas**: `golang.org/x/crypto`, `modernc.org/sqlite` (Go); react, react-router-dom, zustand, tailwindcss (frontend).

## 5. Criterios de Aceptación

### 5.1 Funcionales

- [x] CA-001: Dado un sistema sin usuarios, al hacer POST `/api/setup` con email+password válidos, se crea el usuario y se establece la sesión.
- [x] CA-002: Dado un sistema con usuario configurado, GET `/api/setup-status` responde `setup_completed: true` y POST `/api/setup` falla con `already_setup`.
- [x] CA-003: Dadas credenciales correctas, POST `/api/login` devuelve 200 y `/api/me` responde el email autenticado.
- [x] CA-004: Dadas credenciales incorrectas, POST `/api/login` responde 401 con `invalid_credentials`.
- [x] CA-005: Dado un POST `/api/logout`, la sesión queda invalidada.
- [x] CA-006: La SPA se sirve en `/` y los diccionarios i18n en `/i18n/{es,en}.json`.
- [x] CA-007: El dashboard muestra un placeholder vacío y settings una pantalla vacía, ambos con título.
- [x] CA-008: El sidebar navega entre Dashboard y Settings.

### 5.2 No funcionales

- [x] CA-NF-001: `go build ./...` compila sin errores.
- [x] CA-NF-002: `npm run build` typecheckea y genera el bundle en `../public`.
- [x] CA-NF-003: `/health` responde `{"status":"ok"}` (usado por el HEALTHCHECK del contenedor).
- [x] CA-NF-004: El puerto por defecto es 8089 en config, Dockerfile, docker-compose y proxy de Vite.

### 5.3 Testing

- **Unit tests**: Pendientes (auth service). El proyecto original tiene tests; se replicarán junto con los módulos futuros.
- **Integration tests**: Verificado manualmente el flujo completo setup → login → me → logout (curl).
- **E2E tests**: No aplica todavía (sin módulos).
- **Carga/Performance**: Sin requerimientos específicos en el cascote.

## 6. Plan de Implementación

### 6.1 Fases

| Fase | Descripción | Estimación | Dependencias |
|------|-------------|------------|--------------|
| 1 | Replicar estructura del proyecto original recortada | 1 día | Ninguna |
| 2 | Backend: auth + settings + migración 0001 | 0.5 día | Fase 1 |
| 3 | Frontend: layout + dashboard + settings + i18n | 0.5 día | Fase 2 |
| 4 | Build/CI: Dockerfile, docker-compose, workflow, scripts | 0.5 día | Fase 3 |

### 6.2 Milestones

- **MVP**: Cascote verificado (`go build` + `npm run build` + flujo API ok).

## 7. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| Choque de puerto con `p40la-ihost` (8088) | Media | Alto | Puerto 8089 en todos los puntos de configuración |
| Dependencias Go requieren Go >= 1.24 | Media | Medio | Dockerfile usa `golang:1.24`; `GOTOOLCHAIN=auto` para dev local |
| Desincronización de i18n servido vs bundle | Baja | Medio | `scripts/sync-i18n.cjs` copia `public/i18n` → `src/i18n` en cada build |

## 8. Notas y Referencias

- Proyecto hermano de referencia: `https://github.com/paulomcnally/p40la-ihost`
- Repositorio de este proyecto: `https://github.com/paulomcnally/p40la-ihost-automation`
- Imagen Docker: `paulomcnally/p40la-ihost-automation` (Docker Hub)

## 9. Historial de Cambios

| Fecha | Autor | Descripción |
|-------|-------|-------------|
| 2026-09-11 | paulomcnally | Creación inicial: spec base que documenta el cascote (auth, dashboard placeholder, settings vacío, infra y build). Código ya existente en `main` (commits `2e53fb3`, `70fd2b0`). |