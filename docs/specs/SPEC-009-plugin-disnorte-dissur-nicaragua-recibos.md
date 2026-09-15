---
title: "Plugin disnorte.dissur.nicaragua: autenticación y recibos"
id: "SPEC-009"
status: "released"
author: "paulomcnally"
created: "2026-09-12"
updated: "2026-09-12"
github_issue: 10
---

# Plugin disnorte.dissur.nicaragua: autenticación y recibos

**ID**: SPEC-009  
**Estado**: released  
**Autor**: paulomcnally  
**Creado**: 2026-09-12  
**Actualizado**: 2026-09-12

---

## 1. Resumen Ejecutivo

Se necesita un plugin de integración llamado `disnorte.dissur.nicaragua` que consulte la
**lista de recibos/facturas** de energía eléctrica de un cliente de **DISNORTE-DISSUR**
(Distribuidora de Electricidad del Norte y del Sur, S.A., Nicaragua) usando la API pública
que consume la app oficial (`https://<DOMINIO_DISNORTE>`). El usuario capturó con
Charles Proxy una sesión real de la app (iOS) en `/home/paulomcnally/disnorte_dissur.chlz`,
que incluye el tráfico de **autenticación** (`nis` + `pin` → id de cliente) y el tráfico de
**recibos/facturas** (lista paginada de recibos, y endpoint que resuelve la URL del TIF de
cada factura).

La API es un backend **ASP.NET (Microsoft-IIS/10.0)** detrás de un WAF de **Imperva**
(headers `X-CDN: Imperva`). El flujo es sencillo y **sin sesión/token persistente**: cada
request lleva parámetros en `application/x-www-form-urlencoded` y replica los headers de la
app móvil (UA de iPhone, `Origin: file://`). La autenticación (`POST /ovdev/api/cliente/autenticate/`
con `nis` y `pin`) devuelve el `Codigo`/id del cliente (ej. `"<CODIGO>"`) como JSON string;
ese id y el propio NIS se usan como `id` en los endpoints de consulta. Los recibos se obtienen
con `POST /ovdev/api//cliente/traerrecibopagina/` (pagina) usando el **NIS** como `id`.

Este plugin sigue el contrato de infraestructura de SPEC-003/SPEC-007 (interfaz `Plugin`,
registro, tabla `bills`, asociación `accounts.plugin_name`). **No requiere cambios de esquema.**
Reutiliza las credenciales JSON libres de la cuenta (`accounts.credentials`) para guardar
`pin` (y opcionalmente `nis`), nunca en código. El resultado esperado: plugin
`disnorte.dissur.nicaragua` registrado, `FetchBills` autentica con `nis`+`pin`, lista los
recibos paginados de los últimos meses, normaliza cada recibo a `[]plugins.Bill` con
periodo `YYYY-MM` (derivado de `FechaRecibo`), monto real, estado derivado de `Estado` y
`invoice_number` = `Factura`, y persiste en `bills`. El job/webhook existente (SPEC-004) reenvía
las facturas con el payload correcto (`year`, `month`, `amount`, `status`, `invoice_number`).

**Importante sobre credenciales**: el tráfico capturado expone el `nis` (`<NIS>`) y el
`pin` (`<PIN>`) del usuario. **No deben hardcodearse en el código.** Se usan únicamente para
poblar `accounts.credentials` en la base SQLite local de desarrollo (vía `seed-apps.sh` con
variables de entorno), y el plugin los lee de ahí en runtime. Nada se persiste ni se loguea.

## 2. Requerimientos

### 2.1 Requerimientos Funcionales (P0 - Obligatorios)

1. **REQ-001**: Plugin `disnorte.dissur.nicaragua` que implementa la interfaz `Plugin` existente (`Name`, `Version`, `Source`, `Description`, `FetchBills`), se registra en `main.go` y aparece en el catálogo `plugins` vía `SyncCatalog` (sin migración nueva).
2. **REQ-002**: Autenticación con credenciales de la cuenta: `POST https://<DOMINIO_DISNORTE>/ovdev/api/cliente/autenticate/` con `application/x-www-form-urlencoded` body `nis=<nis>&pin=<pin>`. La respuesta es un JSON string con el id del cliente (`Codigo`, ej. `"<CODIGO>"`). `FetchBills` lo obtiene y lo usa como `id` en los endpoints de consulta.
3. **REQ-003**: Lista de recibos paginada: `POST /ovdev/api//cliente/traerrecibopagina/` con `id=<NIS>&pagina=N` (N = 1, 2, ...). La respuesta es `Suminstro[0].Recibo[]` con `Factura`, `FechaRecibo` (`"2026-08-19T00:00:00"`), `TipoRecibo`, `Importe` (float, ej. `1662.11`), `ImporteTotalDeuda`, `Estado` (ej. `"Cobrado por cuenta bancaria"`). Iterar páginas hasta que una página devuelva un `Recibo[]` vacío.
4. **REQ-004**: Normalización a `[]plugins.Bill`:
   - `Period`: `YYYY-MM` derivado de `FechaRecibo` (ej. `"2026-08-19"` → `"2026-08"`).
   - `Amount`: `Importe` formateado sin ceros de más (ej. `"1662.11"`, `"60.66"`).
   - `DueDate`: `YYYY-MM-DD` de `FechaRecibo` (la API no expone vencimiento real en `Recibo`; el `FechaVencimiento` viene en `0001-01-01T00:00:00`).
   - `Status`: `paid` si `Estado` indica cobrado/pagado (ej. contiene `Cobrado`, `Pagado`, `Cancelado`); si no, `pending`.
   - `Raw`: debe incluir `numFactura` = `Factura` (clave leída por `buildWebhookPayload`) y `fechaRecibo` = `FechaRecibo`.
5. **REQ-005**: Headers obligatorios en todas las llamadas (replicando la app, verificados en la captura): `Content-Type: application/x-www-form-urlencoded`, `Accept: application/json, text/javascript, */*; q=0.01`, `Origin: file://`, `Sec-Fetch-*` (site `cross-site`, mode `cors`, dest `empty`), `User-Agent` iPhone (`Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148`), `Accept-Language: es-es`.
6. **REQ-006**: `invoice_number` en el webhook: al poner `Raw["numFactura"]`, el `buildWebhookPayload` existente (`webhook.go:241-245`) ya emite `invoice_number` = `Factura` (ej. `<NUM_RECIBO>`). Verificar que el payload final es correcto (`year`, `month`, `amount`, `status`, `invoice_number`).
7. **REQ-007**: Seed local para pruebas: `scripts/seed-apps.sh` soporta variables para crear una app + cuenta DISNORTE de ejemplo (`PLUGIN_NAME=disnorte.dissur.nicaragua`, `IDENTIFIERS=<NIS>`, `CREDENTIALS='{"pin":"..."}'`). Las credenciales reales se proveen vía entorno, nunca hardcodeadas.

### 2.2 Requerimientos Funcionales (P1 - Importantes)

1. **REQ-008**: Manejo de errores estructurado mapeado a los errores del dominio (`ErrAuthFailed`, `ErrUpstream`, `ErrAPISchemaChanged`, `ErrAuthExpired`), persistidos como `bills` con `status: error` y `plugin_version`, sin exponer credenciales.
2. **REQ-009**: El plugin debe funcionar cuando el `identifier` de la cuenta es el **NIS** (ej. `<NIS>`): el id de cliente obtenido en autenticación se usa para `traersuministro`/`traermedidor`; el NIS se usa para `traerrecibopagina`. Si `identifier` no es numérico NIS, el plugin deriva el NIS de la autenticación si la respuesta lo permite; caso contrario, error claro.
3. **REQ-010**: Resolución de la URL del recibo en TIF (informativo, para no romper el MVP): `POST /ovdev/api//factura/traerfacturaios/` con `id=<Factura>&nis=<NIS>&dia=<DD%2FMM%2FYYYY>` devuelve un JSON string con `<DOMINIO_DISNORTE>/_services/upload/<Factura>.tif`. No es obligatorio descargar el TIF en el MVP; se documenta el endpoint para la versión siguiente.

### 2.3 Requerimientos Funcionales (P2 - Deseables)

1. **REQ-011**: Descarga del TIF de cada factura (`GET /_services/upload/<Factura>.tif`, `image/tiff`) y guardado en `public/` o `data/` para adjuntar al webhook (si el receptor lo soporta).
2. **REQ-012**: Endpoint `GET /ovdev/api/cliente/traerconsumo/?id=<id>&peticion=HistoricoConsumo` (histórico de consumo mensual) como dato complementario opcional en el payload del webhook.

### 2.4 Requerimientos No Funcionales

- **Rendimiento**: Timeouts obligatorios (≤ 15 s) en llamadas HTTP salientes. Una consulta < 5 s.
- **Seguridad**: El `pin` y el `nis` jamás se exponen en respuestas de API ni en `raw`; viven solo en el JSON de credenciales de la cuenta y en memoria durante la consulta. No hardcodear credenciales en el código.
- **Almacenamiento**: Reusa la tabla `bills`; cero migraciones nuevas.
- **Disponibilidad**: Todas las rutas detrás de `authMiddleware`; `/health` sin cambios.
- **iHost**: Cero dependencias nuevas (Go stdlib `net/http`). El parsing de fechas usa `time` (stdlib).

## 3. Investigación y Decisiones Técnicas

### 3.1 Contexto investigado (captura Charles Proxy real, 2026-09-12)

Se capturó una sesión real de la app oficial de DISNORTE-DISSUR (iOS) desde `192.168.1.197`
mediante Charles Proxy (`/home/paulomcnally/disnorte_dissur.chlz`). Host: `<DOMINIO_DISNORTE>`
(IP `45.60.51.44`, WAF Imperva, TLS 1.3). Endpoints verificados en el orden de la app:

- **`POST /ovdev/api/cliente/autenticate/`** — body `nis=<NIS>&pin=<PIN>` →
  respuesta 200 `application/json`: **`"<CODIGO>"`** (JSON string = `Codigo`/id del cliente).
- **`POST /ovdev/api/cliente/traermedidor/`** — body `id=<CODIGO>&peticion=Medidor` →
  datos del medidor (`HEXING HXE12DL-3`, nº `24600326HE`).
- **`POST /ovdev/api//cliente/traersuministropagina/`** — body `id=<CODIGO>&pagina=1&peticion=Suministro`.
- **`POST /ovdev/api//cliente/traersuministro/`** — body `id=<CODIGO>&peticion=HistoricoRecibo`
  (y `peticion=Lectura`) → datos del suministro (NIS, ubicación, departamento, tipo de cuenta).
- **`POST /ovdev/api//cliente/traerrecibopagina/`** — body `id=<NIS>&pagina=1` (y `pagina=2`) →
  **lista de recibos**: `Suminstro[0].Recibo[]` con `Factura` (`<NUM_RECIBO>`),
  `FechaRecibo` (`2026-08-19T00:00:00`), `TipoRecibo` (`Recibos de energia electrica`),
  `Importe` (`60.66`, `1662.11`, ...), `ImporteTotalDeuda` (`0.0`), `Estado`
  (`Cobrado por cuenta bancaria`). Nótese el **doble slash** en la ruta (`/ovdev/api//cliente/...`).
- **`POST /ovdev/api//cliente/traerlecturamultiple/`** — body `id=<NIS>&pagina=1` → lecturas del medidor.
- **`GET /ovdev/api/cliente/traerconsumo/?id=<CODIGO>&peticion=HistoricoConsumo`** → histórico de consumo mensual.
- **`POST /ovdev/api//factura/traerfacturaios/`** — body `id=<NUM_RECIBO>&nis=<NIS>&dia=19%2F8%2F2026` →
  respuesta JSON string: `"<DOMINIO_DISNORTE>/_services/upload/<NUM_RECIBO>.tif"`.
- **`GET /_services/upload/<Factura>.tif`** → `image/tiff` (escaneo del recibo).

**Headers típicos de request** (todas): `Host: <DOMINIO_DISNORTE>`,
`Accept: application/json, text/javascript, */*; q=0.01`, `Sec-Fetch-Site: cross-site`,
`Accept-Language: es-es`, `Content-Type: application/x-www-form-urlencoded`,
`Origin: file://`, `User-Agent: Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148`,
`Connection: keep-alive`, cookies de Imperva (`incap_ses_*`, `visid_incap_*`).
Respuestas: `Server: Microsoft-IIS/10.0`, `X-AspNet-Version: 4.0.30319`,
`Access-Control-Allow-Origin: *`, `X-CDN: Imperva`, `Content-Encoding: gzip`.

**Datos reales del usuario** (para seed local vía entorno, NO en código):
- `nis` = `<NIS>` (identifier de la cuenta)
- `pin` = `<PIN>` (credencial → `accounts.credentials`)
- `Codigo`/id de cliente = `<CODIGO>` (derivado del autenticate)
- Medidor = `24600326HE`
- Recibos recientes: `<NUM_RECIBO>` (2026-08, C$60.66), `<NUM_RECIBO>` (2026-07, C$1662.11),
  `<NUM_RECIBO>` (2026-06, C$135.66), `<NUM_RECIBO>` (2026-05, C$1737.03), etc.

### 3.2 Opciones evaluadas

| Opción | Pros | Contras | Decisión |
|--------|------|---------|----------|
| Auth por `nis`+`pin` en `autenticate/` (sin token) | La app real lo hace; la respuesta da el id del cliente | No hay token de sesión persistente; el `pin` viaja en el body | ✅ Seleccionada (verificado en captura) |
| Auth por cookies de sesión (Imperva) | Las cookies del WAF existen en la captura | Son cookies del WAF/anti-bot, no de sesión de la API; cambian por dispositivo | ❌ Rechazada (no es el mecanismo de auth de la API) |
| Token manual / cabecera Authorization | Simple | La API no usa Authorization en ninguna llamada | ❌ Rechazada |
| Descargar el TIF en el MVP | Adjunto completo del recibo | Costo de descarga + almacenamiento; el receptor aún no lo soporta | ❌ Rechazada para el MVP (documentado en REQ-011) |
| Migración/schema nuevo para DISNORTE | ... | Reusa todo lo existente | ❌ Rechazada (cero migraciones) |

### 3.3 Decisiones arquitectónicas (ADRs)

**ADR-001**: Autenticación por `nis`+`pin` contra `autenticate/`, sin token de sesión
- **Contexto**: La captura muestra que cada request lleva parámetros `form-urlencoded` y que
  el `autenticate/` devuelve el id del cliente (`Codigo`). No hay `Authorization` ni cookie de sesión.
- **Decisión**: `FetchBills` llama `autenticate/` con `nis`+`pin`, obtiene `Codigo` y lo usa
  como `id` en los endpoints de consulta. Para `traerrecibopagina` se usa el **NIS** como `id`
  (tal como lo hace la app), no el `Codigo`.
- **Consecuencias**: El `pin` debe vivir en las credenciales de la cuenta (SQLite) y no en código.

**ADR-002**: Las credenciales (`pin`) viven en `accounts.credentials`
- **Contexto**: SPEC-002 ya guarda credenciales JSON por cuenta; el `pin` es la credencial de acceso.
- **Decisión**: `creds["pin"]` (obligatorio) + `creds["nis"]` (opcional, por defecto el `identifier`
  de la cuenta). Nunca hardcodear los valores reales; el seed los toma de variables de entorno.
- **Consecuencias**: El plugin lee las credenciales en runtime y no expone nada.

**ADR-003**: Cero migraciones nuevas; reuso total de `plugins`/`bills`/webhook
- **Contexto**: SPEC-003/SPEC-004/SPEC-007 dejaron catálogo, tabla `bills`, asociación `plugin_name`
  y scheduler/webhook con `parseBillPeriod` tolerante (`DD-MM-YYYY` y `YYYY-MM`, SPEC-008).
- **Decisión**: El plugin DISNORTE se registra en `main.go` y `SyncCatalog` lo persiste.
  Emite `Period` en `YYYY-MM` (compatible con `parseBillPeriod`).
- **Consecuencias**: Menos superficie de cambio y riesgo en iHost.

**ADR-004**: Cliente HTTP estándar (`net/http`), sin deps nuevas
- **Contexto**: La API acepta requests con TLS 1.3 estándar de Go (la app es HTTP/1.1 a
  diferencia de la de Tigo). El WAF Imperva responde `Access-Control-Allow-Origin: *` y no
  bloqueó las llamadas desde la IP residencial de la captura.
- **Decisión**: `net/http` estándar con Timeout 15 s y los headers de la app móvil.
- **Consecuencias**: Cero deps; menor tamaño de binario para iHost.

**ADR-005**: Periodo `YYYY-MM` derivado de `FechaRecibo`
- **Contexto**: La API de recibos no expone un periodo explícito (solo `FechaRecibo` y
  `FechaVencimiento` = `0001-01-01T00:00:00`). El mes de la factura coincide con el mes de emisión.
- **Decisión**: `Period = FechaRecibo[:7]` (ej. `2026-08`), `DueDate = FechaRecibo[:10]`.
- **Consecuencias**: Compatible con `parseBillPeriod` (SPEC-008) y el contrato del receptor.

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
                                                        [internal/plugins: Registry + disnorte.dissur.nicaragua]
                                                                                                 |
                                              POST /ovdev/api/cliente/autenticate/ (nis+pin)  --> <DOMINIO_DISNORTE> (Imperva)
                                                                                                 |
                                              POST /ovdev/api//cliente/traerrecibopagina/  -->  [WAF Imperva → IIS/ASP.NET]
                                                                                                 |
                                                                                                 v
                                                      [SQLite: plugins + accounts(plugin_name + credentials) + bills]
                                                                                                 |
                                            [Scheduler/Webhook existente (SPEC-004/008) reusa bills, sin cambios]
```

### 4.2 Componentes

#### 4.2.1 Plugin `disnorte.dissur.nicaragua` (`internal/plugins/disnorte/dissur/nicaragua/nicaragua.go`)
- **Responsabilidad**: Autenticar con `nis`+`pin`, listar recibos paginados y normalizarlos a `plugins.Bill`.
- **Interfaz**: Implementa `Plugin`. `FetchBills(ctx, creds, identifier)` con
  `creds = {"pin": "...", "nis": "..."}` e `identifier` = NIS (ej. `<NIS>`).
- **Dependencias**: Solo stdlib (`net/http`, `net/url`, `encoding/json`, `time`, `strings`).
- **Ubicación**: `internal/plugins/disnorte/dissur/nicaragua/nicaragua.go`; registro en `cmd/server/main.go`.

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
  - account.credentials  = {"pin": "<PIN>", "nis": "<NIS>"}   ← nis opcional (default identifier)
  - account.identifier   = NIS de la cuenta (ej. "<NIS>")
  - account.plugin_name  = "disnorte.dissur.nicaragua"

plugins / bills (SPEC-003, sin cambios)
  - disnorte.dissur.nicaragua se registra en runtime (SyncCatalog → UpsertPlugin)
  - bills: una fila por consulta con plugin_version + source (https://<DOMINIO_DISNORTE>)
```

### 4.4 APIs / Contratos

#### Endpoint: `POST /api/accounts/{accountId}/bills:fetch` (existente)

Ejecuta la consulta del plugin asociado a la cuenta. Para DISNORTE, el plugin:
1. `POST https://<DOMINIO_DISNORTE>/ovdev/api/cliente/autenticate/` con body
   `nis=<NIS>&pin=<PIN>` → `"<CODIGO>"` (id del cliente).
2. `POST https://<DOMINIO_DISNORTE>/ovdev/api//cliente/traerrecibopagina/` con body
   `id=<NIS>&pagina=N` (N = 1, 2, ... hasta página vacía) → `Suminstro[0].Recibo[]`.
3. Normaliza cada recibo a `plugins.Bill` (REQ-004) y persiste en `bills`.

**Response 201** (igual que SPEC-003/007):
```json
{ "id": 43, "account_id": 7, "plugin_name": "disnorte.dissur.nicaragua", "plugin_version": "1.0.0", "status": "ok", "bills": [{ "period": "2026-08", "amount": "60.66", "due_date": "2026-08-19", "status": "paid" }] }
```

**Response Error** (códigos): `invalid_request` (400), `not_found` (404), `auth_expired` (401),
`auth_failed` (401), `upstream_error` (502), `api_changed` (502).

#### Endpoint: `GET /api/accounts/{accountId}/bills` (existente)

**Response 200**: idéntico al de SPEC-003; cada fila trae `plugin_name: "disnorte.dissur.nicaragua"`,
`plugin_version`, `source` y `status`.

> El `pin`/`nis` NUNCA aparecen en respuestas ni en `raw`.

#### Payload enviado al webhook (correcto, sin cambios de contrato)

```json
{
  "year": 2026,
  "month": 8,
  "amount": 60.66,
  "status": "paid",
  "invoice_number": "<NUM_RECIBO>"
}
```

### 4.5 Dependencias

- **Internas**: `internal/plugins` (interfaz/registry), `internal/services` (`BillsService`), `internal/storage` (plugins/bills).
- **Externas**: **Ninguna nueva**. Solo stdlib.

## 5. Criterios de Aceptación

### 5.1 Funcionales

- [ ] CA-001: Dado un `GET /api/plugins` autenticado, aparece `disnorte.dissur.nicaragua` 1.0.0 con su `source` y descripción.
- [ ] CA-002: Dado un `PUT /api/accounts/{id}/plugin` con `disnorte.dissur.nicaragua`, la cuenta queda asociada.
- [ ] CA-003: Dado un `POST /api/accounts/{id}/bills:fetch` con `pin` válido en credenciales e `identifier` = NIS, el plugin autentica, pagina los recibos y persiste `bills` con `status: ok`, `plugin_version` y `source` poblados.
- [ ] CA-004: Cada recibo se normaliza con `Period: "YYYY-MM"` (de `FechaRecibo`), `Amount` = `Importe`, `DueDate` = `FechaRecibo[:10]` y `Status` derivado de `Estado`.
- [ ] CA-005: El `Raw` de cada bill incluye `numFactura` = `Factura`, y `buildWebhookPayload` emite `invoice_number` correcto (ej. `<NUM_RECIBO>`).
- [ ] CA-006: Dado un `pin` inválido, la consulta persiste `bills` con `status: error` y código `auth_failed` (sin exponer credenciales).
- [ ] CA-007: Las credenciales (`pin`/`nis`) NUNCA aparecen en respuestas de API ni en `raw`.
- [ ] CA-008: El seed local crea app + cuenta de ejemplo con la data real de DISNORTE (`identifier <NIS>`) usando credenciales de entorno.
- [ ] CA-DARK: N/A (no toca formularios frontend).
- [ ] CA-BACK: N/A (sin páginas nuevas).

### 5.2 No funcionales

- [ ] CA-NF-001: `go build ./...` compila sin errores.
- [ ] CA-NF-002: `go test ./internal/plugins/disnorte/...` y `go test ./internal/services/...` pasan.
- [ ] CA-NF-003: Sin dependencias nuevas en `go.mod`.
- [ ] CA-NF-004: Toda llamada saliente a DISNORTE tiene timeout ≤ 15 s y no bloquea otras peticiones.

### 5.3 Testing

- **Unit tests**: Parsing del autenticate (JSON string), parsing de `Recibo[]`, normalización a `Bill`
  (periodo/monto/estado), paginación (página vacía detiene), mapeo de errores (`auth_failed`, `api_changed`).
- **Integration tests**: Flujo con fixture: `autenticate` → `traerrecibopagina` (página 1 y 2) →
  `bills` persistido con `plugin_version` y `source`; y `buildWebhookPayload` con `Raw["numFactura"]`.
- **E2E tests**: Desde la UI, consultar facturas de una cuenta DISNORTE con `pin` real (local).
- **Carga/Performance**: Una consulta < 5 s; sin picos de RAM (sin deps, sin pools).

## 6. Plan de Implementación

### 6.1 Fases

| Fase | Descripción | Estimación | Dependencias |
|------|-------------|------------|--------------|
| 1 | Espec con datos reales de la captura (este documento) | 0.25 día | Ninguna |
| 2 | Plugin `disnorte.dissur.nicaragua` v1.0.0: autenticate (nis+pin), traerrecibopagina paginado, parsing de `Recibo[]`, normalización a `Bill`, errores de dominio | 1 día | Fase 1 |
| 3 | Registro en `main.go` + `SyncCatalog`; verificación de `buildWebhookPayload` con `numFactura` | 0.25 día | Fase 2 |
| 4 | Seed local de app+cuenta DISNORTE (credenciales por entorno, sin hardcodear) | 0.25 día | Fase 2 |
| 5 | Tests unitarios/integración + `go build`/`go test`/`npm run build` | 0.5 día | Fase 4 |
| 6 | Validación end-to-end local con el `pin` real del usuario | 0.5 día | Fase 5 |

### 6.2 Milestones

- **MVP**: Plugin `disnorte.dissur.nicaragua` v1.0.0 con auth `nis`+`pin`, lista de recibos
  paginada, normalización a `Bill`, seed local, webhook con `invoice_number` correcto.
- **V1.1** (opcional): Descarga del TIF (REQ-011) y consumo histórico (REQ-012).

## 7. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| WAF Imperva bloquea la IP del iHost (403/captcha) | Media | Alto | Las llamadas de la captura fueron aceptadas desde IP residencial; el WAF usa cookies anti-bot que se obtienen con una primera request. Mitigación: probar desde la red del usuario; no martillar la API (frecuencia moderada). |
| El `pin` cambia o se bloquea | Media | Medio | Error `auth_failed` claro en la UI; actualizar credenciales de la cuenta. |
| DISNORTE cambia el path/formato del API | Media | Alto | Versionamiento obligatorio (SPEC-003): bump + stamp en `bills`; error `api_changed` avisa al usuario. |
| El doble slash (`/ovdev/api//cliente/...`) cambia | Baja | Bajo | Se replica exactamente como en la captura; si cambia, se ajusta la constante. |
| La app exige cookies Imperva persistentes | Media | Bajo | Guardar cookies de la primera request en el cliente HTTP y reenviarlas (jar en memoria); sin persistencia en disco. |

## 8. Notas y Referencias

- API base: `https://<DOMINIO_DISNORTE>`
- Endpoint de auth: `POST /ovdev/api/cliente/autenticate/` (body `nis=<NIS>&pin=<PIN>` → JSON string `Codigo`)
- Endpoint de recibos: `POST /ovdev/api//cliente/traerrecibopagina/` (body `id=<NIS>&pagina=N` → `Suminstro[0].Recibo[]`)
- Endpoint de factura TIF: `POST /ovdev/api//factura/traerfacturaios/` (body `id=<Factura>&nis=<NIS>&dia=<DD%2FMM%2FYYYY>`) → `"<DOMINIO_DISNORTE>/_services/upload/<Factura>.tif"`
- Descarga TIF: `GET /_services/upload/<Factura>.tif` (`image/tiff`)
- Captura real (Charles, `.chlz`): `/home/paulomcnally/disnorte_dissur.chlz`
- Datos reales del usuario (solo seed local vía entorno): `nis` `<NIS>`, `pin` `<PIN>`, `Codigo` `<CODIGO>`
- Reutiliza: SPEC-002 (cuentas/credenciales), SPEC-003 (infraestructura de plugins/bills/endpoints), SPEC-004 (scheduler/webhook), SPEC-008 (`parseBillPeriod` tolerante)
- Repo: `https://github.com/paulomcnally/p40la-ihost-automation`

## 9. Historial de Cambios

| Fecha | Autor | Descripción |
|-------|-------|-------------|
| 2026-09-12 | paulomcnally | Creación inicial de la especificación basada en la captura real de Charles Proxy de la app oficial DISNORTE-DISSUR (iOS). Flujo confirmado: `autenticate` (nis+pin → Codigo), `traerrecibopagina` paginado (NIS como id), `traerfacturaios` (URL del TIF). Sin token de sesión. Cero migraciones, cero deps. Credenciales nunca en código (solo `accounts.credentials` en SQLite local vía seed por entorno). Webhook con `invoice_number` vía `Raw["numFactura"]`. |
| 2026-09-12 | paulomcnally | Estado → `pending_execution` → `in_progress`. Inicio de desarrollo. |
| 2026-09-12 | paulomcnally | Implementación completa. Plugin `disnorte.dissur.nicaragua` v1.0.0 en `internal/plugins/disnorte/dissur/nicaragua/nicaragua.go`: auth `autenticate` (nis+pin), recibos paginados `traerrecibopagina` (con jar de cookies del WAF Imperva), normalización a `Bill` (period `YYYY-MM` de `FechaRecibo`, amount `Importe`, due_date, status de `Estado`, `Raw["numFactura"]`). Registro en `cmd/server/main.go`. Seed en `scripts/seed-apps.sh` con defaults de DISNORTE (credenciales por entorno, nunca hardcodeadas). Tests unitarios + test de webhook con `numFactura`. Verificado end-to-end contra la API real: devolvió los recibos reales (2026-08 C$60.66 → 2026-04 C$739.52, todos `paid`). `go build ./...`, `go vet ./...`, `go test ./...` y `npm run build` OK. Estado → `pending_release`. |
| 2026-09-12 | paulomcnally | **Release** (cierre por decisión del usuario; el deploy a iHost lo realiza él). Servidor local verificado por el usuario ("funciona"). El plugin quedó activo en el catálogo local (`disnorte.dissur.nicaragua` 1.0.0) y la cuenta DISNORTE con credenciales en SQLite. Issue #10 cerrado con label `spec/released`. Commits: `b7a5c2c` (implementación), `62c76c7` (release: docs + tracker). |