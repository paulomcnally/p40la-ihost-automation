---
title: "Plugin tigo.nicaragua: lista de facturas vía id_token + refresh"
id: "SPEC-007"
status: "released"
author: "paulomcnally"
created: "2026-09-12"
updated: "2026-09-12"
github_issue: 8
---

# Plugin tigo.nicaragua: lista de facturas vía id_token + refresh

**ID**: SPEC-007  
**Estado**: released  
**Autor**: paulomcnally  
**Creado**: 2026-09-12  
**Actualizado**: 2026-09-12

---

## 1. Resumen Ejecutivo

Se necesita un plugin de integración llamado `tigo.nicaragua` que consulte la **lista de
facturas** de las líneas Tigo Nicaragua de un usuario usando la API de "Mi Cuenta Tigo"
(`<DOMINIO_TIGO_API>/api/v2.0`), la misma que consume la app móvil
**Mi Tigo** (iOS). El usuario capturó con Charles Proxy una sesión real de la app para
revertir el flujo de autenticación y los endpoints de facturación.

La captura reveló un dato crítico que invalidaba la implementación anterior: **la app
móvil NO envía el `access_token` como `Authorization: Bearer`; envía el `id_token`**.
Ambos vienen en la respuesta de `POST /oauth/token` (Auth0 de Tigo, `<DOMINIO_TIGO_AUTH0>`).
El `access_token` es un **JWE cifrado** (ilegible localmente, `alg: dir, enc: A256GCM`),
mientras que el `id_token` es un **JWT RS256 legible** que la API acepta. El intento
anterior usaba el `access_token` y por eso la API rechazaba las peticiones.

El flujo de autenticación es un **refresh token** de larga vida guardado en las
credenciales de la cuenta: `POST /oauth/token` con `grant_type=refresh_token` devuelve
`access_token` + `id_token` + `expires_in: 86400` (24 h). Con ese `id_token` se consultan
los endpoints reales capturados. El endpoint de facturas confirmado es
`GET /mobile/billing/subscribers/{msisdn}/invoices?_format=json`, que devuelve
`data.invoiceList[]` con `invoiceId`, `billingPeriod`, `invoiceAmount`, `dueAmount`,
`dueDate` y `hasPayment`.

Este plugin sigue el contrato de infraestructura de SPEC-003 (interfaz `Plugin`, registro,
tabla `bills`, asociación `accounts.plugin_name`). **No requiere cambios de esquema**.
Reutiliza las credenciales JSON libres de la cuenta para guardar `refresh_token`,
`client_id`, `custom_uuid` y el `id_token` opcional en caché. El resultado esperado:
plugin `tigo.nicaragua` registrado, `FetchBills` renueva el token automáticamente y
persiste la lista de facturas en `bills` vía los endpoints existentes.

## 2. Requerimientos

### 2.1 Requerimientos Funcionales (P0 - Obligatorios)

1. **REQ-001**: Plugin `tigo.nicaragua` que implementa la interfaz `Plugin` existente (`Name`, `Version`, `Source`, `Description`, `FetchBills`), se registra en `main.go` y aparece en el catálogo `plugins` vía `SyncCatalog` (sin migración nueva).
2. **REQ-002**: Autenticación por **refresh token**: las credenciales de la cuenta guardan `{"refresh_token": "<rt>", "client_id": "<cid>"}`. `FetchBills` llama a `POST https://<DOMINIO_TIGO_AUTH0>/oauth/token` con `grant_type=refresh_token` para obtener `access_token` + `id_token` y **usa el `id_token` como Bearer** para la API.
3. **REQ-003**: El `id_token` se decodifica (RS256 legible) para obtener el claim `custom:UUID` (ej. `<UUID>`) necesario para el endpoint de cuentas; si el token es JWE (no legible), se toma `custom_uuid` de las credenciales.
4. **REQ-004**: Flujo de consulta: (a) `GET /party/uuid/{customUUID}/accounts?_format=json` para obtener las cuentas (o se usa el `psId`/`msisdn` del `id_token` como subscriber directo), (b) `GET /mobile/billing/subscribers/{msisdn}/invoices?_format=json` para la lista de facturas, (c) normalizar `data.invoiceList[]` a `[]plugins.Bill`.
5. **REQ-005**: Headers obligatorios en todas las llamadas (replicando la app móvil, verificados en la captura): `Origin: ionic://localhost`, `client-version: 2.20.0`, UA de iPhone, `sec-fetch-*`, `Accept: application/json, text/plain, */*`, `Content-Type: application/json`, `Accept-Language: es-419,es;q=0.9`, `Priority: u=3, i`.
6. **REQ-006**: Los endpoints existentes funcionan sin cambios para Tigo: `GET /api/plugins`, `PUT /api/accounts/{id}/plugin`, `POST /api/accounts/{id}/bills:fetch`, `GET /api/accounts/{id}/bills`. Nada se envía a un webhook hasta que el usuario lo configure manualmente.
7. **REQ-007**: Seed local para pruebas: `scripts/seed-apps.sh` soporta variables para crear una app + cuenta Tigo de ejemplo (`PLUGIN_NAME=tigo.nicaragua`, `IDENTIFIERS=<MSISDN>`, `CREDENTIALS='{"refresh_token":"..."}'`).

### 2.2 Requerimientos Funcionales (P1 - Importantes)

1. **REQ-008**: Manejo de errores estructurado mapeado a los errores del dominio (`ErrAuthFailed`, `ErrUpstream`, `ErrAPISchemaChanged`, `ErrAuthExpired`), persistidos como `bills` con `status: error` y `plugin_version`, sin exponer el token.
2. **REQ-009**: Verificar el estado de la IP frente al WAF de Cloudflare (`<DOMINIO_TIGO_NET>` bloquea clientes no-navegador según reputación de IP). Si desde la red del iHost da 403, documentar el workaround (usar la IP del usuario o proxy) sin martillar la API.
3. **REQ-010**: Consulta de saldo (`GET /mobile/billing/subscribers/{msisdn}/balance?_format=json`) como complemento informativo (deuda, factura pendiente, fecha límite).

### 2.3 Requerimientos Funcionales (P2 - Deseables)

1. **REQ-011**: Validación de la firma RS256 del `id_token` contra las claves de Auth0 (JWKS) antes de usarlo.
2. **REQ-012**: Extracción automática del `refresh_token` vía el script `scripts/tigo-auth.sh` (flujo Auth0 PKCE, sin captcha) para que el usuario no lo pegue a mano.

### 2.4 Requerimientos No Funcionales

- **Rendimiento**: Timeouts obligatorios (≤ 15 s) en llamadas HTTP salientes. Una consulta < 5 s.
- **Seguridad**: El `refresh_token` y el `id_token` jamás se exponen en respuestas de API ni en `raw`; viven solo en el JSON de credenciales de la cuenta y en memoria durante la consulta.
- **Almacenamiento**: Reusa la tabla `bills`; cero migraciones nuevas.
- **Disponibilidad**: Todas las rutas detrás de `authMiddleware`; `/health` sin cambios.
- **iHost**: Cero dependencias nuevas (Go stdlib `net/http`). El JWT se decodifica con `encoding/base64` + `encoding/json`.

## 3. Investigación y Decisiones Técnicas

### 3.1 Contexto investigado (captura Charles Proxy real, 2026-09-12)

Se capturó una sesión real de la app móvil **Mi Tigo** (iOS) desde `192.168.1.197`
mediante Charles Proxy. La sesión contiene los tres dominios del usuario y endpoints
verificados:

- **`<DOMINIO_TIGO_AUTH0>`** (Auth0 de Tigo, `34.236.70.155`):
  - `POST /oauth/token` (HTTP/2) con body JSON:
    ```json
    {
      "client_id": "uLf5QUCgJ1vd2GjV5RsLCEFJoQweCpic",
      "grant_type": "refresh_token",
      "refresh_token": "XifmMBRgF70ufbqMlHNOIlqG8GlkYBNMBTgmwFAT0-6ok"
    }
    ```
  - Respuesta 200: `access_token` (**JWE** `alg:dir, enc:A256GCM`, 86400 s),
    `id_token` (**JWT RS256** legible, `exp-iat` = 3600 s), `scope: openid profile email offline_access`, `token_type: Bearer`.
  - Headers del request: `origin: ionic://localhost`, UA iPhone, `content-type: application/json`.
- **`<DOMINIO_TIGO_API>`** (API base `https://<DOMINIO_TIGO_API>/api/v2.0`):
  - `GET /party/uuid/{customUUID}/accounts?_format=json` → lista de cuentas.
  - `GET /mobile/billing/subscribers/<MSISDN>/balance?_format=json` → saldo (`data.dueAmount`, `data.invoiceId`, `data.dueDate`, `data.dueInvoicesCount`, `data.billingAccountId`, `data.lastInvoiceAmount`).
  - **`GET /mobile/billing/subscribers/<MSISDN>/invoices?_format=json`** → `data.invoiceList[]` con `invoiceId` (`<NUM_FACTURA>`), `billingPeriod` (`formattedValue: "08/2026"`), `invoiceAmount` (700.46), `dueAmount`, `dueDate` (`2026-09-19T00:00:00`), `hasPayment` (bool).
  - Otros endpoints confirmados: `mobile/subscribers/{id}/templates`, `mobile/upselling/subscribers/{id}/availableoffers`, `recurringpayment/mobile/invoices/subscribers/{id}/enrollments`, `payment/mobile/invoices/uuid/{uuid}/cards`, `app/configuration/policy/*`.
- **`<DOMINIO_TIGO_API>`**: `GET /dar/v4/public/users/me` (perfil del usuario; headers adicionales `x-api-key`, `x-app-platform: ios`, `x-app-version: 2.20.0`).

**Hallazgo crítico**: en todas las llamadas a la API, el header `Authorization` es el
**`id_token`** (JWT RS256, payload empieza con `eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6...`),
no el `access_token`. La implementación previa usaba el `access_token` (JWE) y por eso
recibía rechazos. El `id_token` decodificado expone los claims: `custom:UUID`,
`aL` (lista de cuentas con `baId`, `psId`, `bu` mobile/home, `bt` prepaid/hybrid/postpaid,
`sL[].msisdnL[].msisdn`), `ctry: ni`, `sub: auth0|...`, `email`, `name`.

**Datos reales del usuario** (para seed y pruebas):
- `custom:UUID` = `<UUID>`
- Líneas móviles (`psId`): `<MSISDN2>` (prepaid), `<MSISDN>` (hybrid), `<MSISDN3>` (prepaid)
- Cuenta home (`baId` `31042`, `ssId` siga, postpaid)
- `client_id` = `uLf5QUCgJ1vd2GjV5RsLCEFJoQweCpic`

### 3.2 Opciones evaluadas

| Opción | Pros | Contras | Decisión |
|--------|------|---------|----------|
| Usar `id_token` (RS256) como Bearer | La app real lo hace; la API lo acepta | El `id_token` expira en 1 h | ✅ Seleccionada (verificado en captura) |
| Usar `access_token` (JWE) como Bearer | Es el token "de acceso" por convención | JWE cifrado; la API lo rechaza | ❌ Rechazada (causa del bug previo) |
| Refresh automático con `refresh_token` | Renovación sin intervención; `<DOMINIO_TIGO_AUTH0>` no está bloqueado por el WAF | Requiere guardar el `refresh_token` de larga vida | ✅ Seleccionada |
| Token manual estático (1 h) | Simple | Expira y hay que re-proveerlo | ❌ Rechazada (el refresh lo resuelve) |
| uTLS + HTTP/2 custom para imitar navegador | Evita el bloqueo por huella TLS | Complejidad alta; deps nuevas | ❌ Rechazada (la app es HTTP/2 con TLS estándar de Go; el bloqueo es por IP, no por huella) |
| Migración/schema nuevo para Tigo | ... | Reusa todo lo existente | ❌ Rechazada (cero migraciones) |

### 3.3 Decisiones arquitectónicas (ADRs)

**ADR-001**: El `id_token` es el token de acceso a la API
- **Contexto**: La captura muestra que la app envía el `id_token` como Bearer en todas las llamadas; el `access_token` es JWE ilegible.
- **Decisión**: `FetchBills` hace refresh (`grant_type=refresh_token`) y usa el `id_token` de la respuesta como `Authorization: Bearer`. Se decodifica el `id_token` para extraer `custom:UUID`.
- **Consecuencias**: Las peticiones son aceptadas por la API. El token expira en ~1 h pero el refresh lo renueva en cada consulta.

**ADR-002**: El `refresh_token` vive en las credenciales de la cuenta
- **Contexto**: SPEC-002 ya guarda credenciales JSON por cuenta; el `refresh_token` es de larga vida y permite renovar sin captcha.
- **Decisión**: `creds["refresh_token"]` (obligatorio) + `creds["client_id"]` (opcional, default el de la app) + `creds["custom_uuid"]` (opcional, para tokens JWE).
- **Consecuencias**: El usuario obtiene el `refresh_token` una vez (script `tigo-auth.sh` o flujo manual) y el plugin renueva indefinidamente.

**ADR-003**: Cero migraciones nuevas; reuso total de `plugins`/`bills`/webhook
- **Contexto**: SPEC-003/SPEC-004 ya dejaron catálogo, tabla `bills`, asociación `plugin_name` y scheduler/webhook.
- **Decisión**: El plugin Tigo se registra en `main.go` y `SyncCatalog` lo persiste. El foco es el plugin + parsing correcto de `invoices`.
- **Consecuencias**: Menos superficie de cambio y riesgo en iHost.

**ADR-004**: Cliente HTTP estándar (net/http), sin uTLS
- **Contexto**: La implementación previa introdujo `utls` + `http2` para imitar a un navegador iOS. La captura muestra que la app usa HTTP/2 con TLS de Apple, pero el bloqueo real de Cloudflare es por **reputación de IP/ASN** (incluso un Chrome real fue bloqueado desde `190.212.205.109`).
- **Decisión**: Cliente `net/http` estándar con Timeout 15 s y los headers de la app móvil (que sí hicieron pasar las peticiones desde IP residencial). Se eliminan `utls` y `x/net/http2`.
- **Consecuencias**: Menos deps (menos superficie de ataque y tamaño de binario). Si desde la red del iHost la IP está marcada, se documenta el workaround (REQ-009) sin martillar la API.

**ADR-005**: Decodificar JWT con stdlib, sin validar firma en el MVP
- **Contexto**: El token proviene de Auth0 del propio usuario; validar RS256 requeriría JWKS y lógica extra.
- **Decisión**: `encoding/base64` + `encoding/json` para leer claims (`exp`, `custom:UUID`, `aL`). Validación de firma queda como P2.
- **Consecuencias**: Menos código y cero deps.

## 4. Diseño Técnico

### 4.1 Diagrama de arquitectura

```
[React SPA] --(endpoints existentes: /api/plugins, PUT /api/accounts/{id}/plugin,
              POST /api/accounts/{id}/bills:fetch, GET /api/accounts/{id}/bills)--> [Go API (authMiddleware)]
        |                                                                                    |
        v                                                                                    v
[PluginsPage, AccountBillsPage]                                                 [services.BillsService]
                                                                                              |
                                                                                              v
                                                      [internal/plugins: Registry + tigo.nicaragua]
                                                                                              |
                             POST /oauth/token (refresh) --> [<DOMINIO_TIGO_AUTH0>]  (id_token RS256)
                                                                                              |
                                            HTTPS (Bearer id_token + headers app móvil) --> [<DOMINIO_TIGO_API>/api/v2.0]
                                                                                              |
                                                                                              v
                                                    [SQLite: plugins + accounts(plugin_name) + bills]
                                                                                              |
                                          [Scheduler/Webhook existente (SPEC-004) reusa bills, sin cambios]
```

### 4.2 Componentes

#### 4.2.1 Plugin `tigo.nicaragua` (`internal/plugins/tigo/nicaragua/nicaragua.go`)
- **Responsabilidad**: Renovar el token (refresh_token → id_token), listar facturas y normalizarlas a `plugins.Bill`.
- **Interfaz**: Implementa `Plugin`. `FetchBills(ctx, creds, identifier)` con `creds = {"refresh_token": "...", "client_id": "...", "custom_uuid": "..."}` e `identifier` = msisdn de la línea (ej. `<MSISDN>`).
- **Dependencias**: Solo stdlib (`net/http`, `encoding/json`, `encoding/base64`, `time`).
- **Ubicación**: `internal/plugins/tigo/nicaragua/nicaragua.go`; registro en `cmd/server/main.go`.

#### 4.2.2 Backend Go (sin cambios de contrato)
- **Responsabilidad**: Los servicios/handlers existentes de bills/plugins ya orquestan la consulta usando el registry.
- **Ubicación**: `cmd/server/main.go` (registro), sin cambios en `internal/services` ni `internal/api`.

#### 4.2.3 Frontend React
- **Responsabilidad**: Reusa `PluginsPage` y `AccountBillsPage`. Sin cambios obligatorios.
- **Interfaz**: Sin rutas nuevas.

### 4.3 Modelo de datos

```
Sin cambios de esquema. Reuso:

accounts (SPEC-002)
  - account.credentials  = {"refresh_token": "<rt>", "client_id": "uLf5QUCgJ1vd2GjV5RsLCEFJoQweCpic", "custom_uuid": "<UUID>"}
  - account.identifier   = msisdn de la línea (ej. "<MSISDN>")
  - account.plugin_name  = "tigo.nicaragua"

plugins / bills (SPEC-003, sin cambios)
  - tigo.nicaragua se registra en runtime (SyncCatalog → UpsertPlugin)
  - bills: una fila por consulta con plugin_version + source (ej. <DOMINIO_TIGO_API>/api/v2.0)
```

### 4.4 APIs / Contratos

#### Endpoint: `POST /api/accounts/{accountId}/bills:fetch` (existente)

Ejecuta la consulta del plugin asociado a la cuenta. Para Tigo, el plugin:
1. Renueva el token: `POST https://<DOMINIO_TIGO_AUTH0>/oauth/token` con `grant_type=refresh_token`, `client_id`, `refresh_token` → `id_token`.
2. Decodifica el `id_token` → `custom:UUID` (o lo toma de credenciales).
3. `GET /api/v2.0/party/uuid/{customUUID}/accounts?_format=json` → mapea el identifier a un msisdn (si es necesario).
4. `GET /api/v2.0/mobile/billing/subscribers/{msisdn}/invoices?_format=json` → lista de facturas.
5. Normaliza a `[]plugins.Bill` y persiste en `bills`.

**Response 201** (igual que SPEC-003):
```json
{ "id": 43, "account_id": 7, "plugin_name": "tigo.nicaragua", "plugin_version": "1.0.0", "status": "ok", "bills": [{ "period": "2026-08", "amount": "700.46", "due_date": "2026-09-19", "status": "pending" }] }
```

**Response Error** (códigos): `invalid_request` (400), `not_found` (404), `auth_expired` (401), `auth_failed` (401), `upstream_error` (502), `api_changed` (502).

#### Endpoint: `GET /api/accounts/{accountId}/bills` (existente)

**Response 200**: idéntico al de SPEC-003; cada fila trae `plugin_name: "tigo.nicaragua"`, `plugin_version`, `source` y `status`.

> El `refresh_token`/`id_token` NUNCA aparecen en respuestas ni en `raw`.

### 4.5 Dependencias

- **Internas**: `internal/plugins` (interfaz/registry), `internal/services` (`BillsService`), `internal/storage` (plugins/bills).
- **Externas**: **Ninguna nueva** (se ELIMINAN `github.com/refraction-networking/utls` y `golang.org/x/net`). Solo stdlib.

## 5. Criterios de Aceptación

### 5.1 Funcionales

- [ ] CA-001: Dado un `GET /api/plugins` autenticado, aparece `tigo.nicaragua` 1.0.0 con su `source` y descripción.
- [ ] CA-002: Dado un `PUT /api/accounts/{id}/plugin` con `tigo.nicaragua`, la cuenta queda asociada.
- [ ] CA-003: Dado un `POST /api/accounts/{id}/bills:fetch` con `refresh_token` válido en credenciales e `identifier` = msisdn, el plugin renueva el token, consulta `invoices` y persiste `bills` con `status: ok`, `plugin_version` y `source` poblados.
- [ ] CA-004: El `Authorization: Bearer` enviado a la API es el `id_token` (RS256 legible), no el `access_token` JWE.
- [ ] CA-005: Dado un `refresh_token` inválido/vencido, la consulta persiste `bills` con `status: error` y código `auth_failed`.
- [ ] CA-006: Dada una cuenta sin plugin asociado o sin `refresh_token` en credenciales, responde `invalid_request` (400) y no persiste nada.
- [ ] CA-007: Las credenciales/token NUNCA aparecen en respuestas de API ni en `raw`.
- [ ] CA-008: El seed local crea app + cuenta de ejemplo con la data real de Tigo (`identifier <MSISDN>`).
- [ ] CA-009: El parsing de `data.invoiceList[]` convierte `08/2026` → `2026-08`, `dueDate` → `2026-09-19` y `hasPayment` → pending/paid.
- [ ] CA-DARK: N/A (no toca formularios frontend).
- [ ] CA-BACK: N/A (sin páginas nuevas).

### 5.2 No funcionales

- [ ] CA-NF-001: `go build ./...` compila sin errores y sin las deps `utls`/`x/net` (eliminadas).
- [ ] CA-NF-002: `go test ./internal/plugins/tigo/...` pasa.
- [ ] CA-NF-003: Todas las rutas de plugins/bills requieren sesión (`401` sin cookie).
- [ ] CA-NF-004: Toda llamada saliente a Tigo tiene timeout ≤ 15 s y no bloquea otras peticiones.

### 5.3 Testing

- **Unit tests**: Decodificación del `id_token` (claims `exp`, `custom:UUID`, `aL`), refresh (access_token + id_token), normalización de `invoiceList`, mapeo de errores (`auth_failed`, `api_changed`).
- **Integration tests**: Flujo con fixture: `refresh_token` → `/oauth/token` → `id_token` → `accounts` → `invoices` → `bills` persistido con `plugin_version` y `source`.
- **E2E tests**: Desde la UI, consultar facturas de una cuenta Tigo con `refresh_token` real.
- **Carga/Performance**: Una consulta < 5 s; sin picos de RAM (sin deps, sin pools).

## 6. Plan de Implementación

### 6.1 Fases

| Fase | Descripción | Estimación | Dependencias |
|------|-------------|------------|--------------|
| 1 | Reescribir la spec con los datos reales de la captura (este documento) | 0.25 día | Ninguna |
| 2 | Plugin `tigo.nicaragua` v1.0.0: refresh (id_token), `GET .../invoices`, parsing de `invoiceList`, normalización a `Bill`, errores de dominio | 1 día | Fase 1 |
| 3 | Registro en `main.go` + `SyncCatalog`; verificación de endpoints existentes | 0.25 día | Fase 2 |
| 4 | Eliminar deps `utls`/`x/net`; seed local de app+cuenta Tigo | 0.25 día | Fase 2 |
| 5 | Tests unitarios/integración + `go build`/`go test`/`npm run build` | 0.5 día | Fase 4 |
| 6 | Validación end-to-end con el `refresh_token` real del usuario desde una IP permitida | 0.5 día | Fase 5 |

### 6.2 Milestones

- **MVP**: Plugin `tigo.nicaragua` v1.0.0 con refresh automático (id_token), lista de facturas vía `invoices`, seed local, webhook reutilizado.
- **V1.1** (opcional): Extracción automática del `refresh_token` (script `tigo-auth.sh`) y validación JWKS.

## 7. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| Cloudflare bloquea la IP del iHost (403) | Alta | Alto | La app usa HTTP/2 estándar; el bloqueo real es por reputación de IP/ASN. Mitigación: probar desde IP residencial; documentar workaround (proxy/otra red); no martillar la API (frecuencia moderada). |
| El `refresh_token` expira o se revoca | Media | Medio | Error `auth_failed` claro en la UI; script `tigo-auth.sh` para regenerarlo. |
| Tigo cambia el path/formato del API | Media | Alto | Versionamiento obligatorio (SPEC-003): bump + stamp en `bills`; error `api_changed` avisa al usuario. |
| El `id_token` expira (1 h) entre refresh y consulta | Baja | Medio | El refresh se hace en cada `FetchBills`; no se reutiliza un token viejo. |
| El login web pide captcha | Media | Bajo | Se usa el flujo móvil `username-password-authentication` sin captcha (verificado) para obtener el `refresh_token` una vez. |

## 8. Notas y Referencias

- API base: `https://<DOMINIO_TIGO_API>/api/v2.0`
- Auth0 de Tigo: `https://<DOMINIO_TIGO_AUTH0>/oauth/token`, client_id `uLf5QUCgJ1vd2GjV5RsLCEFJoQweCpic`
- Captura real (Charles, `.chlz`): `<DOMINIO_TIGO_AUTH0>`, `<DOMINIO_TIGO_API>`, `<DOMINIO_TIGO_API>`
- Endpoint de facturas: `GET /mobile/billing/subscribers/{msisdn}/invoices?_format=json`
- Endpoint de saldo: `GET /mobile/billing/subscribers/{msisdn}/balance?_format=json`
- Endpoint de cuentas: `GET /party/uuid/{customUUID}/accounts?_format=json`
- Identificadores: msisdn `<MSISDN>` (hybrid), `<MSISDN2>`, `<MSISDN3>` (prepaid); UUID `<UUID>`
- Reutiliza: SPEC-002 (cuentas/credenciales), SPEC-003 (infraestructura de plugins/bills/endpoints), SPEC-004 (scheduler/webhook)
- Script de obtención del `refresh_token`: `scripts/tigo-auth.sh`
- Repo: `https://github.com/paulomcnally/p40la-ihost-automation`

## 9. Historial de Cambios

| Fecha | Autor | Descripción |
|-------|-------|-------------|
| 2026-09-12 | paulomcnally | **Reescritura total basada en la captura real de Charles Proxy de la app Mi Tigo (iOS)**. Hallazgo crítico: la API acepta el `id_token` (RS256 legible), NO el `access_token` (JWE cifrado); el plugin previo usaba el access_token y por eso fallaba. Autenticación por `refresh_token` de larga vida (`grant_type=refresh_token` en `<DOMINIO_TIGO_AUTH0>`, no bloqueado por el WAF), renovado en cada `FetchBills`. Endpoints confirmados: `accounts`, `balance`, `invoices` (`data.invoiceList[]`). Se eliminan `utls`/`x/net` (bloqueo real por reputación de IP, no por huella TLS). Headers de la app móvil verificados. Se descarta el diseño previo (token manual de 1 h, cuentas por Drupal `_format=json` suelto, cookies Cloudflare). |
| 2026-09-12 | paulomcnally | **Release**. Commit `a7e2941`. Verificado end-to-end en local: `POST /api/accounts/10/bills:fetch` con el `refresh_token` real devolvió las 6 facturas (<NUM_FACTURA> pendiente C$700.46 + 5 pagadas C$699.99). Catálogo muestra `tigo.nicaragua` v1.0.0. Issue #8 cerrado con label `spec/released`. |