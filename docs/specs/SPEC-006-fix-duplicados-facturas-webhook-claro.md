---
title: "Fix duplicados de facturas en webhook Claro Nicaragua"
id: "SPEC-006"
status: "cancelled"
author: "paulomcnally"
created: "2026-09-12"
updated: "2026-09-12"
github_issue: 6
---

# Fix duplicados de facturas en webhook Claro Nicaragua

**ID**: SPEC-006  
**Estado**: cancelled  
**Autor**: paulomcnally  
**Creado**: 2026-09-12  
**Actualizado**: 2026-09-12

---

## 1. Resumen Ejecutivo

El job diario de facturas (SPEC-004) envía las facturas de Claro Nicaragua al webhook
de `p40la-ihost` (`POST /webhooks/{uuid}`). Algunos periodos responden
`400 {"error":"invalid_body","message":"crear factura: insertar factura: constraint failed: UNIQUE constraint failed: bills.service_id, bills.year, bills.month (2067)"}`,
bloqueando la sincronización de esos meses de forma permanente.

La investigación reproduce el fallo en vivo contra el webhook real y demuestra que la
causa **no está en el emisor** (este repo): el emisor envía payloads correctos y registra
el 400 tal cual. La causa está en el **receptor `p40la-ihost`**: cuando una factura del
periodo fue **borrada lógicamente** (soft-delete vía `DELETE /api/bills/{id}`, que setea
`deleted_at`), la restricción `UNIQUE(service_id, year, month)` a nivel de tabla sigue
cubriendo esa fila, pero `FindByServicePeriod` filtra `deleted_at IS NULL`, así que el
webhook cree que la factura no existe e intenta `INSERT`, chocando con el UNIQUE.

El resultado esperado: el webhook de `p40la-ihost` es **resiliente al soft-delete**, de
modo que reenviar un periodo cuya factura fue borrada lógicamente vuelva a crear/
reactivar la factura sin errores, y la integración vuelva a ser idempotente (documentada
en `docs/webhooks-api.md` §"Idempotencia").

## 2. Requerimientos

### 2.1 Requerimientos Funcionales (P0 - Obligatorios)

1. **REQ-001**: El webhook `POST /webhooks/{uuid}` debe soportar el caso en que exista una factura **soft-deleted** para `(service_id, year, month)`: en lugar de fallar con 400 por UNIQUE, debe reactivar/actualizar esa factura (o crear una nueva si es correcto) y responder `200` con `created: true|false`.
2. **REQ-002**: Si se opta por reactivar la fila soft-deleted, la factura resultante conserva el `id` original, se limpia `deleted_at`, y se actualizan los campos descriptivos del payload (`amount`, `invoice_number`, `status`, `drive_url`); si `status=paid`, se aplica la lógica de pago existente (`paid_at`, `payment_reference`).
3. **REQ-003**: La corrección no debe romper la idempotencia para facturas no borradas: reenviar un periodo existente sigue haciendo `UPDATE` (comportamiento actual, `created: false`).

### 2.2 Requerimientos Funcionales (P1 - Importantes)

1. **REQ-004**: Evaluar y, si aplica, migrar el esquema de `bills` a un índice UNIQUE **parcial** (`CREATE UNIQUE INDEX ... ON bills(service_id, year, month) WHERE deleted_at IS NULL`) como fix estructural, manteniendo la compatibilidad con el esquema actual.
2. **REQ-005**: Documentar el comportamiento en `docs/webhooks-api.md` del proyecto receptor (sección de idempotencia y soft-delete).

### 2.3 Requerimientos Funcionales (P2 - Deseables)

1. **REQ-006**: En este repo (`p40la-ihost-automation`), mejorar el log/visibilidad del error 400 en `webhook_logs` para que un fallo por `UNIQUE constraint` se distinga claramente de otros errores (p. ej. incluir el periodo en la respuesta registrada, ya disponible en el payload).

### 2.4 Requerimientos No Funcionales

- **Rendimiento**: El fix agrega a lo sumo una query adicional solo en el caso soft-deleted; sin impacto en iHost.
- **Seguridad**: Sin cambios en autenticación; la api_key global y el contrato `X-Webhook-Key` se mantienen.
- **Almacenamiento**: Sin tablas nuevas obligatorias; si se adopta REQ-004, una migración SQLite numerada en `p40la-ihost`.
- **Disponibilidad**: Reenviar un periodo fallido ya no queda bloqueado permanentemente; se resuelve con la próxima ejecución del job o el botón "Enviar ahora".
- **iHost**: Cero dependencias nuevas; solo lógica en Go (servicio/almacenamiento del receptor) y SQL.

## 3. Investigación y Decisiones Técnicas

### 3.1 Contexto investigado

- **Reproducción en vivo** contra el webhook real (`http://192.168.1.191:8088/webhooks/bf05bab4-6421-42c5-b9c3-449dc6c1b421`):
  - `year 2026 month 8` → `400 UNIQUE constraint ... (2067)`.
  - `year 2026 month 9` (nuevo) → `400` también.
  - `year 2025 month 1` → `200` creada (`id 31`, `service_id 3`).
  - `year 2025 month 2` → `200` creada (`id 32`) → **servicio mensual**, el mes no se normaliza a 0.
  - Reenviar `2025 month 1` → `200 created: false` (UPDATE correcto) → la idempotencia normal funciona.
  - `year 2026 months 1, 3, 10, 11, 12` (nunca enviados) → `200` creados.
  - `year 2026 months 4-7` (existentes) → `200 created: false` (UPDATE).
  - **Conclusión**: solo fallan los periodos `2026/08` y `2026/09` → son exactamente los que tienen una fila **soft-deleted** en `bills` del receptor.
- **El número `(2067)`** en el mensaje es el código extendido de SQLite `SQLITE_CONSTRAINT_UNIQUE` (2067), NO un año.
- Código receptor (`../p40la-ihost`):
  - `internal/services/webhook.go:126` → `FindByServicePeriod(service.ID, year, month)`; si `nil` → `Create` (`:145`) → `crear factura: insertar factura: ...`.
  - `internal/storage/bill.go:51-59` → `FindByServicePeriod` filtra `AND deleted_at IS NULL`.
  - `internal/storage/bill.go:165-175` → `SoftDelete` setea `deleted_at = CURRENT_TIMESTAMP`; NO elimina físicamente.
  - `internal/api/bill_handlers.go:126` → `DELETE /api/bills/{id}` → `SoftDelete`.
  - `migrations/0005_create_bills.up.sql` → `UNIQUE(service_id, year, month)` a nivel de tabla → cubre también filas soft-deleted.
- Código emisor (este repo): `internal/services/webhook.go:225` `buildWebhookPayload` genera `{year, month, amount, status, invoice_number}` correcto; `send` (`:199`) registra el 400 tal cual en `webhook_logs`.

### 3.2 Opciones evaluadas

| Opción | Pros | Contras | Decisión |
|--------|------|---------|----------|
| En el receptor: ante UNIQUE, buscar la fila soft-deleted por `(service_id, year, month)` sin filtro de `deleted_at`, reactivarla y actualizarla | Fix localizado, sin cambios de esquema; conserva `id` e historial | Requiere manejo del error 2067/`sqlite3.ErrConstraintUnique` | ✅ Seleccionada |
| Índice UNIQUE parcial `WHERE deleted_at IS NULL` en `bills` | Fix estructural: soft-delete ya no bloquea ningún insert | Requiere migración que recrea la tabla (SQLite no permite alterar UNIQUE); riesgo de duplicados si hay filas soft-deleted del mismo periodo | 🟡 Recomendada como refuerzo (REQ-004) |
| En el emisor: no reenviar periodos que antes fallaron | Evita el error | Pierde la sincronización de meses; el problema queda oculto | ❌ Rechazada |
| En el emisor: detectar 400 y marcarlo | Mejora visibilidad | No resuelve la sincronización | 🟡 Parcial (REQ-006) |

### 3.3 Decisiones arquitectónicas (ADRs)

**ADR-001**: El fix principal va en el receptor (`p40la-ihost`), no en el emisor
- **Contexto**: La reproducción demuestra que el payload del emisor es válido y que el fallo es un conflicto de UNIQUE en el receptor por filas soft-deleted.
- **Decisión**: Corregir `WebhookService.UpsertBill` (y/o `BillStorage`) en `p40la-ihost` para que, cuando `Create` falle por `SQLITE_CONSTRAINT_UNIQUE`, recupere la fila soft-deleted y la reactive/actualice.
- **Consecuencias**: La integración vuelve a ser idempotente incluso tras borrar facturas desde la UI de `p40la-ihost`. Este repo solo mejora el registro del error (REQ-006).

**ADR-002**: El número `(2067)` es el código de error de SQLite, no un año
- **Contexto**: El mensaje de error confundía al interpretar `2067` como un año del periodo.
- **Decisión**: Se trata de `SQLITE_CONSTRAINT_UNIQUE = 2067`; el periodo real es el enviado en el payload (2026/08, 2026/09).
- **Consecuencias**: Facilita detectar el caso en el código receptor (`errors.Is(err, sqlite3.ErrConstraintUnique)` o `strings.Contains(err.Error(), "constraint failed")`).

## 4. Diseño Técnico

### 4.1 Diagrama de arquitectura

```
[p40la-ihost-automation: job SPEC-004]
        |  POST /webhooks/{uuid}  (X-Webhook-Key)
        v
[p40la-ihost: WebhookHandlers.UpsertBill]
        |  WebhookService.UpsertBill
        v
[bills.FindByServicePeriod(service, year, month)]  -- deleted_at IS NULL
        |
        ├─ existe → UPDATE (ok, created:false)          [ya funciona]
        └─ nil → Create (INSERT)
                 |
                 └─ UNIQUE (2067) → ¿fila soft-deleted?
                         |
                         ├─ sí → reactivar + UPDATE (fix REQ-001)
                         └─ no → error real (propagar)
```

### 4.2 Componentes

#### 4.2.1 WebhookService (`../p40la-ihost/internal/services/webhook.go`)
- **Responsabilidad**: Orquestar el upsert del webhook.
- **Interfaz**: `UpsertBill(ctx, service, payload)` (existe).
- **Cambio**: Detectar UNIQUE en `Create` y delegar a recuperación de soft-deleted.
- **Ubicación**: `../p40la-ihost/internal/services/webhook.go`.

#### 4.2.2 BillStorage (`../p40la-ihost/internal/storage/bill.go`)
- **Responsabilidad**: Acceso a `bills`.
- **Cambio**: Nuevo método `FindByServicePeriodIncludingDeleted` (sin filtro `deleted_at`) y/o `Reactivate(id)` (limpia `deleted_at`).
- **Ubicación**: `../p40la-ihost/internal/storage/bill.go`.

#### 4.2.3 Sender (`p40la-ihost-automation/internal/services/webhook.go`)
- **Responsabilidad**: Enviar payloads y loguear entregas.
- **Cambio (REQ-006)**: Opcional: etiquetar en `webhook_logs` la causa `UNIQUE constraint` para diagnóstico.
- **Ubicación**: `internal/services/webhook.go`.

### 4.3 Modelo de datos

```
bills (receptor, sin cambios obligatorios si se adopta solo REQ-001)
- UNIQUE(service_id, year, month)  ← cubre filas soft-deleted (causa del bug)
- deleted_at: DATETIME NULL        ← soft-delete de la UI

Opcional (REQ-004): índice UNIQUE parcial
- CREATE UNIQUE INDEX idx_bills_service_year_month
  ON bills(service_id, year, month) WHERE deleted_at IS NULL
```

### 4.4 APIs / Contratos

#### Endpoint: `POST /webhooks/{uuid}` (receptor, comportamiento corregido)

**Request**:
```json
{
  "year": 2026,
  "month": 8,
  "amount": 975.97,
  "invoice_number": "<NUM_FACTURA>",
  "status": "pending"
}
```

**Response 200** (periodo soft-deleted reactivado):
```json
{
  "created": false,
  "bill": { "id": 7, "service_id": 3, "year": 2026, "month": 8, "amount": 975.97, "status": "pending" }
}
```

**Response Error** (sin cambios para errores reales):
```json
{ "error": "invalid_body", "message": "..." }
```

### 4.5 Dependencias

- **Internas**: `../p40la-ihost` → `internal/storage/bill.go`, `internal/services/webhook.go`, `internal/models`. Este repo → `internal/services/webhook.go` (solo si REQ-006).
- **Externas**: **Ninguna nueva**. SQLite (`database/sql`); opcional `github.com/mattn/go-sqlite3` ya usado en `p40la-ihost` para detectar `ErrConstraintUnique`.

## 5. Criterios de Aceptación

### 5.1 Funcionales

- [ ] CA-001: Dado un periodo con factura soft-deleted en el receptor, cuando se reenvía `POST /webhooks/{uuid}`, entonces responde `200` y la factura vuelve a estar activa con los datos del payload.
- [ ] CA-002: Dado un periodo nuevo, cuando se envía `POST /webhooks/{uuid}`, entonces responde `200 created: true` (sin regresión).
- [ ] CA-003: Dado un periodo existente no borrado, cuando se reenvía, entonces responde `200 created: false` (idempotencia intacta).
- [ ] CA-004: Con `status=paid` en un periodo reactivado, la factura queda pagada con `paid_at`/`payment_reference` (si aplica).
- [ ] CA-005: El fix queda cubierto por un test de integración del webhook en `p40la-ihost` (payload duplicado sobre fila soft-deleted → 200).

### 5.2 No funcionales

- [ ] CA-NF-001: `go build ./...` compila en `p40la-ihost` sin dependencias nuevas obligatorias.
- [ ] CA-NF-002: `npm run build` (este repo) intacto si se toca frontend/i18n.

### 5.3 Testing

- **Unit tests** (`p40la-ihost`): `FindByServicePeriodIncludingDeleted`, `Reactivate`.
- **Integration tests** (`p40la-ihost`): webhook sobre fila soft-deleted → 200 + fila activa; repetición → 200 sin duplicar.
- **E2E** (este repo): ejecutar `POST /api/accounts/{id}/webhook:test` o el job y verificar que los meses 2026/08 y 2026/09 quedan `ok` en `webhook_logs`.
- **Carga/Performance**: Sin impacto medible (una query extra solo en el caso soft-deleted).

## 6. Plan de Implementación

### 6.1 Fases

| Fase | Descripción | Estimación | Dependencias |
|------|-------------|------------|--------------|
| 1 | En `p40la-ihost`: `FindByServicePeriodIncludingDeleted` + `Reactivate` en `BillStorage` | 0.25 día | Ninguna |
| 2 | En `p40la-ihost`: manejo de UNIQUE en `WebhookService.UpsertBill` (detectar 2067 → reactivar/actualizar) | 0.25 día | Fase 1 |
| 3 | Tests de integración del webhook (soft-delete → 200; idempotencia intacta) | 0.25 día | Fase 2 |
| 4 | (Opcional REQ-004) Migración a índice UNIQUE parcial + backfill | 0.25 día | Fase 2 |
| 5 | (Opcional REQ-006) Etiquetado del 400 en `webhook_logs` de este repo | 0.1 día | Fase 2 |
| 6 | Verificación en vivo contra el webhook real: 2026/08 y 2026/09 → 200 | 0.1 día | Fase 3 |

### 6.2 Milestones

- **MVP**: Fases 1-3 + verificación en vivo: los periodos 2026/08 y 2026/09 responden 200 y se sincronizan.
- **V1.0**: REQ-004 (índice UNIQUE parcial) si se decide como refuerzo estructural.

## 7. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| Reactivar una fila soft-deleted conserva datos residuales (`paid_at`, `file_hash`) | Media | Medio | Actualizar campos descriptivos desde el payload y seguir la lógica de pago de SPEC-069 |
| Migración a índice UNIQUE parcial genera filas duplicadas si ya hay soft-deleted del mismo periodo | Baja | Medio | Backfill/limpieza previa en la migración o descartar REQ-004 (REQ-001 es suficiente) |
| El fix toca el receptor pero la spec vive en el emisor | Media | Bajo | Documentar claramente la ubicación; coordinarse con el repo `p40la-ihost` y su tracker (SPEC-069) |
| Error 2067 detectado por string en vez de tipo | Baja | Bajo | Usar `errors.Is(err, sqlite3.ErrConstraintUnique)` (mattn/go-sqlite3) |

## 8. Notas y Referencias

- Reproducción en vivo: `http://192.168.1.191:8088/webhooks/bf05bab4-6421-42c5-b9c3-449dc6c1b421` (payloads 2026/08 y 2026/09 → 400; 2025/01, 2025/02, 2026/01, 2026/03, 2026/04-07, 2026/10-12 → 200).
- Receptor: `../p40la-ihost` — `internal/services/webhook.go`, `internal/storage/bill.go`, `internal/api/bill_handlers.go`, `migrations/0005_create_bills.up.sql`.
- Contrato webhook: `../p40la-ihost/docs/webhooks-api.md` (§Idempotencia).
- Spec relacionada del receptor: `../p40la-ihost/docs/specs/SPEC-069-webhooks-facturas-por-servicio.md`.
- Este repo: `docs/specs/SPEC-004-jobs-webhook-facturas.md` (emisor), `internal/services/webhook.go` (sender + `webhook_logs`).
- Código SQLite: `SQLITE_CONSTRAINT_UNIQUE = 2067`.

## 9. Historial de Cambios

| Fecha | Autor | Descripción |
|-------|-------|-------------|
| 2026-09-12 | paulomcnally | Creación inicial: investigación del error 400 `UNIQUE constraint (2067)` en webhook de Claro Nicaragua; reproducción en vivo; causa raíz en `p40la-ihost` (soft-delete + UNIQUE a nivel tabla vs `FindByServicePeriod` con `deleted_at IS NULL`); fix propuesto en el receptor (reactivar soft-deleted) con índice UNIQUE parcial opcional. |
| 2026-09-12 | paulomcnally | **Cancelada**: el fix se implementa en el repo `p40la-ihost` (receptor), que tiene su propio tracker de specs. La spec del fix se traslada a `../p40la-ihost/docs/specs/SPEC-071-*.md`. Este archivo queda como documento de diagnóstico y el issue #6 se cierra. |