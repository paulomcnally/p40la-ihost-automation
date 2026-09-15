---
title: "Plugin enacal.nicaragua: autenticación y facturas"
id: "SPEC-012"
status: "released"
author: "paulomcnally"
created: "2026-09-12"
updated: "2026-09-12"
github_issue: 13
---

# Plugin enacal.nicaragua: autenticación y facturas

**ID**: SPEC-012  
**Estado**: released  
**Autor**: paulomcnally  
**Creado**: 2026-09-12  
**Actualizado**: 2026-09-12

---

## 1. Resumen Ejecutivo

Se necesita un plugin de integración llamado `enacal.nicaragua` que consulte la
**lista de facturas** de agua potable de una cuenta de **ENACAL** (Empresa
Nicaragüense de Acueductos y Alcantarillados Sanitarios) usando el portal en línea
`https://<DOMINIO_ENACAL>` (SPA AngularJS "SOE" servida por ASP.NET/IIS detrás de
un WAF F5 BIG-IP ASM). El usuario capturó con Charles Proxy una sesión real del
portal (`/home/paulomcnally/enacal.chlz`) que incluye el tráfico de **autenticación**
(`POST /soe/api/account/authenticate` con `{username, password}`) y el tráfico de
**facturas/pagos** del NIC `<NIC>`.

El flujo es sencillo y **sin token de sesión persistente**: la autenticación
(`POST /soe/api/account/authenticate`) valida `{username, password}` y devuelve
`{"success":true}` (además fija cookies del WAF F5: `f5_cspm`, `TS01381d51`,
`TS56f84491027`, `f5avra...session_`). A partir de ahí, **todos** los requests
llevan `Authorization: Basic base64(username:password)` y replican los headers del
portal (UA de iPhone Safari, `Origin`/`Referer` del propio portal, `Sec-Fetch-Site:
same-origin`). Las facturas se obtienen con
`GET /soe/api/InfoCuentas/facturas/?CuentaNro=<NIC>&filter=` → array de facturas
con `NoDoc` (ej. `<NUM_FACTURA>`), `TotalFac`, `Pendiente`, `Emision`, `Vence`,
`Pagado_Por`, `Fecha_Recibo` y `UrlFactura` (PDF en `<DOMINIO_ENACAL_WSE>`).

Este plugin sigue el contrato de infraestructura de SPEC-003/SPEC-007/SPEC-009
(interfaz `Plugin`, registro, tabla `bills`, asociación `accounts.plugin_name`).
**No requiere cambios de esquema.** Reutiliza las credenciales JSON libres de la
cuenta (`accounts.credentials`) para guardar `username` y `password`, nunca en
código. El resultado esperado: plugin `enacal.nicaragua` registrado, `FetchBills`
autentica con `username`+`password`, consulta las facturas del NIC (identifier) y
normaliza cada una a `[]plugins.Bill` con periodo `YYYY-MM` (derivado de `Emision`),
monto real (`TotalFac`), vencimiento (`Vence`), estado derivado de `Pendiente` y
`invoice_number` = `NoDoc`. El job/webhook existente (SPEC-004/SPEC-008/SPEC-010)
reenvía las facturas con el payload correcto
(`year`, `month`, `amount`, `status`, `invoice_number`).

**Importante sobre credenciales**: el tráfico capturado expone el `username`
(`<USER>`), el NIC (`<NIC>`) y el `password` del usuario. **No deben
hardcodearse en el código.** Se usan únicamente para poblar `accounts.credentials`
en la base SQLite local de desarrollo (vía `seed-apps.sh` con variables de entorno),
y el plugin los lee de ahí en runtime. Nada se persiste ni se loguea. Para la prueba
local se configura la cuenta real en SQLite (como el usuario pidió), nunca en el repo.

## 2. Requerimientos

### 2.1 Requerimientos Funcionales (P0 - Obligatorios)

1. **REQ-001**: Plugin `enacal.nicaragua` que implementa la interfaz `Plugin` existente (`Name`, `Version`, `Source`, `Description`, `CredentialSchema`, `FetchBills`), se registra en `main.go` y aparece en el catálogo `plugins` vía `SyncCatalog` (sin migración nueva).
2. **REQ-002**: Autenticación con credenciales de la cuenta: `POST https://<DOMINIO_ENACAL>/soe/api/account/authenticate` con `Content-Type: application/json;charset=utf-8` y body `{"username":"<USER>","password":"<PASS>"}` → respuesta 200 `{"success":true}`. `FetchBills` la usa como validación de credenciales y conserva las cookies del WAF F5 (jar en memoria).
3. **REQ-003**: Lista de facturas: `GET /soe/api/InfoCuentas/facturas/?CuentaNro=<NIC>&filter=` con `Authorization: Basic base64(username:password)` → array JSON de facturas (ej. `<NUM_FACTURA>`, `<NUM_FACTURA>`, ...). El NIC viene del `identifier` de la cuenta (ej. `<NIC>`).
4. **REQ-004**: Normalización a `[]plugins.Bill`:
   - `Period`: `YYYY-MM` derivado de `Emision` (`"29/08/2026"` → `"2026-08"`), compatible con `parseBillPeriod` (SPEC-008).
   - `Amount`: `TotalFac` formateado sin ceros de más (ej. `"28.19"`, `"37.86"`).
   - `DueDate`: `Vence` convertido a `YYYY-MM-DD` (`"20/09/2026"` → `"2026-09-20"`).
   - `Status`: `pending` si `Pendiente > 0` (o `Pagado_Por` vacío); `paid` si `Pendiente <= 0` (la captura muestra `Pendiente: 0.0` + `Pagado_Por: "<NUM_PAGO>"` en facturas pagadas).
   - `Raw`: debe incluir `numFactura` = `NoDoc` (clave leída por `buildWebhookPayload`) y opcionalmente `emision`, `vence`, `totalFac`, `pendiente`, `pagadoPor`, `fechaRecibo`, `urlFactura`.
5. **REQ-005**: Headers obligatorios en todas las llamadas (replicando el portal, verificados en la captura): `Authorization: Basic base64(username:password)`, `Accept: application/json, text/plain, */*`, `User-Agent` iPhone (`Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.6 Mobile/15E148 Safari/604.1`), `Accept-Language: es-419,es;q=0.9`, `Origin: https://<DOMINIO_ENACAL>`, `Referer: https://<DOMINIO_ENACAL>/soe/`, `Sec-Fetch-Site: same-origin`. El cliente HTTP conserva en un jar las cookies del WAF F5 de la primera request.
6. **REQ-006**: `invoice_number` en el webhook: al poner `Raw["numFactura"]`, el `buildWebhookPayload` existente (`webhook.go`) emite `invoice_number` = `NoDoc` (ej. `<NUM_FACTURA>`). Verificar que el payload final es correcto (`year`, `month`, `amount`, `status`, `invoice_number`) — formato del contrato `docs/webhooks-api.md`.
7. **REQ-007**: Seed local para pruebas: `scripts/seed-apps.sh` soporta el plugin ENACAL (`PLUGIN_NAME=enacal.nicaragua`, `IDENTIFIERS=<NIC>`, `CREDENTIALS='{"username":"...","password":"..."}'`) y la descripción por defecto. Las credenciales reales se proveen vía entorno, nunca hardcodeadas. Para la prueba local se configura la cuenta real del usuario en SQLite.

### 2.2 Requerimientos Funcionales (P1 - Importantes)

1. **REQ-008**: Manejo de errores estructurado mapeado a los errores del dominio (`ErrAuthFailed`, `ErrUpstream`, `ErrAPISchemaChanged`, `ErrAuthExpired`), persistidos como `bills` con `status: error` y `plugin_version`, sin exponer credenciales.
2. **REQ-009**: `CredentialSchema` (SPEC-011): `username` (text, required, no secret) y `password` (password, required, secret), para que la UI genere el formulario de credenciales dinámico del plugin.
3. **REQ-010**: El `identifier` de la cuenta es el **NIC** (`CuentaNro`, ej. `<NIC>`). `FetchBills` lo usa directamente en `CuentaNro=` de `facturas/`. Validar que no esté vacío antes de consultar.

### 2.3 Requerimientos Funcionales (P2 - Deseables)

1. **REQ-011**: Descarga del PDF de cada factura (`UrlFactura` apunta a `https://<DOMINIO_ENACAL_WSE>/AquaMailFac/servlet/com.aquavisumsgc.repfacenacalmail?...`) para adjuntar al webhook (si el receptor lo soporta). Nota: `<DOMINIO_ENACAL_WSE>` es un host distinto y puede exigir su propia autenticación/cookies.
2. **REQ-012**: Listado de cuentas asociadas (`GET /soe/api/cuentasAsociadas/search/?filter=&username=<USER>` → `[{CuentaId, CuentaNro, Nombre, ...}]`) para resolver el NIC a partir del usuario si el identifier no se conoce, y endpoints complementarios `pagos/`, `lecturas/`, `exoneraciones/`, `contactosCuentas/getContacts` como dato opcional.

### 2.4 Requerimientos No Funcionales

- **Rendimiento**: Timeouts obligatorios (≤ 15 s) en llamadas HTTP salientes. Una consulta < 5 s.
- **Seguridad**: El `username`/`password` jamás se exponen en respuestas de API ni en `raw`; viven solo en el JSON de credenciales de la cuenta y en memoria durante la consulta. No hardcodear credenciales en el código.
- **Almacenamiento**: Reusa la tabla `bills`; cero migraciones nuevas.
- **Disponibilidad**: Todas las rutas detrás de `authMiddleware`; `/health` sin cambios.
- **iHost**: Cero dependencias nuevas (Go stdlib `net/http`). Parsing de fechas con `time` (stdlib). Sin persistencia de cookies en disco.

## 3. Investigación y Decisiones Técnicas

### 3.1 Contexto investigado (captura Charles Proxy real, 2026-09-12)

Se capturó una sesión real del portal ENACAL (`https://<DOMINIO_ENACAL>`, SPA
AngularJS "SOE") desde `192.168.1.197` mediante Charles Proxy
(`/home/paulomcnally/enacal.chlz`). Backend **ASP.NET** detrás de **F5 BIG-IP ASM**
(cookies `f5_cspm`, `TS01381d51`, `TS56f84491027`, `f5avra...session_`). Endpoints
verificados en el orden del portal:

- **`GET /soe/`** → 200 HTML; `Set-Cookie`: `f5avra...session_`, `f5_cspm=1234`,
  `TS01381d51`, `TS56f84491027` (cookies del WAF F5; no son de sesión de la API).
- **`POST /soe/api/account/authenticate`** — body JSON
  `{"username":"<USER>","password":"<PASS>"}` → 200 `{"success":true}`.
- **`GET /soe/api/cuentasAsociadas/search/?filter=&username=<USER>`** —
  `Authorization: Basic <base64(user:pass)>` → array con `CuentaId: <CUENTA_ID>`,
  `CuentaNro: <NIC>`, `Nombre: "<NOMBRE>"`, más `Deuda`,
  `Servicio`, `Corte`, `Convenio`, `Requerimientos`.
- **`GET /soe/api/InfoCuentas/facturas/?CuentaNro=<NIC>&filter=`** — array de
  **facturas**: `Emision` (`29/08/2026`), `Vence` (`20/09/2026`),
  `NoDoc` (`<NUM_FACTURA>`), `TotalFac` (`28.19`), `Pendiente` (`28.19`),
  `Pagado_Por` (`""` o `<NUM_PAGO>`), `Fecha_Recibo` (`null` o `2026-08-17`),
  `UrlFactura` (`https://<DOMINIO_ENACAL_WSE>/AquaMailFac/servlet/...?<NIC>,<hash>`).
- **`GET /soe/api/InfoCuentas/pagos/?CuentaNro=<NIC>&filter=`** — array de pagos:
  `Emision`, `NoPago` (`<NUM_PAGO>`), `Total`, `Pendiente`, `Estado_Recibo`,
  `Realizado_con` (`BAC (DEBITO)`).
- Otros endpoints capturados: `GET /soe/api/InfoCuentas/lecturas/`,
  `GET /soe/api/InfoCuentas/exoneraciones/`,
  `GET /soe/api/contactosCuentas/getContacts/?CuentaNro=<NIC>`,
  `GET /soe/api/Tabs/`, `GET /soe/api/servicio/`, `GET /soe/api/tipoIdentificacion/`.

**Flujo de auth confirmado en el JS del portal** (`membershipService.js`):
1. `login(user)` → `POST api/account/authenticate` con `{username, password}`.
2. En éxito, `saveCredentials(user)`:
   - `authdata = base64(username + ":" + password)`.
   - `$http.defaults.headers.common['Authorization'] = 'Basic ' + authdata`.
   - guarda cookie `repository={"loggedUser":{"username":"...","authdata":"base64"}}`.

Es decir, **no hay token**; cada request reusa el **Basic auth** y las cookies F5.
El plugin puede reconstruir el header `Authorization` en cada llamada a partir de
`creds["username"]` y `creds["password"]`.

**Headers típicos de request** (todas): `Host: <DOMINIO_ENACAL>`,
`Accept: application/json, text/plain, */*`, `Sec-Fetch-Site: same-origin`,
`Sec-Fetch-Mode: cors`, `User-Agent: Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like
Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.6 Mobile/15E148
Safari/604.1`, `Referer: https://<DOMINIO_ENACAL>/soe/`, `Origin: https://<DOMINIO_ENACAL>`,
`Accept-Language: es-419,es;q=0.9`, cookies F5.

**Datos reales del usuario** (para seed local vía entorno, NO en código):
- `username` = `<USER>` (credencial → `accounts.credentials`)
- `password` = `<PASS>` (credencial → `accounts.credentials`)
- NIC (`CuentaNro`) = `<NIC>` (identifier de la cuenta)
- Facturas reales: `<NUM_FACTURA>` (2026-08, C$28.19, **pendiente**), `<NUM_FACTURA>`
  (2026-07, C$37.86, pagada `<NUM_PAGO>`), `<NUM_FACTURA>` (2026-06, C$47.53),
  `<NUM_FACTURA>` (2026-05, C$57.20), `<NUM_FACTURA>` (2026-04, C$66.87),
  `<NUM_FACTURA>` (2026-03, C$37.86).

### 3.2 Opciones evaluadas

| Opción | Pros | Contras | Decisión |
|--------|------|---------|----------|
| Auth por `username`+`password` → Basic header (sin token) | El portal real lo hace; el header se reconstruye en cada request | El `password` viaja en el body del `authenticate` y en el header Basic (HTTPS) | ✅ Seleccionada (verificado en captura + JS) |
| Token manual / sesión persistente | Simple de implementar | La API no devuelve token (respuesta `{"success":true}`) | ❌ Rechazada |
| Cookie `repository` del portal (localStorage→cookie) | Replica exacta del navegador | Es una cookie de la SPA, no un mecanismo de la API; el plugin ya reconstruye el Basic | ❌ Rechazada (no necesario) |
| Usar `pagos/` para derivar estado | Datos de pago explícitos | `facturas/` ya trae `Pendiente` y `Pagado_Por`; una llamada menos | ❌ Rechazada (facturas es suficiente) |
| Descargar el PDF en el MVP | Adjunto completo de la factura | Host distinto (subdominio dedicado) + costo de descarga/almacenamiento; el receptor aún no lo soporta | ❌ Rechazada para el MVP (documentado en REQ-011) |
| Migración/schema nuevo para ENACAL | ... | Reusa todo lo existente | ❌ Rechazada (cero migraciones) |

### 3.3 Decisiones arquitectónicas (ADRs)

**ADR-001**: Autenticación por `username`+`password` con header `Authorization: Basic`
- **Contexto**: La captura muestra que `POST /soe/api/account/authenticate` valida las
  credenciales y devuelve `{"success":true}`, y que el resto de llamadas usan
  `Authorization: Basic base64(username:password)`.
- **Decisión**: `FetchBills` llama `authenticate` con `{username, password}`, conserva
  las cookies del WAF F5 en un jar en memoria y reenvía `Authorization: Basic` en
  cada request. No persiste nada.
- **Consecuencias**: El `password` debe vivir en las credenciales de la cuenta (SQLite)
  y no en código.

**ADR-002**: Las credenciales (`username`, `password`) viven en `accounts.credentials`
- **Contexto**: SPEC-002 ya guarda credenciales JSON por cuenta; SPEC-011 genera el
  formulario dinámico desde `CredentialSchema`.
- **Decisión**: `creds["username"]` (obligatorio) + `creds["password"]` (obligatorio,
  `Secret: true`). Nunca hardcodear los valores reales; el seed los toma de variables
  de entorno. Para la prueba local se cargan en SQLite.
- **Consecuencias**: El plugin lee las credenciales en runtime y no expone nada.

**ADR-003**: Cero migraciones nuevas; reuso total de `plugins`/`bills`/webhook
- **Contexto**: SPEC-003/004/007/008/010 dejaron catálogo, tabla `bills`, asociación
  `plugin_name`, scheduler/webhook con `parseBillPeriod` tolerante y
  `extractInvoiceNumber` (clave `numFactura`).
- **Decisión**: El plugin ENACAL se registra en `main.go` y `SyncCatalog` lo persiste.
  Emite `Period` en `YYYY-MM` (compatible con `parseBillPeriod`) y `Raw["numFactura"]`.
- **Consecuencias**: Menos superficie de cambio y riesgo en iHost.

**ADR-004**: Cliente HTTP estándar (`net/http`) con jar de cookies, sin deps nuevas
- **Contexto**: El portal está tras un WAF F5 BIG-IP ASM que fija cookies
  (`TS*`, `f5*`) en la primera request. Las llamadas de la captura fueron aceptadas
  desde IP residencial sin JS challenge.
- **Decisión**: `net/http` con `cookiejar` en memoria y Timeout 15 s, replicando los
  headers del portal. Cero deps externas.
- **Consecuencias**: Binario pequeño para iHost; si el WAF empezara a exigir un JS
  challenge, sería un riesgo a mitigar (ver §7).

**ADR-005**: Periodo `YYYY-MM` derivado de `Emision` y vencimiento de `Vence`
- **Contexto**: La API expone `Emision` y `Vence` en `DD/MM/YYYY`.
- **Decisión**: `Period = "YYYY-MM"` (ej. `2026-08`), `DueDate = "YYYY-MM-DD"`
  (ej. `2026-09-20`). Ambos compatibles con los formatos que ya maneja el sistema.
- **Consecuencias**: Payload de webhook correcto con `year`/`month` reales.

## 4. Diseño Técnico

### 4.1 Diagrama de arquitectura

```
[React SPA] --(endpoints existentes: /api/plugins, PUT /api/accounts/{id}/plugin,
              POST /api/accounts/{id}/bills:fetch, GET /api/accounts/{id}/bills)--> [Go API (authMiddleware)]
        |                                                                                      |
        v                                                                                      v
[PluginsPage, AccountBillsPage]                                                   [services.BillsService]
                                                                                                  |
                                                                                                  v
                                                         [internal/plugins: Registry + enacal.nicaragua]
                                                                                                  |
                                              POST /soe/api/account/authenticate ({user,pass}) --> <DOMINIO_ENACAL> (F5 ASM → IIS/ASP.NET)
                                                                                                  |
                                              GET  /soe/api/InfoCuentas/facturas/?CuentaNro=<NIC>  --> [SPA AngularJS SOE]
                                                                                                  |
                                                                                                  v
                                                      [SQLite: plugins + accounts(plugin_name + credentials) + bills]
                                                                                                  |
                                            [Scheduler/Webhook existente (SPEC-004/008/010) reusa bills, sin cambios]
```

### 4.2 Componentes

#### 4.2.1 Plugin `enacal.nicaragua` (`internal/plugins/enacal/nicaragua/nicaragua.go`)
- **Responsabilidad**: Autenticar con `username`+`password`, listar facturas del NIC y normalizarlas a `plugins.Bill`.
- **Interfaz**: Implementa `Plugin`. `FetchBills(ctx, creds, identifier)` con
  `creds = {"username": "...", "password": "..."}` e `identifier` = NIC (ej. `<NIC>`).
- **Dependencias**: Solo stdlib (`net/http`, `net/http/cookiejar`, `encoding/json`, `encoding/base64`, `strings`, `time`).
- **Ubicación**: `internal/plugins/enacal/nicaragua/nicaragua.go`; registro en `cmd/server/main.go`.

#### 4.2.2 Backend Go (sin cambios de contrato)
- **Responsabilidad**: Los servicios/handlers existentes de bills/plugins ya orquestan la consulta usando el registry.
- **Ubicación**: `cmd/server/main.go` (registro), sin cambios en `internal/services` ni `internal/api`.

#### 4.2.3 Frontend React
- **Responsabilidad**: Reusa `PluginsPage` y `AccountBillsPage`. Sin cambios obligatorios.
- **Interfaz**: Sin rutas nuevas. El formulario de credenciales sale de `CredentialSchema` (SPEC-011).

### 4.3 Modelo de datos

```
Sin cambios de esquema. Reuso:

accounts (SPEC-002)
  - account.credentials  = {"username": "<USER>", "password": "<PASS>"}
  - account.identifier   = NIC de la cuenta (ej. "<NIC>")
  - account.plugin_name  = "enacal.nicaragua"

plugins / bills (SPEC-003, sin cambios)
  - enacal.nicaragua se registra en runtime (SyncCatalog → UpsertPlugin)
  - bills: una fila por consulta con plugin_version + source (https://<DOMINIO_ENACAL>)
```

### 4.4 APIs / Contratos

#### Endpoint: `POST /api/accounts/{accountId}/bills:fetch` (existente)

Ejecuta la consulta del plugin asociado a la cuenta. Para ENACAL, el plugin:
1. (opcional) `GET https://<DOMINIO_ENACAL>/soe/` con jar de cookies → cookies del WAF F5.
2. `POST https://<DOMINIO_ENACAL>/soe/api/account/authenticate` con body
   `{"username":"<USER>","password":"<PASS>"}` → `{"success":true}` (valida credenciales).
3. `GET https://<DOMINIO_ENACAL>/soe/api/InfoCuentas/facturas/?CuentaNro=<NIC>&filter=`
   con `Authorization: Basic base64(<USER>:<PASS>)` → array de facturas.
4. Normaliza cada factura a `plugins.Bill` (REQ-004) y persiste en `bills`.

**Response 201** (igual que SPEC-003/007/009):
```json
{
  "id": 45, "account_id": 8, "plugin_name": "enacal.nicaragua",
  "plugin_version": "1.0.0", "status": "ok",
  "bills": [
    { "period": "2026-08", "amount": "28.19", "due_date": "2026-09-20", "status": "pending" },
    { "period": "2026-07", "amount": "37.86", "due_date": "2026-08-20", "status": "paid" }
  ]
}
```

**Response Error** (códigos): `invalid_request` (400), `not_found` (404), `auth_expired` (401),
`auth_failed` (401), `upstream_error` (502), `api_changed` (502).

#### Endpoint: `GET /api/accounts/{accountId}/bills` (existente)

**Response 200**: idéntico al de SPEC-003; cada fila trae `plugin_name: "enacal.nicaragua"`,
`plugin_version`, `source` y `status`.

> El `username`/`password` NUNCA aparecen en respuestas ni en `raw`.

#### Payload enviado al webhook (formato del contrato `docs/webhooks-api.md`)

Derivado por `buildWebhookPayload` (`internal/services/webhook.go`):
```json
{
  "year": 2026,
  "month": 8,
  "amount": 28.19,
  "status": "pending",
  "invoice_number": "<NUM_FACTURA>"
}
```

`year`/`month` salen de `Period` (`2026-08`), `amount` de `TotalFac`, `status` de
`Pendiente` y `invoice_number` de `Raw["numFactura"]` (vía `extractInvoiceNumber`,
SPEC-010). El formato es idéntico al de Claro/DISNORTE/Tigo.

### 4.5 Dependencias

- **Internas**: `internal/plugins` (interfaz/registry), `internal/services` (`BillsService`), `internal/storage` (plugins/bills).
- **Externas**: **Ninguna nueva**. Solo stdlib.

## 5. Criterios de Aceptación

### 5.1 Funcionales

- [ ] CA-001: Dado un `GET /api/plugins` autenticado, aparece `enacal.nicaragua` 1.0.0 con su `source` y descripción.
- [ ] CA-002: Dado un `PUT /api/accounts/{id}/plugin` con `enacal.nicaragua`, la cuenta queda asociada.
- [ ] CA-003: Dado un `POST /api/accounts/{id}/bills:fetch` con credenciales válidas en SQLite e `identifier` = NIC, el plugin autentica, consulta `facturas/` y persiste `bills` con `status: ok`, `plugin_version` y `source` poblados.
- [ ] CA-004: Cada factura se normaliza con `Period: "YYYY-MM"` (de `Emision`), `Amount` = `TotalFac`, `DueDate` = `Vence` en `YYYY-MM-DD` y `Status` derivado de `Pendiente`.
- [ ] CA-005: El `Raw` de cada bill incluye `numFactura` = `NoDoc`, y `buildWebhookPayload` emite `invoice_number` correcto (ej. `<NUM_FACTURA>`) con el payload `{year, month, amount, status, invoice_number}`.
- [ ] CA-006: Dado un `password` inválido, la consulta persiste `bills` con `status: error` y código `auth_failed` (sin exponer credenciales).
- [ ] CA-007: Las credenciales (`username`/`password`) NUNCA aparecen en respuestas de API ni en `raw`.
- [ ] CA-008: El seed local crea app + cuenta de ejemplo con la data real de ENACAL (`identifier <NIC>`) usando credenciales de entorno, y permite probar la consulta end-to-end desde la UI local.
- [ ] CA-DARK: N/A (no toca formularios frontend; reusa `CredentialSchema` de SPEC-011).
- [ ] CA-BACK: N/A (sin páginas nuevas).

### 5.2 No funcionales

- [ ] CA-NF-001: `go build ./...` compila sin errores.
- [ ] CA-NF-002: `go test ./internal/plugins/enacal/...` y `go test ./internal/services/...` pasan.
- [ ] CA-NF-003: Sin dependencias nuevas en `go.mod`.
- [ ] CA-NF-004: Toda llamada saliente a ENACAL tiene timeout ≤ 15 s y no bloquea otras peticiones.

### 5.3 Testing

- **Unit tests**: Parsing del `authenticate` (`{"success":true}`), parsing de `facturas[]`,
  normalización a `Bill` (periodo `DD/MM/YYYY` → `YYYY-MM`, monto, vencimiento, estado de
  `Pendiente`), base64 del header `Authorization`, mapeo de errores (`auth_failed`, `api_changed`).
- **Integration tests**: Flujo con fixture: `GET /soe/` (cookies F5) → `authenticate` →
  `facturas/` → `bills` persistido; y `buildWebhookPayload` con `Raw["numFactura"]`
  produce `{year, month, amount, status, invoice_number}`.
- **E2E tests**: Desde la UI, consultar facturas de una cuenta ENACAL con credenciales reales (local).
- **Carga/Performance**: Una consulta < 5 s; sin picos de RAM (sin deps, sin pools).

## 6. Plan de Implementación

### 6.1 Fases

| Fase | Descripción | Estimación | Dependencias |
|------|-------------|------------|--------------|
| 1 | Espec con datos reales de la captura (este documento) | 0.25 día | Ninguna |
| 2 | Plugin `enacal.nicaragua` v1.0.0: authenticate (user+pass), cookies F5, facturas paginadas/lista, parsing, normalización a `Bill`, errores de dominio | 1 día | Fase 1 |
| 3 | Registro en `main.go` + `SyncCatalog`; verificación de `buildWebhookPayload` con `numFactura` (formato del webhook) | 0.25 día | Fase 2 |
| 4 | Seed local de app+cuenta ENACAL (credenciales por entorno, configuradas en SQLite para prueba local) | 0.25 día | Fase 2 |
| 5 | Tests unitarios/integración + `go build`/`go test`/`npm run build` | 0.5 día | Fase 4 |
| 6 | Validación end-to-end local con las credenciales reales del usuario | 0.5 día | Fase 5 |

### 6.2 Milestones

- **MVP**: Plugin `enacal.nicaragua` v1.0.0 con auth `username`+`password`, lista de
  facturas, normalización a `Bill`, seed local (cuenta real en SQLite) y webhook con
  `invoice_number` correcto.
- **V1.1** (opcional): Descarga del PDF (REQ-011) y listado de cuentas asociadas (REQ-012).

## 7. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| El WAF F5 BIG-IP ASM exige JS challenge desde el iHost (cookies TS no basta) | Media | Alto | Las llamadas de la captura fueron aceptadas desde IP residencial con un simple `GET /soe/` (sin JS challenge). Mitigación: replicar el primer request que fija cookies; probar desde la red del usuario; no martillar la API. |
| El `password` cambia o se bloquea | Media | Medio | Error `auth_failed` claro en la UI; actualizar credenciales de la cuenta. |
| ENACAL cambia el path/formato del API | Media | Alto | Versionamiento obligatorio (SPEC-003): bump + stamp en `bills`; error `api_changed` avisa al usuario. |
| `Emision`/`Vence` cambian de formato (DD/MM/YYYY) | Baja | Medio | Parser tolerante (SPEC-008/010) + test de fixture; si cambia, se ajusta el plugin. |
| El endpoint `facturas/` requiere `username` en query (como `cuentasAsociadas`) | Baja | Bajo | Se puede añadir `&username=<USER>` sin romper la consulta (opcional, documentado en REQ-012). |
| `UrlFactura` (PDF) requiere sesión/cookies propias en `<DOMINIO_ENACAL_WSE>` | Media | Medio | Fuera del MVP; documentado en REQ-011 para la V1.1. |

## 8. Notas y Referencias

- Portal: `https://<DOMINIO_ENACAL>` (SPA AngularJS "SOE", IIS/ASP.NET tras F5 BIG-IP ASM).
- Endpoint de auth: `POST /soe/api/account/authenticate` (JSON `{username, password}` → `{"success":true}`).
- Endpoint de facturas: `GET /soe/api/InfoCuentas/facturas/?CuentaNro=<NIC>&filter=` (con `Authorization: Basic`).
- Endpoint de pagos: `GET /soe/api/InfoCuentas/pagos/?CuentaNro=<NIC>&filter=`.
- Listado de cuentas: `GET /soe/api/cuentasAsociadas/search/?filter=&username=<USER>`.
- PDF de factura: `UrlFactura` → `https://<DOMINIO_ENACAL_WSE>/AquaMailFac/servlet/...` (host distinto).
- Captura real (Charles, `.chlz`): `/home/paulomcnally/enacal.chlz` (y `/home/paulomcnally/Downloads/enacal`).
- Datos reales del usuario (solo seed local vía entorno): `username` `<USER>`, `password` `<PASS>`, NIC `<NIC>`.
- Reutiliza: SPEC-002 (cuentas/credenciales), SPEC-003 (infraestructura de plugins/bills/endpoints), SPEC-004 (scheduler/webhook), SPEC-008 (`parseBillPeriod` tolerante), SPEC-010 (`extractInvoiceNumber`), SPEC-011 (`CredentialSchema` + formulario dinámico).
- Repo: `https://github.com/paulomcnally/p40la-ihost-automation`

## 9. Historial de Cambios

| Fecha | Autor | Descripción |
|-------|-------|-------------|
| 2026-09-12 | paulomcnally | Creación inicial de la especificación basada en la captura real de Charles Proxy del portal ENACAL (`enacal.chlz`). Flujo confirmado: `GET /soe/` (cookies WAF F5) → `POST /soe/api/account/authenticate` (`{username,password}` → `{"success":true}`) → `GET /soe/api/InfoCuentas/facturas/?CuentaNro=<NIC>` con `Authorization: Basic`. Sin token de sesión. Cero migraciones, cero deps. Credenciales nunca en código (solo `accounts.credentials` en SQLite local vía seed por entorno). Webhook con formato `{year, month, amount, status, invoice_number}` vía `Raw["numFactura"]`. |
| 2026-09-12 | paulomcnally | Estado → `pending_execution` → `in_progress`. Inicio de desarrollo. |
| 2026-09-12 | paulomcnally | Implementación completa. Plugin `enacal.nicaragua` v1.0.0 en `internal/plugins/enacal/nicaragua/nicaragua.go`: auth `POST /soe/api/account/authenticate` (`{username,password}` → `{"success":true}`), cookies del WAF F5 en jar de memoria (primer `GET /soe/`), facturas vía `GET /soe/api/InfoCuentas/facturas/?CuentaNro=<NIC>` con `Authorization: Basic`, normalización a `Bill` (period `YYYY-MM` de `Emision`, amount `TotalFac`, due_date de `Vence`, status de `Pendiente`/`Pagado_Por`, `Raw["numFactura"]` = `NoDoc`). `CredentialSchema` (username/password) para el formulario dinámico. Registro en `cmd/server/main.go`. Seed en `scripts/seed-apps.sh` (credenciales por entorno, nunca hardcodeadas). Cuenta real configurada en SQLite local (NIC `<NIC>`). Tests unitarios + test de webhook con `numFactura` (`{year, month, amount, status, invoice_number}`). Verificado end-to-end contra la API real: devolvió las 6 facturas reales (2026-08 C$28.19 `pending` → 2026-03 C$37.86 `paid`). `go build ./...`, `go vet ./...`, `go test ./...` y `npm run build` OK. Cero deps nuevas. Estado → `pending_release`. |
| 2026-09-12 | paulomcnally | **Release** (cierre por decisión del usuario; el deploy a iHost lo realiza él). El plugin quedó activo en el catálogo local (`enacal.nicaragua` 1.0.0) y la cuenta ENACAL con credenciales en SQLite local. Issue #13 cerrado con label `spec/released`. Commits: implementación + release (docs + tracker). |