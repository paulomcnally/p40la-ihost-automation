---
title: "Plugin claro.nicaragua: consulta de facturas"
id: "SPEC-003"
status: "released"
author: "paulomcnally"
created: "2026-09-11"
updated: "2026-09-11"
github_issue: null
---

# Plugin claro.nicaragua: consulta de facturas

**ID**: SPEC-003  
**Estado**: in_progress  
**Autor**: paulomcnally  
**Creado**: 2026-09-11  
**Actualizado**: 2026-09-11

---

## 1. Resumen Ejecutivo

Se necesita un **plugin** de integración llamado `claro.nicaragua` que se autentique en
Mi Claro Nicaragua (`https://<DOMINIO_CLARO_WEB>/<portal_claro>/login`) usando las credenciales de
una cuenta registrada en el módulo Apps (SPEC-002), ubique el servicio por su
identificador (ej. `<NUM_SERVICIO>`) y obtenga los detalles de pago/factura. El objetivo es
consultar automáticamente las facturas pendientes de un servicio Claro sin abrir el
portal manualmente.

Los plugins forman parte de la estrategia de automatización del proyecto: cada
proveedor (Claro, otro operador, servicios web) es un plugin que conoce cómo
autenticarse y qué endpoints llamar. Este plugin es el primer caso real y define el
contrato de la infraestructura de plugins: una interfaz Go común, un registro, y sobre
todo **versionamiento obligatorio**. La fuente (portal de Claro) puede cambiar su lógica,
su dominio o su formato en cualquier momento; etiquetar cada resultado con la versión
del plugin y la fuente que lo generó permite auditar y actualizar sin romper datos
históricos.

El resultado esperado es: una infraestructura de plugins mínima (dependencias cero
nuevas, acorde a iHost), el plugin `claro.nicaragua` v1.0.0 capaz de autenticar y traer
facturas, una tabla SQLite `bills` que guarda cada consulta con su versión y fuente, y
endpoints/UI para listar plugins y ver las facturas de cada cuenta.

## 2. Requerimientos

### 2.1 Requerimientos Funcionales (P0 - Obligatorios)

1. **REQ-001**: Infraestructura de plugins en Go: interfaz común (`Name`, `Version`, `FetchBills`), registro por nombre y acceso desde el servicio de aplicaciones.
2. **REQ-002**: Plugin `claro.nicaragua` v1.0.0 que, dado el JSON de credenciales de una cuenta (`username`, `password`) y su identificador (nº de servicio), se autentica en Mi Claro Nicaragua, encuentra el servicio y devuelve sus facturas/pagos.
3. **REQ-003**: **Versionamiento obligatorio de plugins**: todo plugin declara una versión semver; cada consulta persistida queda etiquetada con la versión del plugin y la fuente (dominio/URL) que la generó. Un cambio de lógica, dominio o formato del proveedor implica un bump de versión del plugin (nunca sobreescribir sin versionar).
4. **REQ-004**: Asociación de plugins a cuentas: cada cuenta (SPEC-002) puede asociarse a un plugin mediante `plugin_name`. Al consultar facturas de una cuenta, se resuelve el plugin asociado; si la cuenta no tiene plugin asignado, la consulta responde `invalid_request`.
5. **REQ-005**: Persistencia en SQLite vía migración numerada `0003`: (a) tabla `plugins` (catálogo versionado de plugins), (b) ALTER de `accounts` añadiendo `plugin_name`, y (c) tabla `bills` (datos crudos de cada consulta con su `plugin_version`, `source` y estado ok/error).
6. **REQ-006**: Endpoints autenticados para (a) listar plugins con su versión, (b) consultar/forzar la actualización de facturas de una cuenta y listarlas, y (c) asociar/desasociar un plugin a una cuenta.

### 2.2 Requerimientos Funcionales (P1 - Importantes)

1. **REQ-007**: Manejo de errores estructurado: credenciales faltantes, login fallido, servicio no encontrado, respuesta con formato inesperado (API cambiada). Cada error se registra como `bills` con estado `error` y `plugin_version`.
2. **REQ-008**: Frontend: página de plugins (listado con versión y estado) y vista de facturas por cuenta con botón "Consultar facturas" y etiqueta de versión de plugin/fuente. Al editar una cuenta se permite seleccionar/asociar su plugin.
3. **REQ-009**: Gestión de sesión del plugin: el token JWT de Mi Claro se obtiene en cada consulta (login on-demand) o se cachea en memoria hasta su expiración; nunca se persiste.

### 2.3 Requerimientos Funcionales (P2 - Deseables)

1. **REQ-010**: Configuración de fuente por plugin almacenada en SQLite (dominio base, rutas) para que un cambio de dominio del proveedor sea una actualización de configuración/versión sin recompilar.
2. **REQ-011**: Detección y aviso de cambio de API: si la respuesta ya no coincide con el esquema esperado, el plugin lo reporta como error distintivo (`api_changed`) para que el usuario sepa que debe actualizar el plugin.

### 2.4 Requerimientos No Funcionales

- **Rendimiento**: Timeouts obligatorios en las llamadas HTTP salientes (ej. 15s). Consultas SQL simples con índice en `account_id`. Sin carga apreciable en iHost.
- **Seguridad**: Credenciales y tokens jamás se exponen en respuestas API. El JSON de credenciales se lee desde la cuenta (SPEC-002) solo al momento de la consulta. Tokens JWT en memoria, no persistidos.
- **Almacenamiento**: Tabla `bills` crece con cada consulta (una fila por servicio por consulta); `raw` es el payload original (JSON). Sin límite explícito; la UI lista las últimas N.
- **Disponibilidad**: Todas las rutas detrás de `authMiddleware`. `/health` no cambia.
- **iHost**: **Cero dependencias nuevas** (ni Go ni npm). HTTP con `net/http` stdlib; el envelope cifrado de Mi Claro se maneja con `crypto/aes`/`crypto/cipher` del stdlib. Nada de goroutines persistentes.

## 3. Investigación y Decisiones Técnicas

### 3.1 Contexto investigado

- El portal `https://<DOMINIO_CLARO_WEB>/<portal_claro>/login` es un SPA; el login produce un token JWT que se envía como header `authorization` junto a un token `screen` (ambos JWT firmados por la app).
- **Flujo de login real (desarmado del bundle del SPA)**: `POST /AccountManagement/QueryLogin` con body JSON plano `{email, password, token}` y headers `origin-route`, `content-type`, `x-forwarded-host: <DOMINIO_CLARO_WEB>`, `pais: ni` y `screen`. Responde `{sessionFrontId, data: {access_token, token_id, ...}}`. El `authorization` de las llamadas posteriores es `sessionFrontId`; la clave de cifrado es `data.token_id` (clave pública RSA SPKI en base64).
- **Cifrado del envelope**: cada petición viaja en `{"data":"<base64>"}` donde el plaintext descifrado es un JSON `{iv, key, payload}`. Esquema real: AES-256-GCM (iv de 12 bytes aleatorio) para el payload, con la clave AES (32 bytes) cifrada con **RSA-OAEP/SHA-256** usando la clave pública del `token_id`. El **payload se doble-codifica**: el plaintext del AES es `JSON.stringify(JSON.stringify(payload))`.
- **`screen` JWT**: HS256 firmado con el secreto `<SECRETO_DEL_BUNDLE>` (base64 en el bundle), payload `{screen:"data", iat, exp}` con TTL de 240s.
- Las **respuestas** del API son JSON plano (sin envelope).
- Endpoints (verificados en vivo con la cuenta real):
  - `POST /AccountManagement/RetrieveAssociatedAccountsv2` → cuentas asociadas. Payload: `{retrieveAssociatedAccounts:{LineOfBusiness:"1", UserProfileID: email}}`. Respuesta: `retrieveAssociatedAccountsResponse.AssociatedAccountListType[]` con `AccountID`, `LineOfBusiness`, `AssociationRoleType`.
  - `POST /BillingManagement/RetrieveBillHistoryList` → facturas. Payload: `{retrieveBillHistoryList:{LineOfBusiness, AccountID, UserProfileID, BillsQuantity:"5"}}`. Respuesta: `retrieveBillHistoryListResponse.BillsDetails[]` con `numFactura`, `balanceRestante`, `fechaEmision`, `fechaVencimiento`.
- El servicio objetivo `<NUM_SERVICIO>` aparece entre las cuentas asociadas (Linea Fija, `LineOfBusiness: 1`); su facturación se consulta luego con el historial de facturas. Verificado en vivo: devuelve la factura pendiente C$3544.60 (<NUM_FACTURA>).
- Se revisó SPEC-002 (in_progress): las cuentas guardan `identifier` (nº de servicio) y `credentials` como JSON libre (`{"username": "...", "password": "..."}`). El plugin lee de ahí.
- El repo **no** tenía aún infraestructura de plugins (grep `plugin` en `internal/` = sin resultados). Esta spec la introduce.

### 3.2 Opciones evaluadas

| Opción | Pros | Contras | Decisión |
|--------|------|---------|----------|
| Plugin como paquete Go compilado con interfaz común | Tipado, sin deps, versionable en código | Cambio de lógica = recompilar/release | ✅ Seleccionada |
| Plugin como script externo (Node/Python) | Actualización sin recompilar Go | Nuevas deps/runtime, más RAM en iHost | ❌ Rechazada |
| Versión del plugin implícita (sin registrar) | Simple | Imposible auditar qué versión generó cada dato | ❌ Rechazada |
| `Version()` semver + stamp en cada `bills` | Auditable, seguro ante cambios del proveedor | Requiere disciplina en el bump | ✅ Seleccionada |
| Envelope de Mi Claro descifrado con librería externa | Menos código propio | Dep nueva en iHost | ❌ Rechazada (se usa stdlib) |

### 3.3 Decisiones arquitectónicas (ADRs)

**ADR-001**: Plugins como paquetes Go compilados con interfaz común
- **Contexto**: iHost impone dependencias y memoria mínimas; los proveedores externos son impredecibles.
- **Decisión**: `internal/plugins` define la interfaz `Plugin` y un `Registry`; cada proveedor es un paquete Go que la implementa y se registra en `main.go`. El servicio de aplicaciones consulta el registro por nombre.
- **Consecuencias**: Cambios de lógica del proveedor requieren bump de versión del plugin y nuevo build/release; a cambio no hay runtime extra y el versionamiento es explícito.

**ADR-002**: Versionamiento obligatorio con stamp de versión y fuente en cada resultado
- **Contexto**: La fuente (Claro) puede cambiar lógica, dominio o formato; los datos históricos deben seguir siendo interpretables.
- **Decisión**: Todo plugin expone `Version()` (semver). La tabla `bills` guarda `plugin_version` y `source` (dominio + ruta) por fila. Un cambio del proveedor se refleja como bump de versión; nunca se reescribe el historial.
- **Consecuencias**: Auditoría completa de qué integración generó cada dato; el usuario ve en la UI la versión/fuente de cada consulta.

**ADR-003**: El envelope de Mi Claro se maneja con stdlib (`crypto/rsa`, `crypto/aes`, `crypto/cipher`)
- **Contexto**: El body del API de Claro viaja en `{"data": "<base64>"}` con esquema RSA-OAEP/SHA-256 + AES-256-GCM, y el payload se doble-codifica (`JSON.stringify(JSON.stringify(payload))`).
- **Decisión**: Implementar el cifrado con stdlib en el propio plugin, encapsulado para que un cambio de esquema solo toque el plugin (bump de versión). El screen JWT (HS256) se genera con `crypto/hmac` + el secreto del bundle.
- **Consecuencias**: Cero dependencias nuevas; el código del envelope queda versionado junto al plugin.

**ADR-004**: Los tokens JWT de Mi Claro viven solo en memoria
- **Contexto**: No se deben persistir credenciales ni sesiones más allá de lo necesario.
- **Decisión**: La sesión (JWT `authorization`/`screen`) se obtiene por consulta o se cachea en memoria respetando `exp`; nunca se escribe en SQLite.
- **Consecuencias**: Cada reinicio del server requiere un login nuevo; seguro ante exfiltración de la DB.

## 4. Diseño Técnico

### 4.1 Diagrama de arquitectura

```
[React SPA] --(GET /api/plugins, PUT /api/accounts/{id}/plugin, POST /api/accounts/{id}/bills:fetch)--> [Go API (authMiddleware)]
        |                                                                          |
        v                                                                          v
[PluginsPage, Bills (por cuenta)]                                        [services.BillsService]
                                                                                   |
                                                                                   v
                                                          [internal/plugins: Registry + claro.nicaragua]
                                                                                   |
                                                          HTTPS (login + <DOMINIO_CLARO_API>) --> [Claro]
                                                                                   |
                                                                                   v
                                                                  [SQLite: plugins + accounts(plugin_name) + bills]
```

### 4.2 Componentes

#### 4.2.1 Infraestructura de plugins (`internal/plugins`)
- **Responsabilidad**: Definir la interfaz común, el registro de plugins y el contrato de consulta de facturas.
- **Interfaz**:
  ```go
  type Plugin interface {
      Name() string
      Version() string                 // semver
      Source() string                  // dominio/ruta base de la fuente
      FetchBills(ctx context.Context, creds map[string]string, identifier string) ([]Bill, error)
  }

  type Bill struct {
      Period    string // ej. "2026-09"
      Amount    string
      DueDate   string
      Status    string
      Raw       map[string]any // payload original
  }
  ```
- **Dependencias**: Solo stdlib.
- **Ubicación**: `internal/plugins/registry.go`, `internal/plugins/types.go`, `internal/plugins/claro/nicaragua/nicaragua.go`.

#### 4.2.2 Backend Go (servicio + handlers)
- **Responsabilidad**: Orquestar consultas: leer credenciales de la cuenta (SPEC-002) y su `plugin_name`, resolver el plugin por nombre, invocarlo y persistir en `bills`. Mantener sincronizado el catálogo `plugins` con el registro compilado.
- **Interfaz**: `GET /api/plugins`, `GET /api/accounts/{accountId}/plugin`, `PUT /api/accounts/{accountId}/plugin`, `POST /api/accounts/{accountId}/bills:fetch`, `GET /api/accounts/{accountId}/bills`.
- **Dependencias**: stdlib + componentes existentes (`storage`, `services.AppsService`).
- **Ubicación**: `internal/services/bills.go`, `internal/api/bills_handlers.go`, `internal/api/plugins_handlers.go`, registro en `routes.go`, wiring en `cmd/server/main.go`.

#### 4.2.3 Frontend React
- **Responsabilidad**: Página de plugins (listado con versión/fuente) y vista de facturas por cuenta con acción "Consultar facturas". En el formulario de cuenta se permite seleccionar/asociar el plugin.
- **Interfaz**: `PluginsPage` (`/plugins`), vista de facturas en el detalle de cuenta o página `AccountBillsPage` (`/apps/:appId/accounts/:accountId/bills`), selector de plugin en el formulario de cuenta (SPEC-002).
- **Dependencias**: Sin nuevas (react, react-router, zustand, tailwind).
- **Ubicación**: `frontend/src/pages/PluginsPage.tsx`, `frontend/src/pages/AccountBillsPage.tsx`, rutas en `App.tsx`, item en `Sidebar.tsx`, endpoints en `src/api/index.ts`, i18n es/en.

### 4.3 Modelo de datos

```
apps / accounts  (SPEC-002, sin cambios de esquema inicial)
  - account.credentials  = {"username": "...", "password": "..."}
  - account.identifier   = nº de servicio (ej. "<NUM_SERVICIO>")
  - account.plugin_name  = plugin asociado (añadido por esta spec, ver abajo)

plugins  (nueva, migración 0003)
- id: INTEGER PK AUTOINCREMENT
- name: TEXT NOT NULL UNIQUE (ej. "claro.nicaragua")
- version: TEXT NOT NULL (semver actual del plugin, ej. "1.0.0")
- source: TEXT NOT NULL (dominio/ruta base de la fuente, ej. <DOMINIO_CLARO_API>)
- description: TEXT NULL
- status: TEXT NOT NULL DEFAULT 'active' ('active' | 'deprecated')
- created_at / updated_at: DATETIME NOT NULL

accounts  (ALTER, migración 0003)
- ADD COLUMN plugin_name TEXT NULL REFERENCES plugins(name)
  (nullable: una cuenta puede no tener plugin asociado todavía)

bills  (nueva, migración 0003)
- id: INTEGER PK AUTOINCREMENT
- account_id: INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE
- plugin_name: TEXT NOT NULL (plugin que generó la consulta)
- plugin_version: TEXT NOT NULL (semver del plugin que generó la consulta)
- source: TEXT NOT NULL (dominio/ruta, ej. <DOMINIO_CLARO_API>/BillingManagement/...)
- status: TEXT NOT NULL ('ok' | 'error')
- error: TEXT NULL (código + mensaje cuando status=error)
- raw: TEXT NOT NULL (payload JSON original de la consulta, o detalle del error)
- fetched_at: DATETIME NOT NULL
- created_at: DATETIME NOT NULL
- Índice en account_id
```

Migración: `migrations/0003_plugins_and_bills.{up,down}.sql`.

### 4.4 APIs / Contratos

#### Endpoint: `GET /api/plugins`

**Response 200**:
```json
[
  { "name": "claro.nicaragua", "version": "1.0.0", "source": "<DOMINIO_CLARO_API>", "description": "Consulta de facturas Claro Nicaragua" }
]
```

#### Endpoint: `GET /api/accounts/{accountId}/plugin`

**Response 200**:
```json
{ "plugin_name": "claro.nicaragua", "version": "1.0.0" }
```

**Response 200** (sin plugin asociado):
```json
{ "plugin_name": null }
```

#### Endpoint: `PUT /api/accounts/{accountId}/plugin`

**Request**:
```json
{ "plugin_name": "claro.nicaragua" }
```

**Response 200**:
```json
{ "plugin_name": "claro.nicaragua", "version": "1.0.0" }
```
> Enviar `{"plugin_name": null}` desasocia el plugin. `404` si el plugin no existe en el catálogo.

#### Endpoint: `POST /api/accounts/{accountId}/bills:fetch`

Ejecuta la consulta de facturas de la cuenta (lee credenciales + identificador de la
cuenta, resuelve el plugin **asociado a la cuenta** vía `plugin_name`) y persiste el
resultado en `bills`. Si la cuenta no tiene plugin asociado responde `invalid_request` (400).

**Response 201**:
```json
{ "id": 42, "account_id": 7, "plugin_name": "claro.nicaragua", "plugin_version": "1.0.0", "status": "ok", "bills": [{ "period": "2026-09", "amount": "1250.00", "due_date": "2026-09-25", "status": "pending" }] }
```

**Response Error** (códigos): `invalid_request` (400, sin plugin asociado), `not_found` (404, cuenta o plugin), `upstream_error` (502, el proveedor devolvió error), `api_changed` (502, esquema inesperado), `unauthorized` (401).

#### Endpoint: `GET /api/accounts/{accountId}/bills`

**Response 200**:
```json
[
  { "id": 42, "plugin_name": "claro.nicaragua", "plugin_version": "1.0.0", "source": "<DOMINIO_CLARO_API>", "status": "ok", "fetched_at": "2026-09-11T10:00:00Z", "bills": [] }
]
```
> Las credenciales y tokens nunca aparecen en las respuestas.

### 4.5 Dependencias

- **Internas**: `internal/storage` (acceso SQL), `internal/services` (`AppsService` para credenciales/identificador), `internal/api` (authMiddleware, respondJSON/respondError).
- **Externas**: **Ninguna nueva**. `net/http`, `crypto/aes`, `crypto/cipher`, `encoding/json`, `encoding/base64` del stdlib Go. Frontend: sin nuevas dependencias npm.

## 5. Criterios de Aceptación

### 5.1 Funcionales

- [ ] CA-001: Dado un GET `/api/plugins` autenticado, se listan los plugins registrados con su `version` y `source` (al menos `claro.nicaragua` 1.0.0).
- [ ] CA-002: Dado un PUT `/api/accounts/{id}/plugin` con `claro.nicaragua`, la cuenta queda asociada al plugin y el GET devuelve `{ "plugin_name": "claro.nicaragua", "version": "1.0.0" }`; con un plugin inexistente responde `404`.
- [ ] CA-003: Dado un POST `/api/accounts/{id}/bills:fetch` con credenciales válidas de Mi Claro, identificador del servicio y plugin asociado, el plugin se autentica, ubica el servicio y persiste la factura con `status: ok`.
- [ ] CA-004: Dado el servicio `<NUM_SERVICIO>`, la consulta devuelve los detalles de la factura/pago y quedan visibles en la UI con la versión del plugin y la fuente.
- [ ] CA-005: Toda fila en `bills` tiene `plugin_name`, `plugin_version` y `source` poblados (no null), independientemente de `status` (ok o error).
- [ ] CA-006: Dado un POST `/api/accounts/{id}/bills:fetch` en una cuenta **sin** plugin asociado, responde `invalid_request` (400) y no persiste nada.
- [ ] CA-007: Dado un login fallido, servicio no encontrado o esquema inesperado, la consulta persiste un `bills` con `status: error` y el código correspondiente (`auth_failed`, `service_not_found`, `api_changed`), sin exponer credenciales.
- [ ] CA-008: Las credenciales y tokens JWT NO aparecen en ninguna respuesta de API ni en el `raw` persistido.
- [ ] CA-009: La UI permite asociar el plugin al crear/editar una cuenta, consultar facturas por cuenta, y ver la lista de facturas con la versión/fuente del plugin que las generó.
- [ ] CA-DARK: Los inputs/selects/textarea del formulario de consulta usan tokens del tema (`bg-card`, `text-text`, `text-text-secondary`) y se verificó legibilidad en darkmode (texto y placeholders).
- [ ] CA-BACK: En la vista de facturas de una cuenta, la flecha atrás del header reemplaza la hamburguesa en móvil y vuelve al detalle de la app/cuenta (vía `BACK_ROUTES`); NO existen links "← Título" dentro del contenido.

### 5.2 No funcionales

- [ ] CA-NF-001: `go build ./...` compila sin errores y sin nuevas dependencias Go.
- [ ] CA-NF-002: `npm run build` typecheckea y genera el bundle en `../public`; `scripts/sync-i18n.cjs` copia las claves nuevas de i18n.
- [ ] CA-NF-003: Todas las rutas de plugins/bills requieren sesión válida (`401` sin cookie).
- [ ] CA-NF-004: Cada llamada saliente al proveedor tiene timeout (≤ 15s) y no bloquea otras peticiones del server.

### 5.3 Testing

- **Unit tests**: Registry (registro/duplicados), validación de la respuesta del envelope cifrado de Claro (descifrado), errores mapeados (`auth_failed`, `service_not_found`, `api_changed`).
- **Integration tests**: Flujo con un fixture del envelope: credenciales → accounts (`<NUM_SERVICIO>`) → bill history → `bills` persistido con `plugin_version` y `source`.
- **E2E tests**: Desde la UI, consultar facturas de una cuenta y ver el resultado con versión/fuente; forzar un error de credenciales y ver el registro con estado `error`.
- **Carga/Performance**: Una consulta por cuenta < 5s total en iHost; sin picos de RAM (sin librerías nuevas, sin pools).

## 6. Plan de Implementación

### 6.1 Fases

| Fase | Descripción | Estimación | Dependencias |
|------|-------------|------------|--------------|
| 1 | Investigación del login/envelope de Mi Claro (descifrar `data`, replicar headers y flujo de tokens) | 1 día | Ninguna |
| 2 | Infraestructura `internal/plugins` (interfaz + registry + tipos `Bill`) | 0.5 día | Fase 1 |
| 3 | Plugin `claro.nicaragua` v1.0.0 (login, accounts, bill history, parse) | 1 día | Fase 2 |
| 4 | Migración `0003` (plugins + ALTER accounts + bills) + storage/service de bills + handlers + rutas + wiring | 0.5 día | Fase 3 |
| 5 | Frontend: `PluginsPage`, facturas por cuenta, endpoints, rutas, sidebar, i18n, darkmode | 1 día | Fase 4 |
| 6 | Verificación (`go build`, `npm run build`), test manual con cuenta real, bump a `pending_release` | 0.5 día | Fase 5 |

### 6.2 Milestones

- **MVP**: Infraestructura de plugins + `claro.nicaragua` v1.0.0 consultando facturas + tabla `bills` versionada + endpoints/UI básicos.
- **V1.0**: Manejo de errores estructurado, cache de sesión en memoria, configuración de fuente por plugin (P1/P2).

## 7. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| Claro cambia la lógica/dominio/formato del portal | Media | Alto | Versionamiento obligatorio (REQ-003): bump de versión + stamp en `bills`; error `api_changed` avisa al usuario. |
| El envelope cifrado no se replica exactamente (algoritmo/claves) | Media | Alto | Fase 1 dedicada a descifrar y validar con llamadas reales (fines investigativos con credenciales propias) antes de fijar el contrato. |
| Credenciales/tokens expuestos por error | Baja | Alto | REQ-006/CA-006: nunca en respuestas ni en `raw`; sesión solo en memoria. |
| Login de Claro pide captcha/2FA (bloqueo) | Media | Medio | Documentar limitación; el plugin reporta `auth_failed` y la UI lo muestra claramente. |
| `bills` crece sin límite | Media | Bajo | La UI lista las últimas N; opcional limpieza por antigüedad en fase P2. |

## 8. Notas y Referencias

- Portal de login: `https://<DOMINIO_CLARO_WEB>/<portal_claro>/login`
- API: `https://<DOMINIO_CLARO_API>/AccountManagement/RetrieveAssociatedAccountsv2` y `https://<DOMINIO_CLARO_API>/BillingManagement/RetrieveBillHistoryList`
- Credenciales de prueba: proporcionadas por el usuario, uso exclusivamente investigativo, a sustituir por las de SPEC-002 (Apps/credentiales).
- Depende de SPEC-002 (Apps CRUD con cuentas y credenciales) para el origen de email/password/cuenta.
- Patrón de módulos: `AGENTS.md` (migración → modelo → storage → servicio → handlers → frontend).
- Repo: `https://github.com/paulomcnally/p40la-ihost-automation`

## 9. Historial de Cambios

| Fecha | Autor | Descripción |
|-------|-------|-------------|
| 2026-09-11 | paulomcnally | Creación inicial de la spec: plugin `claro.nicaragua` para consulta de facturas con infraestructura de plugins e **versionamiento obligatorio** (versión semver + fuente stamp en cada consulta). |
| 2026-09-11 | paulomcnally | Se añade catálogo `plugins` en SQLite y **asociación de plugin por cuenta** (`accounts.plugin_name`): cada cuenta elige el plugin que la consulta. |
| 2026-09-11 | paulomcnally | Estado a `in_progress`: inicio del desarrollo (migración 0003, infraestructura de plugins, plugin `claro.nicaragua`, backend y frontend). |
| 2026-09-11 | paulomcnally | Verificación en vivo contra la API real: se desarmó el bundle del SPA de Mi Claro, se replicó el cifrado real (QueryLogin → screen JWT HS256 → envelope RSA-OAEP+AES-GCM con payload doble-codificado) y las llamadas `RetrieveAssociatedAccountsv2`/`RetrieveBillHistoryList`. El plugin v1.2.0 obtiene las facturas del servicio <NUM_SERVICIO> (pendiente C$3544.60). |
| 2026-09-11 | paulomcnally | Cambio iterativo: el monto de las facturas pasa a usar `montoFactura` (monto real) en vez de `balanceRestante` (saldo pendiente, 0 en facturas pagadas). El estado pending/paid se deriva de `balanceRestante`. Bump a v1.3.0. |
| 2026-09-11 | paulomcnally | **Released**: implementación completa verificada en vivo (login QueryLogin, envelope RSA-OAEP+AES-GCM, facturas del servicio <NUM_SERVICIO>). Commit `0629c10`. |