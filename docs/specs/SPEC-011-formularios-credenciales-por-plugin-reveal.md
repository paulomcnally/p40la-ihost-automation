---
title: "Formularios de credenciales por plugin + reveal con password"
id: "SPEC-011"
status: "released"
author: "paulomcnally"
created: "2026-09-12"
updated: "2026-09-12"
github_issue: 12
---

# Formularios de credenciales por plugin + reveal con password

**ID**: SPEC-011  
**Estado**: released  
**Autor**: paulomcnally  
**Creado**: 2026-09-12  
**Actualizado**: 2026-09-12

---

## 1. Resumen Ejecutivo

Hoy las credenciales de una cuenta se guardan como un **JSON libre** en `accounts.credentials`
(SPEC-002). En la UI se editan con un `textarea` donde el usuario pega el JSON a mano
(`AppDetailPage.tsx`, `AccountModal`). El problema: si el sistema se resetea (o se pierde la
base), es imposible recordar qué claves espera cada plugin (`{"username":"...","password":"..."}`
para Claro, `{"refresh_token":"...","client_id":"...","custom_uuid":"..."}` para Tigo,
`{"pin":"...","nis":"..."}` para DISNORTE). El usuario pide que **cada plugin genere su propio
formulario** con los inputs correspondientes para crear/editar ese JSON de credenciales.

Además, la información es **privada**: hoy `GET /api/apps/{appId}/accounts/{accountId}` devuelve
las credenciales en claro y el modal las muestra en el textarea al editar. El usuario quiere que
las credenciales aparezcan **siempre ocultas** y que solo se revelen tras una **aprobación de UI
usando el password del usuario** (verificación de contraseña en backend). Esto aplica a la lista
de cuentas (hoy solo muestra puntos `••••`), al formulario de edición y a cualquier endpoint que
exponga credenciales.

La solución tiene dos partes: (a) **backend** — cada plugin declara su *schema* de credenciales
(campos: clave, etiqueta, tipo text/password, requerido, secreto) vía un nuevo método de la
interfaz `Plugin`; un endpoint expone el schema por plugin; y un endpoint de **reveal** valida el
password del usuario y devuelve las credenciales desenmascaradas; (b) **frontend** — el modal de
cuenta renderiza el formulario dinámico según el schema del plugin asociado (en vez del textarea
JSON) y las credenciales se muestran ocultas con un botón "revelar" que pide el password.

Sin deps nuevas y sin migraciones: el schema es código Go (metadatos por plugin) y el reveal
reutiliza la verificación bcrypt existente de `AuthService`. Se mantiene la compatibilidad
hacia atrás: el JSON libre sigue siendo aceptado por la API (el schema solo guía la UI).

## 2. Requerimientos

### 2.1 Requerimientos Funcionales (P0 - Obligatorios)

1. **REQ-001**: La interfaz `Plugin` (`internal/plugins/types.go`) gana un método
   `CredentialSchema() []CredentialField` que declara los campos del JSON de credenciales:
   `Key` (clave en el JSON), `Label` (etiqueta UI), `Type` (`text` | `password`),
   `Required` (bool) y `Secret` (bool — si es secreto, se oculta y se valida en reveal).
   Los tres plugins implementan su schema real:
   - `claro.nicaragua`: `username` (text, requerido, no secreto), `password` (password, requerido, secreto).
   - `tigo.nicaragua`: `refresh_token` (password, requerido, secreto), `client_id` (text, opcional),
     `custom_uuid` (text, opcional).
   - `disnorte.dissur.nicaragua`: `pin` (password, requerido, secreto), `nis` (text, opcional).
2. **REQ-002**: Nuevo endpoint `GET /api/plugins/{name}/schema` (o `GET /api/plugins/{name}`
   ampliado) que devuelve el schema del plugin. El frontend lo usa para renderizar el formulario.
   Sin cambios de esquema: el schema se calcula del registro en memoria.
3. **REQ-003**: El modal de cuenta (`AppDetailPage.tsx`, `AccountModal`) deja de mostrar el
   `textarea` de JSON libre y renderiza **inputs por campo** según el schema del plugin asociado
   a la cuenta. Al crear/editar, el frontend arma el objeto JSON de credenciales a partir de los
   campos y lo envía a la API existente (`POST/PUT /api/apps/{id}/accounts`).
4. **REQ-004**: Las credenciales están **siempre ocultas** en la UI:
   - Lista de cuentas: se muestra `••••••••` (ya ocurre) — sin cambios.
   - Modal de edición: los inputs de campos `Secret` se muestran vacíos/enmascarados, NO se
     precargan con el valor en claro.
   - `GET /api/apps/{appId}/accounts/{accountId}` NO devuelve las credenciales en claro: devuelve
     un booleano `has_credentials` (o similar) y/o el JSON con los valores enmascarados.
5. **REQ-005**: Nuevo endpoint de reveal con aprobación de password:
   `POST /api/accounts/{accountId}/credentials:reveal` con body `{"password":"..."}`. El backend
   valida el password del usuario autenticado (reutilizando la verificación bcrypt de
   `AuthService`, sin crear sesión nueva) y, si es correcto, devuelve las credenciales en claro
   (solo al usuario aprobado). Si falla: `401 invalid_credentials` (o `403`), sin exponer nada.
6. **REQ-006**: El formulario del modal permite **revelar** las credenciales de un campo secreto
   con un botón que abre un diálogo pidiendo el password; al aprobar, se llaman al endpoint de
   reveal y se completan los inputs con los valores reales. Si el password es incorrecto, se
   muestra el error y no se revela nada.

### 2.2 Requerimientos Funcionales (P1 - Importantes)

1. **REQ-007**: Compatibilidad hacia atrás: si una cuenta no tiene plugin asociado o el plugin no
   define schema, el modal muestra el `textarea` JSON libre actual (fallback). La API sigue
   aceptando el JSON crudo (no se rompe el flujo actual de Claro por si el schema difiere).
2. **REQ-008**: Tests unitarios:
   - Schema de cada plugin (claves, tipos, requeridos, secretos correctos).
   - Endpoint de reveal: password correcto → credenciales en claro; password incorrecto → error;
     sin sesión → 401.
   - `GET` de cuenta sin credenciales en claro (solo `has_credentials`).
3. **REQ-009**: i18n: nuevas traducciones `apps.*` (es + en) para el formulario dinámico
   (labels por defecto si el plugin no define, botón "Revelar", título del diálogo de password,
   errores de password incorrecto).

### 2.3 Requerimientos Funcionales (P2 - Deseables)

1. **REQ-010**: Generar el JSON de credenciales de ejemplo (para debug/seed) desde el schema del
   plugin (endpoint `GET /api/plugins/{name}/schema` con `example` por campo), útil para
   `seed-apps.sh` y para la documentación.
2. **REQ-011**: Enmascarar selectivamente por tipo `Secret` en el historial/auditoría de webhook
   (no guardar el valor en claro en logs; hoy `webhook_logs.payload` no contiene credenciales,
   así que es un refuerzo de seguridad).

### 2.4 Requerimientos No Funcionales

- **Rendimiento**: Sin impacto (schema estático en memoria, verificación bcrypt puntual en reveal).
- **Seguridad**: Las credenciales NUNCA se exponen en claro salvo reveal aprobado con password.
  El password del usuario no viaja por query ni logs. Endpoints bajo `authMiddleware`.
- **Almacenamiento**: Cero migraciones; `accounts.credentials` sigue siendo JSON libre.
- **Disponibilidad**: El reveal falla con 401/403 sin exponer datos; el resto de la app no cambia.
- **iHost**: Cero dependencias nuevas (solo stdlib + bcrypt ya existente). Schema = metadatos
  estáticos Go.

## 3. Investigación y Decisiones Técnicas

### 3.1 Contexto investigado

- **`internal/plugins/types.go`**: interfaz `Plugin` con `Name`, `Version`, `Source`,
  `Description`, `FetchBills`. No hay noción de schema de credenciales.
- **Plugins actuales** y sus credenciales reales:
  - `claro.nicaragua` (SPEC-003): `creds["username"]`, `creds["password"]` (ver
    `nicaragua.go`, seed `scripts/seed-apps.sh:35`).
  - `tigo.nicaragua` (SPEC-007): `creds["refresh_token"]` (obligatorio), `creds["client_id"]`
    (opcional, default `auth0ClientID`), `creds["custom_uuid"]` (opcional) — `nicaragua.go:100-107`.
  - `disnorte.dissur.nicaragua` (SPEC-009): `creds["pin"]` (obligatorio), `creds["nis"]`
    (opcional, default identifier) — `nicaragua.go`.
- **`internal/api/apps_handlers.go:210-229`**: `GetAccount` devuelve `Credentials` en claro
  (campo `json:"credentials"`). El frontend lo usa para precargar el textarea (`AppDetailPage.tsx:30`).
- **`frontend/src/pages/AppDetailPage.tsx`**: `AccountModal` con `textarea` de JSON
  (`credentials`), parseo con `JSON.parse` y envío a `api.apps.createAccount/updateAccount`.
  La lista muestra `maskedCredentials()` = `••••••••••••`.
- **`frontend/src/api/index.ts`**: `apps.account()` devuelve `AccountDetail` con `credentials:
  string`. No hay endpoint de schema ni de reveal.
- **`internal/services/auth.go`**: `AuthService.Login` valida email+password con bcrypt
  (`bcrypt.CompareHashAndPassword`). `ValidateSession` resuelve el usuario de la cookie. El
  reveal puede reutilizar `users.GetByEmail` + `bcrypt.CompareHashAndPassword` sin crear sesión.
- **Rutas** (`internal/api/routes.go`): `GET /api/plugins`, `GET /api/accounts/{id}/plugin`,
  `PUT /api/accounts/{id}/plugin`, cuentas bajo `/api/apps/{appId}/accounts`.

### 3.2 Opciones evaluadas

| Opción | Pros | Contras | Decisión |
|--------|------|---------|----------|
| Schema declarado en cada plugin (método `CredentialSchema`) | La fuente de verdad es el plugin mismo; se actualiza junto al plugin | Método nuevo en la interfaz (rompe implementaciones → se actualizan las 3) | ✅ Seleccionada |
| Schema en JSON externo / tabla SQLite | Configurable sin recompilar | Migración nueva, fuente de verdad duplicada, riesgo de desincronizar | ❌ Rechazada |
| Modal con textarea libre + hint del schema | Mínimo esfuerzo | No resuelve el problema del usuario (seguiría escribiendo JSON a mano) | ❌ Rechazada |
| Reveal con re-login completo (crear sesión nueva) | Reutiliza `Login` tal cual | Crea/rota sesión sin necesidad; sobre-ingeniería para solo validar el password | ❌ Rechazada |
| Endpoint `credentials:reveal` que solo valida password (bcrypt) | Simple, sin tocar sesiones; reutiliza `UserStorage.GetByEmail` + bcrypt | El endpoint debe resolver el email del usuario autenticado vía sesión actual | ✅ Seleccionada |
| Quitar credenciales en claro de `GetAccount` y agregar `has_credentials` | Menos superficie de exposición | El modal necesita saber si hay credenciales guardadas para mostrar el botón revelar | ✅ Seleccionada |

### 3.3 Decisiones arquitectónicas (ADRs)

**ADR-001**: El schema de credenciales es código Go declarado en cada plugin
- **Contexto**: Cada plugin conoce sus claves; centralizarlo fuera generaría duplicación.
- **Decisión**: `CredentialSchema() []CredentialField` en la interfaz `Plugin`. `CredentialField`
  = `{Key, Label, Type, Required, Secret}`. Los 3 plugins lo implementan.
- **Consecuencias**: La UI se adapta a cada plugin sin tocar la API; el schema viaja con el plugin.

**ADR-002**: Las credenciales nunca se exponen en claro por defecto
- **Contexto**: Hoy `GetAccount` devuelve el JSON completo; el modal lo muestra al editar.
- **Decisión**: `GetAccount` devuelve `has_credentials: bool` (y opcionalmente el JSON con
  valores enmascarados). Los inputs `Secret` del modal se renderizan vacíos hasta el reveal.
- **Consecuencias**: La UI no muestra secretos sin aprobación; el reveal es explícito.

**ADR-003**: Reveal = endpoint dedicado que valida el password del usuario autenticado
- **Contexto**: El password del usuario es el gate de aprobación (el usuario quiere "aprobación
  de UI usando el password").
- **Decisión**: `POST /api/accounts/{accountId}/credentials:reveal` con `{"password":"..."}`.
  El handler resuelve el email desde la sesión actual (`authMiddleware`), busca el usuario y
  compara bcrypt. Solo entonces devuelve las credenciales en claro.
- **Consecuencias**: El password no rota sesiones; el reveal es auditable y puntual.

**ADR-004**: Cero migraciones y cero deps
- **Contexto**: Las credenciales ya viven en `accounts.credentials`; no hace falta cambiar el
  almacenamiento.
- **Decisión**: Solo backend (schema + reveal + `GetAccount` sin claro) y frontend (formulario
  dinámico + diálogo de password). 
- **Consecuencias**: Menos riesgo en iHost; la base sigue siendo JSON libre.

## 4. Diseño Técnico

### 4.1 Diagrama de arquitectura

```
[React SPA]
  AppDetailPage (AccountModal)
     |  GET /api/plugins/{name}/schema   →  inputs por campo (text/password, required, secret)
     |  POST /api/accounts/{id}/credentials:reveal  {password}  →  credenciales en claro
     v
[Go API (authMiddleware)]
     |  plugins.Registry.Get(name).CredentialSchema()
     |  AuthService: usuario de sesión + bcrypt.CompareHashAndPassword(password)
     v
[SQLite: accounts.credentials (JSON libre, sin cambios)]

[Plugin] claro.nicaragua / tigo.nicaragua / disnorte.dissur.nicaragua
     └─ CredentialSchema() → [{Key, Label, Type, Required, Secret}]
```

### 4.2 Componentes

#### 4.2.1 `internal/plugins/types.go`
- **Responsabilidad**: Definir el contrato de schema de credenciales.
- **Cambio**: Nuevo tipo `CredentialField{Key, Label string; Type string; Required, Secret bool}`
  y método `CredentialSchema() []CredentialField` en `Plugin`.

#### 4.2.2 Plugins (claro/tigo/disnorte)
- **Responsabilidad**: Declarar sus campos de credenciales.
- **Cambio**: Implementar `CredentialSchema()` con los campos reales (REQ-001).
- **Dependencias**: `internal/plugins` (tipo).

#### 4.2.3 `internal/api/plugins_handlers.go` / `bills_handlers.go`
- **Responsabilidad**: Exponer el schema y el reveal.
- **Cambio**:
  - `GET /api/plugins/{name}/schema` → schema del plugin (404 si no existe).
  - `POST /api/accounts/{accountId}/credentials:reveal` → valida password + devuelve credenciales.
- **Dependencias**: `AuthService` (validación de password), `AppsService` (credenciales).

#### 4.2.4 `internal/api/apps_handlers.go`
- **Responsabilidad**: No exponer credenciales en claro.
- **Cambio**: `GetAccount` devuelve `has_credentials: bool` (y opcionalmente valores
  enmascarados) en lugar del JSON crudo.

#### 4.2.5 Frontend (`AppDetailPage.tsx`, `AccountModal`, `api/index.ts`, i18n)
- **Responsabilidad**: Formulario dinámico por schema + reveal con password.
- **Cambio**: `AccountModal` renderiza inputs según schema; inputs `Secret` vacíos con botón
  "Revelar" que abre diálogo de password; fallback a textarea si no hay plugin/schema.
- **Dependencias**: Tailwind (estilos existentes), Zustand (i18n store).

### 4.3 Modelo de datos

```
Sin cambios de esquema.

accounts.credentials (JSON libre, sin cambios):
  - claro:   {"username": "...", "password": "..."}
  - tigo:    {"refresh_token": "...", "client_id": "...", "custom_uuid": "..."}
  - disnorte: {"pin": "...", "nis": "..."}

Nuevo tipo Go (no persistido): CredentialField
  - Key:      string   (clave en el JSON)
  - Label:    string   (etiqueta UI)
  - Type:     string   ("text" | "password")
  - Required: bool
  - Secret:   bool
```

### 4.4 APIs / Contratos

#### Endpoint: `GET /api/plugins/{name}/schema` (nuevo)

**Response 200**:
```json
[
  { "key": "username", "label": "Usuario", "type": "text", "required": true, "secret": false },
  { "key": "password", "label": "Contraseña", "type": "password", "required": true, "secret": true }
]
```

**Response Error**: `not_found` (404) si el plugin no existe.

#### Endpoint: `POST /api/accounts/{accountId}/credentials:reveal` (nuevo)

**Request**:
```json
{ "password": "..." }
```

**Response 200** (solo si el password es correcto):
```json
{ "credentials": { "username": "...", "password": "..." } }
```

**Response Error**: `invalid_credentials` (401) si el password es incorrecto; `not_found` (404)
si la cuenta no existe; `invalid_request` (400) si falta password.

#### Endpoint: `GET /api/apps/{appId}/accounts/{accountId}` (modificado)

**Response 200** (sin credenciales en claro):
```json
{
  "id": 7, "app_id": 1, "identifier": "<NUM_SERVICIO>", "label": null,
  "plugin_name": "claro.nicaragua", "has_credentials": true
}
```

> El JSON crudo de credenciales solo se devuelve vía `credentials:reveal`.

### 4.5 Dependencias

- **Internas**: `internal/plugins` (interfaz/schema), `internal/api` (handlers/routes),
  `internal/services` (`AuthService`, `AppsService`), `frontend` (AccountModal, api, i18n).
- **Externas**: Ninguna nueva (bcrypt ya existe en `golang.org/x/crypto`).

## 5. Criterios de Aceptación

### 5.1 Funcionales

- [ ] CA-001: Dado un `GET /api/plugins/claro.nicaragua/schema`, entonces devuelve los 2 campos
  con `username` (text, required) y `password` (password, secret, required).
- [ ] CA-002: Dado un `GET /api/plugins/tigo.nicaragua/schema`, entonces devuelve
  `refresh_token` (password, secret, required), `client_id` y `custom_uuid` (text, opcionales).
- [ ] CA-003: Dado un `GET /api/plugins/disnorte.dissur.nicaragua/schema`, entonces devuelve
  `pin` (password, secret, required) y `nis` (text, opcional).
- [ ] CA-004: Dado el modal de crear cuenta de una app con plugin, cuando se llena el formulario
  dinámico, entonces se envía el JSON de credenciales correcto a la API y la cuenta se crea.
- [ ] CA-005: Dado el modal de editar cuenta con credenciales guardadas, entonces los inputs
  `Secret` aparecen vacíos/enmascarados (sin precargar el valor) y hay un botón "Revelar".
- [ ] CA-006: Dado el diálogo de reveal con el password correcto del usuario, entonces se
  muestran las credenciales reales; con password incorrecto, se muestra error y nada se revela.
- [ ] CA-007: Dado un `GET /api/apps/{appId}/accounts/{accountId}` autenticado, entonces la
  respuesta NO contiene el JSON de credenciales en claro (solo `has_credentials`).
- [ ] CA-008: Dado un reveal sin sesión o con password incorrecto, entonces responde 401 sin
  exponer credenciales.
- [ ] CA-009: Dada una cuenta sin plugin asociado, entonces el modal muestra el textarea JSON
  libre (fallback) y el flujo actual sigue funcionando.
- [ ] CA-DARK: Los nuevos inputs/modal usan tokens del tema (`bg-card`, `text-text`,
  `border-border`) y se verificó legibilidad en darkmode.
- [ ] CA-BACK: El modal usa el layout existente; sin links "← Título" (nada nuevo que rompa).

### 5.2 No funcionales

- [ ] CA-NF-001: `go build ./...` compila sin errores.
- [ ] CA-NF-002: `go test ./internal/plugins/...` y `go test ./internal/api/...` pasan.
- [ ] CA-NF-003: `npm run build` (typecheck + build frontend) pasa.
- [ ] CA-NF-004: Sin dependencias nuevas en `go.mod` ni en `frontend/package.json`.

### 5.3 Testing

- **Unit tests**: `CredentialSchema` de los 3 plugins; reveal (password ok/ko/sin sesión);
  `GetAccount` sin credenciales en claro.
- **Integration tests**: Flujo UI: crear cuenta con plugin → formulario dinámico → JSON
  correcto; editar → reveal con password → valores reales; fallback sin plugin.
- **E2E tests**: Crear cuenta DISNORTE con `pin`/`nis` vía formulario, revelar con password y
  consultar facturas.
- **Carga/Performance**: Sin métricas nuevas; schema estático y bcrypt puntual.

## 6. Plan de Implementación

### 6.1 Fases

| Fase | Descripción | Estimación | Dependencias |
|------|-------------|------------|--------------|
| 1 | `CredentialField` + `CredentialSchema()` en interfaz y los 3 plugins + tests | 0.5 día | Ninguna |
| 2 | Endpoints: `GET /api/plugins/{name}/schema` + reveal (valida password) + `GetAccount` sin claro + tests | 0.75 día | Fase 1 |
| 3 | Frontend: `AccountModal` dinámico por schema, inputs secret ocultos, diálogo de password, fallback textarea, i18n es/en | 1 día | Fase 2 |
| 4 | `go build`/`go test`/`npm run build` + verificación e2e (crear/revelar cuenta real) | 0.5 día | Fase 3 |

### 6.2 Milestones

- **MVP**: Schema por plugin + `GetAccount` sin claro + reveal con password (backend).
- **V1.0**: Formulario dinámico en la UI + diálogo de reveal + fallback textarea + i18n.

## 7. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| Un plugin nuevo olvida implementar `CredentialSchema` | Media | Medio | La interfaz obliga el método al compilar; el fallback a textarea cubre el caso. |
| Reveal expone credenciales a un usuario con sesión válida | Media | Medio | El reveal requiere el password del usuario (aprobación explícita); el endpoint está bajo `authMiddleware` y valida bcrypt. |
| Romper el flujo de Claro (que hoy usa textarea) | Media | Alto | Fallback a textarea cuando no hay plugin/schema; tests de regresión del modal (CA-009). |
| El schema se desincroniza con las claves reales del plugin | Baja | Medio | El schema se define junto al plugin (misma compilación); los tests verifican las claves leídas en `FetchBills`. |
| Password del usuario en logs | Baja | Alto | Nunca loguear el body del reveal; mensajes genéricos de error. |

## 8. Notas y Referencias

- Archivos: `internal/plugins/types.go`, `internal/plugins/{claro,tigo,disnorte}/...`,
  `internal/api/{apps,plugins,bills}_handlers.go`, `internal/api/routes.go`,
  `internal/services/auth.go`, `frontend/src/pages/AppDetailPage.tsx`,
  `frontend/src/api/index.ts`, `frontend/public/i18n/{es,en}.json`.
- Relacionadas: SPEC-002 (credenciales en `accounts.credentials`), SPEC-003 (claro),
  SPEC-007 (tigo), SPEC-009 (disnorte).
- Precedente de reveal por password: `internal/services/auth.go` (`Login` + bcrypt).
- Repo: `https://github.com/paulomcnally/p40la-ihost-automation`.

## 9. Historial de Cambios

| Fecha | Autor | Descripción |
|-------|-------|-------------|
| 2026-09-12 | paulomcnally | Creación inicial de la especificación: formulario de credenciales generado por cada plugin (schema declarado en Go, endpoint `GET /api/plugins/{name}/schema`), credenciales siempre ocultas en la UI y reveal solo con el password del usuario (endpoint `credentials:reveal` con bcrypt). Fallback a textarea JSON cuando no hay plugin. Cero migraciones, cero deps nuevas. |
| 2026-09-12 | paulomcnally | Estado → `pending_execution` → `in_progress`. Inicio de desarrollo. |
| 2026-09-12 | paulomcnally | Implementación completa. `CredentialField` + `CredentialSchema()` en la interfaz `Plugin`; schema real en claro (username/password), tigo (refresh_token/client_id/custom_uuid) y disnorte (pin/nis). Endpoints: `GET /api/plugins/{name}/schema` (404 si no existe), `POST /api/accounts/{id}/credentials:reveal` (valida password con `AuthService.ValidatePassword` bcrypt; 401 si falla), `GetAccount` sin credenciales en claro (`has_credentials`). Frontend: `AccountModal` con formulario dinámico por schema (inputs text/password, requeridos/opcionales), campos secret ocultos con botón ojo + diálogo de reveal con password, fallback a textarea JSON sin plugin, iconos `eye`/`eye-off` nuevos, i18n es/en. Tests: schema de 3 plugins, `ValidatePassword`, e2e verificado (schema ok, reveal ok/ko, GetAccount sin claro, 401 sin sesión). `go build`/`go vet`/`go test`/`npm run build` OK. Estado → `pending_release`. |
| 2026-09-12 | paulomcnally | **Release** (cierre por decisión del usuario; el deploy a iHost lo realiza él). Implementación verificada end-to-end. Issue #12 cerrado con label `spec/released`. |