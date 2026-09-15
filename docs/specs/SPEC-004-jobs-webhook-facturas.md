---
title: "Jobs diarios de facturas con envío a webhook"
id: "SPEC-004"
status: "released"
author: "paulomcnally"
created: "2026-09-11"
updated: "2026-09-11"
github_issue: null
---

# Jobs diarios de facturas con envío a webhook

**ID**: SPEC-004  
**Estado**: in_progress  
**Autor**: paulomcnally  
**Creado**: 2026-09-11  
**Actualizado**: 2026-09-11

---

## 1. Resumen Ejecutivo

Cuando una cuenta (SPEC-002) tiene asociado un plugin (SPEC-003) y se le configura una
URL de webhook, se necesita que un **job programado** ejecute cada día a una **hora
configurada** la consulta de facturas de esa cuenta y las envíe al webhook. El webhook
es el que expone `p40la-ihost` (documentado en `docs/webhooks-api.md`, proyecto hermano
en `http://localhost:8088`): `POST /webhooks/{uuid}` con header `X-Webhook-Key` y un
payload con `year`, `month`, `amount`, `invoice_number`, `status`, etc.

La **api_key es global** (una sola para todos los webhooks) y se configura en Settings.
Cada cuenta tiene su propia **URL de webhook** (que contiene el `uuid` del servicio) y su
**hora de ejecución** diaria. Esto permite automatizar la sincronización de facturas de
los servicios (ej. Claro Nicaragua) hacia la app p40la-ihost sin intervención manual.

El resultado esperado: una configuración global de webhook (api key + toggle maestro),
configuración por cuenta (URL, hora, habilitado), un scheduler diario liviano (goroutine
con ticker, bajo consumo de RAM, sin dependencias nuevas) y el cliente que adapta las
facturas obtenidas por el plugin al contrato del webhook.

## 2. Requerimientos

### 2.1 Requerimientos Funcionales (P0 - Obligatorios)

1. **REQ-001**: Settings global: campos `webhook_api_key` (clave global) y `webhook_enabled` (toggle maestro). Ambos editables desde Settings.
2. **REQ-002**: Configuración por cuenta: `webhook_url` (URL completa del webhook, ej. `http://localhost:8088/webhooks/{uuid}`), `schedule_time` (hora HH:MM), `schedule_enabled` (bool). Persistidos en la tabla `accounts`.
3. **REQ-003**: Scheduler diario: cuando una cuenta tiene plugin asociado + webhook_url + schedule_enabled, se ejecuta un job cada día a `schedule_time` que consulta las facturas (vía el plugin) y las envía al webhook.
4. **REQ-004**: Cliente de webhook: cada factura se adapta al contrato de `docs/webhooks-api.md` (`year`, `month`, `amount`, `invoice_number`, `status`) y se envía como `POST` a la `webhook_url` con header `X-Webhook-Key: <api_key global>`. Sin api_key configurada o con toggle OFF, el job no envía y lo registra.
5. **REQ-005**: Registro de entregas: tabla `webhook_logs` que guarda por cuenta la URL, el payload, el estado (`ok`/`error`) y la respuesta, para auditoría.
6. **REQ-006**: **Logs de eventos visibles**: cada ejecución del job (éxito o fallo de cada factura enviada) queda en `webhook_logs` con su `status` (`ok`/`error`), y la UI los muestra por cuenta con detalle (payload y respuesta del receptor).
7. **REQ-007**: Persistencia vía migración numerada `0004`: `ALTER accounts` (webhook_url, schedule_time, schedule_enabled, last_run_at) + `CREATE webhook_logs`.
8. **REQ-008**: Endpoints autenticados: `GET/PUT /api/settings/webhook`, `GET/PUT /api/accounts/{id}/webhook`, `GET /api/accounts/{id}/webhook/logs`, y `POST /api/accounts/{id}/webhook:test` para enviar las facturas actuales manualmente.

### 2.2 Requerimientos Funcionales (P1 - Importantes)

1. **REQ-009**: Validaciones: `schedule_time` en formato `HH:MM` válido; `webhook_url` URL válida (http/https); api_key con longitud mínima (recomendado 32+).
2. **REQ-010**: El job registra su resultado: `last_run_at` en la cuenta y una fila en `webhook_logs` por envío (éxito o error).
3. **REQ-011**: Frontend: sección "Webhooks" en Settings (toggle + campo api_key) y bloque de configuración por cuenta en la página de facturas (URL, hora, toggle habilitado, botón "Enviar ahora") con listado de los últimos logs de eventos (success/fallidos).

### 2.3 Requerimientos Funcionales (P2 - Deseables)

1. **REQ-012**: Regeneración de api_key (genera una nueva y rompe la anterior).
2. **REQ-013**: El detalle de un log de evento se puede abrir para ver el payload exacto enviado y la respuesta del receptor.

### 2.4 Requerimientos Funcionales (P0 - Frecuencia flexible del job)

1. **REQ-014**: **Frecuencia configurable por cuenta** además de la hora: `daily` (actual), `every_n_days` (cada N días, ej. 5), `weekly` (día(s) de semana, ej. Lun/Mié) y `monthly` (día del mes, ej. 15). La hora `HH:MM` se mantiene. Permite evitar consultas diarias al proveedor para no ser auditado/bloqueado.
2. **REQ-015**: El scheduler decide si una cuenta está "debida" combinando hora + frecuencia + `last_run_at`: diario = una vez por día; cada N días = solo si pasaron ≥ N días desde la última ejecución; semanal = solo en los días de semana configurados (una vez por día); mensual = solo el día del mes configurado (una vez al día).
3. **REQ-016**: Migración `0006`: columnas `schedule_frequency` (TEXT default `daily`), `schedule_interval` (INTEGER, para cada N días) y `schedule_days` (TEXT, para semanal/mensual).

### 2.5 Requerimientos No Funcionales

- **Rendimiento**: El scheduler usa un `time.Ticker` (ej. cada 30s) que consulta cuentas pendientes; sin goroutines por cuenta persistente. Consumo mínimo en iHost.
- **Seguridad**: La api_key nunca se expone en listados; el payload enviado no incluye credenciales. Toda la configuración detrás de `authMiddleware`.
- **Almacenamiento**: `webhook_logs` crece con cada envío (una fila por factura por ejecución); `payload` es el JSON enviado. La UI lista las últimas N.
- **Disponibilidad**: Si el servidor se reinicia, el scheduler se rearma; no se pierden configuraciones (persistidas).
- **iHost**: **Cero dependencias nuevas** (Go stdlib `net/http` para el cliente). Sin librerías de cron externas; lógica de "minuto exacto" con ticker.

## 3. Investigación y Decisiones Técnicas

### 3.1 Contexto investigado

- Contrato del webhook (proyecto hermano `p40la-ihost`, `docs/webhooks-api.md`): `POST /webhooks/{uuid}`, header `X-Webhook-Key` (api_key global hex 64), body `{year, month, amount, invoice_number, status, paid_at, payment_reference, drive_url}`. Upsert por `(service_id via uuid, year, month)`. Respuestas: 200 `{bill, created}`, 400/401/403/404.
- SPEC-003 ya implementa `FetchBills` que devuelve `[]plugins.Bill` con `Period`, `Amount`, `DueDate`, `Status`, `Raw`. Los datos crudos de Claro traen `numFactura` (invoice_number), `fechaEmision` (DD-MM-YYYY), `balanceRestante` (monto), `fechaVencimiento`.
- Settings existentes: tabla `settings` (key/value) con `language`. Se reutiliza el patrón para `webhook_api_key` y `webhook_enabled`.
- No existe scheduler/cron en el repo; se introduce una goroutine con `time.Ticker` (stdlib).
- El repo **no** tiene cliente HTTP saliente configurable más allá del plugin; el cliente de webhook será nuevo pero con stdlib.

### 3.2 Opciones evaluadas

| Opción | Pros | Contras | Decisión |
|--------|------|---------|----------|
| Librería cron (robfig/cron) | Sintaxis de cron completa | Dependencia nueva, más RAM | ❌ Rechazada |
| Goroutine con `time.Ticker` + match de `HH:MM` | Cero deps, mínimo, suficiente (jobs diarios a hora fija) | No soporta cron expresivo | ✅ Seleccionada |
| Webhook config en tabla nueva `account_webhooks` | Aislado | Tabla extra; la cuenta ya guarda su config | ❌ Rechazada (se usa ALTER accounts) |
| Logs de entrega en `bills` | Reutiliza tabla | Mezcla conceptos (consulta vs entrega) | ❌ Rechazada |
| Tabla `webhook_logs` | Auditoría clara por envío | Tabla nueva | ✅ Seleccionada |
| Enviar `status` siempre | Refleja pendiente/pagada | Para `paid` el doc usa `paid_at` default "now" | ✅ Seleccionada (se omite `paid_at`) |

### 3.3 Decisiones arquitectónicas (ADRs)

**ADR-001**: Scheduler con `time.Ticker` y match de minuto (sin cron externo)
- **Contexto**: iHost impone dependencias y memoria mínimas; el requisito es "cada día a una hora".
- **Decisión**: `Scheduler` con `time.Ticker` (30s) que consulta cuentas con `schedule_enabled=1`, `schedule_time` = hora:minuto actual, plugin y webhook configurados, y ejecuta el job.
- **Consecuencias**: Simple y barato; si el server no está activo a la hora exacta el job no se ejecuta ese día (aceptable para el caso de uso; documentado como limitación).

**ADR-002**: La api_key es global en `settings`; la URL del webhook por cuenta
- **Contexto**: El contrato de p40la-ihost define api_key global + `{uuid}` por servicio (contenido en la URL).
- **Decisión**: `webhook_api_key` + `webhook_enabled` como settings globales; cada cuenta guarda `webhook_url` (con su uuid) + `schedule_time` + `schedule_enabled`.
- **Consecuencias**: Coincide con el contrato; regenerar api_key o cambiar uuid solo toca un campo.

**ADR-003**: Cliente de webhook con stdlib que adapta `plugins.Bill` al payload
- **Contexto**: El job obtiene `[]plugins.Bill` (campos genéricos) y debe mandarlos con el esquema exacto del webhook.
- **Decisión**: `WebhookService.SendBill` mapea `{year, month (de Period), amount (Amount), invoice_number (Raw.numFactura), status}` y hace POST con `X-Webhook-Key`.
- **Consecuencias**: La adaptación queda centralizada; si cambia el contrato se toca solo el mapeo.

## 4. Diseño Técnico

### 4.1 Diagrama de arquitectura

```
[Settings: api_key global + toggle]    [account: plugin + webhook_url + schedule_time + enabled]
        |                                          |
        v                                          v
[Scheduler (goroutine, ticker 30s)] --> [WebhookService.RunJob(accountID)]
                                              |  FetchBills (plugin SPEC-003)
                                              |  adaptar []Bill → payload webhook
                                              v
                                    POST webhook_url  (X-Webhook-Key)
                                              |
                                              v
                                    [p40la-ihost :8088/webhooks/{uuid}]
                                              |
                                              v
                                      [SQLite: webhook_logs]
```

### 4.2 Componentes

#### 4.2.1 Scheduler (`internal/services/scheduler.go`)
- **Responsabilidad**: Cada tick (30s) detecta cuentas que deben ejecutar el job diario (hora:minuto actual) y las procesa.
- **Interfaz**: `NewScheduler(webhook *WebhookService, storage *storage.PluginsStorage)`, `Start(ctx)` / `Stop()`.
- **Dependencias**: `time.Ticker`, storage, WebhookService.
- **Ubicación**: `internal/services/scheduler.go`.

#### 4.2.2 WebhookService (`internal/services/webhook.go`)
- **Responsabilidad**: Gestión de api_key global, configuración por cuenta, cliente HTTP de envío y registro en `webhook_logs`.
- **Interfaz**: `GetWebhookConfig`, `SetWebhookConfig`, `GetAccountWebhook`, `SetAccountWebhook`, `RunJob(ctx, accountID)`, `SendBills(ctx, accountID, force bool)`.
- **Dependencias**: stdlib (`net/http`), `services.BillsService` (para `FetchBills`), storage.
- **Ubicación**: `internal/services/webhook.go`.

#### 4.2.3 Storage (`internal/storage/plugins.go` + nuevo)
- **Responsabilidad**: Persistir configuración webhook por cuenta y logs de entrega.
- **Interfaz**: `SetAccountWebhook`, `ListDueWebhookAccounts`, `SetLastRun`, `CreateWebhookLog`, `ListWebhookLogs`.
- **Ubicación**: `internal/storage/plugins.go` (se extiende).

#### 4.2.4 Handlers (`internal/api/webhook_handlers.go`)
- **Responsabilidad**: Endpoints de configuración y test manual.
- **Interfaz**: `GET/PUT /api/settings/webhook`, `GET/PUT /api/accounts/{id}/webhook`, `POST /api/accounts/{id}/webhook:test`.
- **Ubicación**: `internal/api/webhook_handlers.go`, registro en `routes.go`, wiring en `main.go`.

#### 4.2.5 Frontend
- **Responsabilidad**: Sección "Webhooks" en Settings (toggle + api_key) y bloque por cuenta en `AccountBillsPage` (URL, hora, toggle, botón "Enviar ahora").
- **Ubicación**: `SettingsPage.tsx`, `AccountBillsPage.tsx`, `src/api/index.ts`, i18n es/en.

### 4.3 Modelo de datos

```
settings (existente, se reutiliza)
- webhook_api_key: TEXT (clave global)
- webhook_enabled: TEXT 'true'/'false'

accounts (ALTER, migración 0004 + 0006)
- ADD webhook_url: TEXT NULL (URL completa, incluye uuid)
- ADD schedule_time: TEXT NULL (HH:MM)
- ADD schedule_enabled: INTEGER NOT NULL DEFAULT 0
- ADD last_run_at: DATETIME NULL
- ADD schedule_frequency: TEXT NOT NULL DEFAULT 'daily' ('daily'|'every_n_days'|'weekly'|'monthly')
- ADD schedule_interval: INTEGER NULL (N días para 'every_n_days')
- ADD schedule_days: TEXT NULL (días de semana 0-6 para 'weekly', día 1-31 para 'monthly')

webhook_logs (nueva, migración 0004)
- id: INTEGER PK AUTOINCREMENT
- account_id: INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE
- webhook_url: TEXT NOT NULL
- status: TEXT NOT NULL ('ok' | 'error')
- payload: TEXT NOT NULL (JSON enviado)
- response: TEXT NOT NULL (respuesta del receptor o detalle del error)
- ran_at: DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
- Índice en account_id
```

Migración: `migrations/0004_webhooks.{up,down}.sql`.

### 4.4 APIs / Contratos

#### Endpoint: `GET /api/settings/webhook`

**Response 200**:
```json
{ "api_key": "7381...", "enabled": true }
```

#### Endpoint: `PUT /api/settings/webhook`

**Request**:
```json
{ "api_key": "7381d6dc...", "enabled": true }
```

**Response 200**:
```json
{ "api_key": "7381d6dc...", "enabled": true }
```

#### Endpoint: `GET /api/accounts/{accountId}/webhook`

**Response 200**:
```json
{ "webhook_url": "http://localhost:8088/webhooks/fd28...", "schedule_time": "06:00", "schedule_enabled": true, "last_run_at": null }
```

#### Endpoint: `PUT /api/accounts/{accountId}/webhook`

**Request**:
```json
{ "webhook_url": "http://localhost:8088/webhooks/fd28...", "schedule_time": "06:00", "schedule_enabled": true }
```

**Response 200**: mismo shape que GET.

#### Endpoint: `GET /api/accounts/{accountId}/webhook/logs`

**Response 200** (últimas N, por defecto 20):
```json
[
  { "id": 1, "account_id": 7, "webhook_url": "http://localhost:8088/webhooks/fd28...", "status": "ok", "payload": "{\"year\":2026,...}", "response": "{\"created\":true,\"bill\":{...}}", "ran_at": "2026-09-11T06:00:05Z" }
]
```

#### Endpoint: `POST /api/accounts/{accountId}/webhook:test`

Ejecuta `RunJob` de inmediato (consulta facturas y envía). Si no hay api_key global o toggle OFF, responde `invalid_request` (400).

**Response 200**:
```json
{ "delivered": 3, "failed": 0 }
```

**Response Error** (códigos): `invalid_request` (400, URL/hora/api_key inválidos o webhooks deshabilitados), `not_found` (404), `unauthorized` (401).

### 4.5 Dependencias

- **Internas**: `internal/api` (authMiddleware), `internal/services` (`BillsService` para `FetchBills`), `internal/storage`.
- **Externas**: **Ninguna nueva**. `net/http`, `time`, `encoding/json`, `strconv`, `strings` del stdlib. Frontend: sin nuevas dependencias npm.

## 5. Criterios de Aceptación

### 5.1 Funcionales

- [ ] CA-001: Dado un PUT `/api/settings/webhook` con `api_key` y `enabled`, el GET lo devuelve; sin sesión responde `401`.
- [ ] CA-002: Dado un PUT `/api/accounts/{id}/webhook` con URL, hora `HH:MM` y enabled, el GET lo devuelve; con hora inválida responde `invalid_request` (400).
- [ ] CA-003: Dado un account con plugin + webhook_url + schedule_enabled, el scheduler lo ejecuta al minuto correspondiente de `schedule_time` y registra `last_run_at`.
- [ ] CA-004: Dado un POST `/api/accounts/{id}/webhook:test`, las facturas actuales del plugin se envían al webhook con header `X-Webhook-Key` correcto y el payload cumple el contrato (`year`, `month`, `amount`, `invoice_number`, `status`).
- [ ] CA-005: Cada envío (ok o error) queda en `webhook_logs` con su payload y respuesta.
- [ ] CA-005B: Dado un GET `/api/accounts/{id}/webhook/logs`, se listan las entregas con su `status` (`ok`/`error`) y se muestran en la UI con detalle de payload/respuesta.
- [ ] CA-006: Con toggle `webhook_enabled=false` o sin `api_key`, el job no envía y registra el error.
- [ ] CA-007: La api_key y las URLs configuradas no exponen credenciales de cuentas.
- [ ] CA-DARK: Los inputs del formulario de webhook usan tokens del tema (`bg-card`, `text-text`, `text-text-secondary`) y se verificó legibilidad en darkmode.
- [ ] CA-BACK: La configuración de webhook vive dentro de la página de facturas de la cuenta; la flecha atrás del header funciona vía `BACK_ROUTES` (sin links "← Título").

### 5.2 No funcionales

- [ ] CA-NF-001: `go build ./...` compila sin errores y sin nuevas dependencias Go.
- [ ] CA-NF-002: `npm run build` typecheckea y genera el bundle; i18n es/en sincronizado.
- [ ] CA-NF-003: El scheduler no consume recursos cuando no hay cuentas configuradas (no bloquea, no goroutines persistentes por cuenta).

### 5.3 Testing

- **Unit tests**: Parseo de `HH:MM`, mapeo `plugins.Bill` → payload webhook, detección de cuentas debidas por hora.
- **Integration tests**: Configurar settings + cuenta, ejecutar `RunJob`, verificar `webhook_logs` y el POST recibido (mock del receptor).
- **E2E tests**: Desde Settings configurar api_key; desde la página de facturas configurar URL/hora y disparar "Enviar ahora".
- **Carga/Performance**: Una cuenta con 5 facturas → < 5s de ejecución; scheduler idle sin impacto.

## 6. Plan de Implementación

### 6.1 Fases

| Fase | Descripción | Estimación | Dependencias |
|------|-------------|------------|--------------|
| 1 | Migración `0004` (ALTER accounts + webhook_logs) | 0.25 día | Ninguna |
| 2 | Settings webhook (api_key + toggle) en servicio + storage + handlers | 0.25 día | Fase 1 |
| 3 | Config webhook por cuenta + storage (SetAccountWebhook, ListDue, CreateLog) | 0.5 día | Fase 1 |
| 4 | WebhookService: mapeo bill→payload, cliente HTTP, RunJob, SendBills | 0.5 día | Fase 2, 3 |
| 5 | Scheduler (ticker) + wiring en main.go | 0.25 día | Fase 4 |
| 6 | Handlers + rutas + endpoint :test | 0.25 día | Fase 4 |
| 7 | Frontend: Settings webhooks + bloque por cuenta + i18n + darkmode | 0.5 día | Fase 6 |
| 8 | Verificación (`go build`, `npm run build`), test end-to-end con webhook real | 0.25 día | Fase 7 |

### 6.2 Milestones

- **MVP**: Config global + por cuenta, scheduler diario, envío al webhook con `webhook_logs` y botón "Enviar ahora".
- **V1.0**: Historial de entregas visible y regeneración de api_key (P2).

## 7. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| El job no se ejecuta si el server no está activo a la hora exacta | Media | Bajo | Documentar; el botón "Enviar ahora" permite recuperar manualmente. |
| El receptor del webhook cambia el contrato | Media | Alto | Mapeo centralizado en `WebhookService` (ADR-003); si cambia, se toca solo ahí. |
| api_key expuesta en logs | Baja | Alto | Nunca se loguea la api_key; solo la URL y el payload. |
| Envíos duplicados si se re-ejecuta | Baja | Medio | El webhook es idempotente (upsert por servicio/año/mes, doc §5). |
| Scheduler con hora inválida provoca error | Baja | Bajo | Validación `HH:MM` en el PUT (REQ-008). |

## 8. Notas y Referencias

- Contrato del webhook: `docs/webhooks-api.md` (proyecto `p40la-ihost`, puerto 8088).
- Depende de SPEC-003 (plugin `claro.nicaragua` + `FetchBills`) y SPEC-002 (cuentas con credenciales).
- Patrón de módulos: `AGENTS.md` (migración → modelo → storage → servicio → handlers → frontend).
- Repo: `https://github.com/paulomcnally/p40la-ihost-automation`

## 9. Historial de Cambios

| Fecha | Autor | Descripción |
|-------|-------|-------------|
| 2026-09-11 | paulomcnally | Creación inicial de la spec: jobs diarios de facturas con envío a webhook (api_key global en settings, webhook_url + hora por cuenta, scheduler con ticker, webhook_logs). |
| 2026-09-11 | paulomcnally | Se refuerza el requisito de **logs de eventos visibles** (success/fallidos) por cuenta con endpoint `GET /api/accounts/{id}/webhook/logs`. |
| 2026-09-11 | paulomcnally | Estado a `in_progress`: inicio del desarrollo (migración 0004, settings webhook, configuración por cuenta, scheduler, cliente webhook, logs, frontend). |
| 2026-09-11 | paulomcnally | Implementación completa verificada en vivo: migración 0004, settings global (api_key + toggle), config por cuenta (URL + hora + enabled), scheduler (ticker 30s) que ejecutó el job a la hora configurada, envío de 5 facturas al webhook con el contrato (`year/month/amount/invoice_number/status`) + `X-Webhook-Key`, y logs de eventos success/fallidos visibles por cuenta. |
| 2026-09-11 | paulomcnally | Cambio iterativo: los logs de eventos de webhook registran y muestran el **código HTTP** de la respuesta del receptor (migración 0005, columna `http_status`), mostrando ej. 200/201 success o 4xx/5xx fallido en la UI. |
| 2026-09-11 | paulomcnally | Frecuencia flexible del job: `daily`, `every_n_days` (cada N días), `weekly` (días de semana) y `monthly` (día del mes), manteniendo la hora. Migración 0006 + lógica de "debido" en el scheduler. |
| 2026-09-11 | paulomcnally | La página de facturas se reorganiza en **tabs Histórico/Configuración** con **paginación** (hoy siempre visible + Más antiguos/recientes). Los filtros (tab, offset) se reflejan en la URL para compartir, y el label **"Última ejecución"** navega a esa fecha/hora (endpoint soporta `?at=<timestamp>`). |
| 2026-09-11 | paulomcnally | Cambio iterativo: los logs de eventos se muestran en el tab Histórico (con paginación) y el toggle de webhook guarda al instante. |
| 2026-09-11 | paulomcnally | **Released**: implementación completa (settings webhook, frecuencia flexible daily/every_n_days/weekly/monthly, scheduler, cliente webhook, logs). Commit `0629c10`. |