---
title: "Menú y CRUD de Apps con cuentas y credenciales"
id: "SPEC-002"
status: "released"
author: "paulomcnally"
created: "2026-09-11"
updated: "2026-09-11"
github_issue: 2
---

# Menú y CRUD de Apps con cuentas y credenciales

**ID**: SPEC-002  
**Estado**: released  
**Autor**: paulomcnally  
**Creado**: 2026-09-11  
**Actualizado**: 2026-09-11

---

## 1. Resumen Ejecutivo

Se necesita registrar **Apps** (proveedores/sitios web con login, ej. "Claro Nicaragua")
y para cada app una lista de **cuentas** (los servicios que el usuario tiene contratados
con ese proveedor, ej. 2 servicios de Claro = 2 cuentas). Cada cuenta debe poder
almacenar **credenciales** de acceso (usuario/contraseña, etc.) y un **identificador
único** que permita distinguir una cuenta de otra aunque compartan las mismas credenciales.

El caso de uso que motiva esta feature: un script futuro se conectará al sitio del
proveedor usando las credenciales de la cuenta e identificará cuál cuenta es (vía el
identificador) para descargar datos de facturación. Esta spec cubre **solo la UI y el
backend para registrar y administrar las cuentas**; el scraping/facturación queda fuera
de alcance para una spec futura.

El resultado esperado es un módulo "Apps" en la sidebar con un CRUD completo: crear,
editar y borrar apps; y dentro de cada app crear, editar y borrar sus cuentas. En línea
con las restricciones de iHost, el modelo es simple (2 tablas SQLite), sin dependencias
nuevas, y las credenciales se guardan como JSON válido en un único campo TEXT.

## 2. Requerimientos

### 2.1 Requerimientos Funcionales (P0 - Obligatorios)

1. **REQ-001**: Nuevo menú "Apps" en la sidebar con página de listado de apps (nombre + cantidad de cuentas + acciones editar/borrar + botón "Nueva app").
2. **REQ-002**: CRUD de apps: crear (solo nombre, único), editar nombre y borrar. Al borrar una app se borran sus cuentas en cascada.
3. **REQ-003**: Página de detalle de una app: lista sus cuentas con identificador y credenciales (enmascaradas), con acciones editar/borrar y botón "Nueva cuenta".
4. **REQ-004**: CRUD de cuentas dentro de una app: cada cuenta tiene **identificador** (obligatorio, único dentro de la app) y **credenciales** (obligatorias, formato JSON válido).
5. **REQ-005**: Validación de credenciales como JSON: el backend rechaza (400) cualquier cadena que no sea un objeto JSON parseable.
6. **REQ-006**: Persistencia en SQLite vía migración numerada `0002` (tablas `apps` y `accounts`), siguiendo el patrón migración → modelo → storage → servicio → handlers → frontend.

### 2.2 Requerimientos Funcionales (P1 - Importantes)

1. **REQ-007**: Campo opcional "etiqueta/label" por cuenta (ej. "Internet Fibra", "TV") para que el usuario distinga visualmente sus servicios en la lista.
2. **REQ-008**: Navegación "volver a la lista" con flecha atrás en el header al entrar al detalle de una app (requiere añadir el mapa `BACK_ROUTES` a `DashboardLayout.tsx`, que hoy no existe en este repo).

### 2.3 Requerimientos Funcionales (P2 - Deseables)

1. **REQ-009**: Ocultar/mostrar credenciales enmascaradas en el frontend (toggle) y desenmascararlas solo al editar la cuenta.
2. **REQ-010**: Confirmación visual antes de borrar una app (la cuenta con sus cuentas) y antes de borrar una cuenta.

### 2.4 Requerimientos No Funcionales

- **Rendimiento**: Consultas simples sobre 2 tablas con índices sobre FK; sin carga adicional apreciable en iHost.
- **Seguridad**: Todas las rutas detrás de `authMiddleware` (igual que settings). Las credenciales se almacenan en claro en SQLite (ámbito local/self-hosted); se mitiga con permisos del archivo DB y se documenta el riesgo (ver §7).
- **Almacenamiento**: Tablas `apps` (~pocos registros) y `accounts` (una fila por servicio). Campo `credentials` como TEXT sin límite superior explícito más allá de JSON.
- **Disponibilidad**: Sin nuevos endpoints públicos; `/health` no cambia.
- **iHost**: **Cero dependencias nuevas** (ni Go ni npm). JSON validado con `encoding/json` del stdlib. Sin procesamiento pesado.

## 3. Investigación y Decisiones Técnicas

### 3.1 Contexto investigado

- Se revisó la arquitectura existente en este repo: patrón de capas (`api` → `services` → `storage` → `models`), migraciones versionadas en `internal/db/db.go`, registro de rutas en `internal/api/routes.go` y wiring en `cmd/server/main.go`.
- Frontend: React Router con layout anidado en `App.tsx`, sidebar en `Sidebar.tsx`, endpoints agrupados en `src/api/index.ts`, i18n con fuente de verdad en `frontend/public/i18n/*.json` (es/en) sincronizada por `npm run build`.
- El repo aún **no** implementa `BACK_ROUTES` en `DashboardLayout.tsx` (el template lo referencia). Se incorpora su creación como parte de esta spec.

### 3.2 Opciones evaluadas

| Opción | Pros | Contras | Decisión |
|--------|------|---------|----------|
| Credenciales en columnas fijas (`username`, `password`) | Esquema rígido y tipado | No cubre apps con OTP, API keys, tokens, campos custom | ❌ Rechazada |
| Credenciales como JSON en campo TEXT | Flexible por app, mínimo en SQLite, validable con stdlib | No se puede indexar/consultar por campo interno | ✅ Seleccionada |
| Identificador global único | Simple de garantizar | Dos proveedores distintos podrían usar el mismo nº de cuenta sin conflicto real | ❌ Rechazada |
| Identificador único por app (`UNIQUE(app_id, identifier)`) | Refleja el dominio (el nº de cuenta solo debe ser único dentro de su proveedor) | Requiere índice compuesto | ✅ Seleccionada |

### 3.3 Decisiones arquitectónicas (ADRs)

**ADR-001**: Credenciales como JSON libre en campo TEXT
- **Contexto**: Cada sitio/proveedor exige campos distintos (usuario+password, API key, token, OTP…). iHost impone dependencias mínimas.
- **Decisión**: Columna `credentials TEXT` que almacena un objeto JSON válido. El servicio valida con `json.Unmarshal` antes de persistir.
- **Consecuencias**: Flexibilidad total por app; el script de scraping leerá el JSON y usará las claves que necesite. No hay consultas SQL sobre campos internos de las credenciales (no se necesitan).

**ADR-002**: Identificador único por app (índice compuesto `UNIQUE(app_id, identifier)`)
- **Contexto**: El identificador es lo que permite al script distinguir cuál cuenta es (ej. nº de servicio) aunque compartan credenciales.
- **Decisión**: `identifier` obligatorio y único dentro de su app (`UNIQUE(app_id, identifier)`).
- **Consecuencias**: Se permite el mismo identificador en apps distintas (correcto por dominio); el backend responde `409 conflict` si se intenta duplicar dentro de la misma app.

**ADR-003**: Añadir `BACK_ROUTES` al layout para navegación atrás
- **Contexto**: El template obliga a que "volver a la lista" vaya en el header, no como link en el contenido.
- **Decisión**: Introducir el mapa `BACK_ROUTES` en `DashboardLayout.tsx` (`{ "/apps/:appId": "/apps" }`) y renderizar la flecha atrás que reemplaza la hamburguesa en móvil cuando la ruta está registrada.
- **Consecuencias**: Cambio acotado en el layout, sin tocar el resto de páginas existentes.

## 4. Diseño Técnico

### 4.1 Diagrama de arquitectura

```
[React SPA] --(GET/POST/PUT/DELETE /api/apps*)--> [Go API (authMiddleware)]
        |                                                    |
        v                                                    v
[ /apps (lista) /apps/:appId (detalle) ]          [ SQLite: apps + accounts ]
```

### 4.2 Componentes

#### 4.2.1 Backend Go
- **Responsabilidad**: CRUD de apps y cuentas con validación de JSON en credenciales.
- **Interfaz**: Endpoints bajo `/api/apps` (ver §4.4), todos autenticados.
- **Dependencias**: Solo stdlib (`encoding/json`, `database/sql`).
- **Ubicación**: `internal/models/app.go`, `internal/storage/apps.go`, `internal/services/apps.go`, `internal/api/apps_handlers.go`, registro en `routes.go`, wiring en `cmd/server/main.go`.

#### 4.2.2 Frontend React
- **Responsabilidad**: Página de listado de apps y página de detalle con CRUD de cuentas.
- **Interfaz**: `AppsPage` (`/apps`) y `AppDetailPage` (`/apps/:appId`), endpoints en `src/api/index.ts`, entradas i18n.
- **Dependencias**: Sin nuevas dependencias (react, zustand, tailwind ya presentes).
- **Ubicación**: `frontend/src/pages/AppsPage.tsx`, `frontend/src/pages/AppDetailPage.tsx`, ruta en `App.tsx`, item en `Sidebar.tsx`, `api/apps` en `src/api/index.ts`.

### 4.3 Modelo de datos

```
apps
- id: INTEGER PK AUTOINCREMENT
- name: TEXT NOT NULL UNIQUE (ej. "Claro Nicaragua")
- created_at / updated_at: DATETIME NOT NULL
- Relaciones: accounts (1:N)

accounts
- id: INTEGER PK AUTOINCREMENT
- app_id: INTEGER NOT NULL REFERENCES apps(id) ON DELETE CASCADE
- identifier: TEXT NOT NULL (ej. nº de servicio; único dentro de la app)
- label: TEXT NULL (etiqueta opcional, ej. "Internet Fibra")
- credentials: TEXT NOT NULL (objeto JSON válido, ej. {"username":"...","password":"..."})
- created_at / updated_at: DATETIME NOT NULL
- UNIQUE(app_id, identifier)
- Índice en app_id
```

Migración: `migrations/0002_create_apps_and_accounts.{up,down}.sql`.

### 4.4 APIs / Contratos

#### Endpoint: `GET /api/apps`

**Response 200**:
```json
[
  { "id": 1, "name": "Claro Nicaragua", "account_count": 2 }
]
```

#### Endpoint: `POST /api/apps`

**Request**:
```json
{ "name": "Claro Nicaragua" }
```

**Response 201**:
```json
{ "id": 1, "name": "Claro Nicaragua" }
```

#### Endpoint: `PUT /api/apps/{id}`

**Request**:
```json
{ "name": "Claro Nicaragua" }
```

**Response 200**:
```json
{ "id": 1, "name": "Claro Nicaragua" }
```

#### Endpoint: `DELETE /api/apps/{id}`

**Response 204** (borra las cuentas en cascada).

#### Endpoint: `GET /api/apps/{id}/accounts`

**Response 200**:
```json
[
  { "id": 1, "app_id": 1, "identifier": "A1B2C3", "label": "Internet Fibra" }
]
```

> Las credenciales NO se devuelven en el listado (solo se muestran enmascaradas).

#### Endpoint: `POST /api/apps/{id}/accounts`

**Request**:
```json
{
  "identifier": "A1B2C3",
  "label": "Internet Fibra",
  "credentials": { "username": "user", "password": "secret" }
}
```

**Response 201**:
```json
{ "id": 1, "app_id": 1, "identifier": "A1B2C3", "label": "Internet Fibra" }
```

#### Endpoint: `PUT /api/apps/{id}/accounts/{accountId}`

Mismo contrato que POST (nombre/identificador/credenciales completas).

**Response 200**:
```json
{ "id": 1, "app_id": 1, "identifier": "A1B2C3", "label": "Internet Fibra" }
```

#### Endpoint: `DELETE /api/apps/{id}/accounts/{accountId}`

**Response 204**.

**Response Error** (todos los endpoints):
```json
{ "error": "codigo", "message": "descripción" }
```
Códigos: `invalid_request` (400, incluye credenciales no-JSON), `not_found` (404), `conflict` (409, nombre de app o identificador duplicado), `unauthorized` (401).

### 4.5 Dependencias

- **Internas**: `internal/api` (authMiddleware, respondJSON/respondError), `internal/storage` (patrón de accesso SQL), `internal/services`.
- **Externas**: **Ninguna nueva**. Se reutilizan `modernc.org/sqlite` (backend) y react/react-router-dom/zustand/tailwind (frontend).

## 5. Criterios de Aceptación

### 5.1 Funcionales

- [x] CA-001: Dado un usuario autenticado, al navegar a `/apps` se listan las apps con su cantidad de cuentas; si no hay ninguna, se muestra estado vacío.
- [x] CA-002: Dado un POST `/api/apps` con nombre nuevo, se crea la app (201); con nombre duplicado responde `conflict` (409).
- [x] CA-003: Dado un PUT `/api/apps/{id}`, se actualiza el nombre; un DELETE `/api/apps/{id}` borra la app y sus cuentas (204).
- [x] CA-004: Dado un GET `/api/apps/{id}/accounts`, se listan las cuentas **sin** las credenciales.
- [x] CA-005: Dado un POST/PUT de cuenta con `credentials` que NO es JSON válido, responde `invalid_request` (400) y no persiste.
- [x] CA-006: Dado un POST de cuenta con `identifier` duplicado dentro de la misma app, responde `conflict` (409); el mismo identificador en otra app es permitido.
- [x] CA-007: La UI permite crear/editar/borrar apps y crear/editar/borrar cuentas dentro de una app, con las credenciales editables como JSON (y enmascaradas en la lista).
- [x] CA-DARK: Los inputs/selects/textarea del formulario usan tokens del tema (`bg-card`, `text-text`, `text-text-secondary`) y se verificó legibilidad en darkmode (texto y placeholders).
- [x] CA-BACK: En `/apps/:appId` la flecha atrás del header reemplaza la hamburguesa en móvil y vuelve a `/apps` (vía `BACK_ROUTES`); NO existen links "← Título" dentro del contenido.

### 5.2 No funcionales

- [x] CA-NF-001: `go build ./...` compila sin errores y sin nuevas dependencias Go.
- [x] CA-NF-002: `npm run build` typecheckea y genera el bundle en `../public`; `scripts/sync-i18n.cjs` copia las claves nuevas de i18n.
- [x] CA-NF-003: Todas las rutas `/api/apps*` requieren sesión válida (`401` sin cookie).

### 5.3 Testing

- **Unit tests**: Pendientes (validación JSON, unicidad, conteo de cuentas). Flujo verificado manualmente vía curl.
- **Integration tests**: Verificado con curl: setup → crear app → 2 cuentas con mismas credenciales y distinto identificador → listar → editar → borrar (caso Claro Nicaragua) → 409 duplicados → 400 JSON inválido → 401 sin sesión → cascada al borrar app.
- **E2E tests**: Verificar manualmente en navegador (crear/editar/borrar apps y cuentas, error JSON, error duplicado).
- **Carga/Performance**: Sin requerimientos especiales; listados ligeros sobre SQLite.

## 6. Plan de Implementación

### 6.1 Fases

| Fase | Descripción | Estimación | Dependencias |
|------|-------------|------------|--------------|
| 1 | Migración `0002` + modelo `App`/`Account` + storage | 0.5 día | Ninguna |
| 2 | Servicio (validación JSON + unicidad) + handlers + rutas + wiring en `main.go` | 0.5 día | Fase 1 |
| 3 | Frontend: endpoints API, `AppsPage`, `AppDetailPage`, rutas, sidebar, i18n es/en | 1 día | Fase 2 |
| 4 | `BACK_ROUTES` en `DashboardLayout` + darkmode + verificación (`go build`, `npm run build`) | 0.5 día | Fase 3 |

### 6.2 Milestones

- **MVP**: CRUD de apps y cuentas funcional de punta a punta (backend + UI) con validaciones.
- **V1.0**: `BACK_ROUTES`, enmascarado de credenciales y confirmaciones de borrado (P1/P2).

## 7. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| Credenciales en claro en SQLite (exposición si roban el archivo DB) | Media | Alto | Acceso solo autenticado + permisos del archivo (`0700` en data dir). Futuro: cifrado opcional en una spec posterior. |
| JSON de credenciales con claves desconocidas para el script de scraping | Media | Medio | El JSON es libre y el script definirá qué claves lee; validar solo sintaxis en backend. |
| Identificadores duplicados por error de tipeo | Media | Bajo | `UNIQUE(app_id, identifier)` con `409` y mensaje claro en la UI. |
| Frontend pierde el patrón visual del resto de páginas | Baja | Medio | Reutilizar tokens del tema y `min-h-[44px]`; verificación en darkmode (CA-DARK). |

## 8. Notas y Referencias

- Proyecto de referencia (patrones de UI/backend): `https://github.com/paulomcnally/p40la-ihost`
- Patrón de agregar módulos: `AGENTS.md` (migración → modelo → storage → servicio → handlers → frontend).
- Repo actual: `https://github.com/paulomcnally/p40la-ihost-automation`

## 9. Historial de Cambios

| Fecha | Autor | Descripción |
|-------|-------|-------------|
| 2026-09-11 | paulomcnally | Creación inicial de la spec: menú y CRUD de Apps con cuentas (identificador único por app) y credenciales en JSON. |
| 2026-09-11 | paulomcnally | Estado a `in_progress`: inicio del desarrollo (migración 0002, backend, frontend). |
| 2026-09-11 | paulomcnally | Estado a `pending_release`: implementación completa y verificada (`go build` + `npm run build` + flujo curl del caso Claro Nicaragua: 2 cuentas, mismas credenciales, distinto identificador; 409 duplicados; 400 JSON inválido; 401 sin sesión; cascada). |
| 2026-09-11 | paulomcnally | Estado a `released`: merge a `main` en commit `0c6fc6e` (SPEC-002 implementada). Verificación local en `:8089`/`:5173` y seed `scripts/seed-apps.sh`. |