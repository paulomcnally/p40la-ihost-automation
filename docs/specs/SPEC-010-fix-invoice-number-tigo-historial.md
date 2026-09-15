---
title: "Fix invoice_number Tigo + invoice_number en historial"
id: "SPEC-010"
status: "released"
author: "paulomcnally"
created: "2026-09-12"
updated: "2026-09-12"
github_issue: 11
---

# Fix invoice_number Tigo + invoice_number en historial

**ID**: SPEC-010  
**Estado**: released  
**Autor**: paulomcnally  
**Creado**: 2026-09-12  
**Actualizado**: 2026-09-12

---

## 1. Resumen Ejecutivo

Al ejecutar el job diario del plugin **`tigo.nicaragua`**, el número de factura que se envía
al webhook llega como un objeto Go serializado en vez del valor limpio. En el receptor se
observa:

```
"invoice_number": "map[formattedValue:<NUM_FACTURA> label:Factura: show:true value:<NUM_FACTURA>]"
```

El valor esperado es `"<NUM_FACTURA>"`. La causa raíz está confirmada con la captura real
de Charles (`charles_tigo_session.chlz`, flujo 94): la API de Mi Cuenta Tigo envuelve cada
campo en `{label, show, value, formattedValue}`. El plugin Tigo guarda en `Raw` el mapa crudo
de la API (`Raw: inv`, `nicaragua.go:358`), y el emisor del webhook (`internal/services/webhook.go:243`)
lee `b.Raw["invoiceId"]` y aplica `fmt.Sprint(invoice)` sobre un `map[string]any`, produciendo
el `map[...]` literal. El plugin Tigo ya tiene la función `unwrap` para extraer el valor real,
pero no se aplica en `buildWebhookPayload`.

El usuario no pudo detectar el bug mirando el **historial de facturas** porque la UI
(`AccountBillsPage.tsx`) solo muestra `period`, `due_date`, `status` y `amount` de cada
factura — **el `invoice_number` no se muestra en ninguna parte del historial**. La spec
corrige el valor que se envía al webhook y, además, **expone `invoice_number` en el historial
de `bills` para todos los plugins** (Claro, Tigo y DISNORTE), de modo que en las pruebas se
pueda verificar de un vistazo si el valor es correcto.

La corrección es acotada: una función de extracción de `invoice_number` robusta en el emisor
del webhook (desenvuelve `{value, formattedValue}`), la adición del campo `invoice_number` al
historial persistido y a la UI, y tests de regresión. **Sin deps nuevas y sin cambios de
esquema** (el `raw` JSON de `bills` ya existe; solo se agrega un campo derivado en la respuesta).

## 2. Requerimientos

### 2.1 Requerimientos Funcionales (P0 - Obligatorios)

1. **REQ-001**: `buildWebhookPayload` (`internal/services/webhook.go`) debe producir
   `invoice_number` como **valor limpio**, no como `map[...]`. Para Tigo, `Raw["invoiceId"]`
   llega como `{label, show, value, formattedValue}`; el `invoice_number` enviado debe ser el
   `value`/`formattedValue` escalar (ej. `<NUM_FACTURA>`). Para Claro/DISNORTE, `Raw["numFactura"]`
   ya es un string; debe seguir funcionando igual.
2. **REQ-002**: El historial de facturas debe exponer `invoice_number` por factura. Hoy
   `BillPayload` (`internal/services/bills.go`) no lo incluye. Agregar el campo
   `invoice_number` a la respuesta de `POST /api/accounts/{id}/bills:fetch` y a
   `GET /api/accounts/{id}/bills` (historial persistido), derivado de `Raw` con la misma
   lógica de extracción de REQ-001.
3. **REQ-003**: La UI de historial (`frontend/src/pages/AccountBillsPage.tsx`) debe mostrar el
   `invoice_number` de cada factura (junto a `period`, `due_date`, `status`, `amount`), para
   que en las pruebas se verifique de un vistazo si el valor está correcto. Aplica a todos los
   plugins. El campo es opcional (`omitempty`/`??`), sin romper facturas que no lo tengan.

### 2.2 Requerimientos Funcionales (P1 - Importantes)

1. **REQ-004**: Tests unitarios de regresión:
   - `buildWebhookPayload` con `Raw["invoiceId"]` como `{label, show, value, formattedValue}`
     (fixture real de Tigo) → `invoice_number == "<NUM_FACTURA>"`.
   - `buildWebhookPayload` con `Raw["numFactura"]` string (Claro/DISNORTE) → sin cambios.
   - `parseBills`/historial: el `invoice_number` derivado aparece en la respuesta del fetch.
2. **REQ-005**: La extracción de `invoice_number` debe ser tolerante: si `Raw` trae
   `invoice_number` directo, `numFactura` (string), o `invoiceId` como `{value, formattedValue}`
   o como string, todos los casos resuelven al valor limpio o se omite el campo.

### 2.3 Requerimientos Funcionales (P2 - Deseables)

1. **REQ-006**: Normalizar en el plugin Tigo: en lugar de depender del emisor, el plugin podría
   poner `Raw["invoice_number"]` limpio ya desenvuelto (o `Raw["invoiceId"]` desenvuelto). Se
   evalúa como mejora de consistencia interna; REQ-001 ya lo resuelve a nivel del emisor.

### 2.4 Requerimientos No Funcionales

- **Rendimiento**: Sin impacto (extracción local de un campo, una función pura).
- **Seguridad**: Sin cambios (no se tocan credenciales ni headers).
- **Almacenamiento**: Cero migraciones; el `raw` JSON de `bills` no cambia de estructura
  (solo la respuesta derivada agrega `invoice_number`).
- **Disponibilidad**: La corrección no bloquea otras peticiones.
- **iHost**: Cero dependencias nuevas (solo stdlib `encoding/json`, `fmt`). La lógica de
  extracción usa `switch type` sobre `any`.

## 3. Investigación y Decisiones Técnicas

### 3.1 Contexto investigado

- **Captura real** (`charles_tigo_session.chlz`, flujo 94, `GET .../invoices?_format=json`):
  `data.invoiceList[]` devuelve cada factura con campos envueltos en
  `{label, show, value, formattedValue}`:
  ```json
  {
    "invoiceId": { "label": "Factura:", "show": true, "value": "<NUM_FACTURA>", "formattedValue": "<NUM_FACTURA>" },
    "billingPeriod": { "label": "Período:", ..., "value": {"startDateTime": "2026-07-20T12:00:00", "endDateTime": "2026-08-19T11:59:59"}, "formattedValue": "08/2026" },
    "invoiceAmount": { ..., "value": 700.46, "formattedValue": "C$700.46" },
    "dueAmount": { ..., "value": 700.46, ... },
    "dueDate": { ..., "value": "2026-09-19T00:00:00", "formattedValue": "19/Sep/2026" },
    "hasPayment": { ..., "value": false, "formattedValue": "Pendiente" }
  }
  ```
- **`internal/plugins/tigo/nicaragua/nicaragua.go:358`**: `Raw: inv` guarda el mapa crudo
  completo (con `invoiceId` como objeto envuelto). El plugin tiene `unwrap` (línea 370) que ya
  extrae el valor real, pero no lo aplica a `invoiceId` en `Raw`.
- **`internal/services/webhook.go:241-245`**: `buildWebhookPayload` hace
  `fmt.Sprint(b.Raw["invoiceId"])` → `map[formattedValue:<NUM_FACTURA> label:Factura: show:true value:<NUM_FACTURA>]`.
  El test existente (`webhook_test.go:46`) usa `Raw: {"invoiceId": "<NUM_FACTURA>"}` (string
  directo), por lo que **no detecta el bug**: no replica el formato real de la API.
- **Historial** (`internal/services/bills.go`, `frontend/src/api/index.ts:55`, 
  `frontend/src/pages/AccountBillsPage.tsx:387-396`): `BillPayload` expone
  `period/amount/due_date/status/raw`; la UI muestra solo `period`, `due_date`, `status`,
  `amount`. El `invoice_number` no aparece en ningún lado del historial.
- **Claro/DISNORTE**: `Raw["numFactura"]` es un string plano; `buildWebhookPayload` ya lo
  convierte bien. Solo Tigo rompe el formato.

### 3.2 Opciones evaluadas

| Opción | Pros | Contras | Decisión |
|--------|------|---------|----------|
| Desenvolver `invoiceId` en `buildWebhookPayload` (switch type) | Corrige el valor sin tocar el plugin; un solo lugar para todos los plugins | La lógica de unwrap queda en el emisor (duplicada con `unwrap` del plugin) | ✅ Seleccionada |
| Normalizar `Raw["invoice_number"]` en el plugin Tigo | El plugin ya tiene `unwrap`; queda limpio aguas abajo | Deja el bug latente si otro plugin repite el patrón; toca el plugin | ❌ Rechazada para el MVP (queda como P2) |
| Solo corregir `buildWebhookPayload` sin historial | Arreglo mínimo | El usuario no puede verificar en el historial; el bug pasa desapercibido de nuevo | ❌ Rechazada |
| Agregar columna `invoice_number` a `bills` (migración) | Campo tipado en SQL | Migración nueva; el `raw` JSON ya lo puede derivar; más superficie | ❌ Rechazada (cero migraciones) |
| Mostrar `invoice_number` derivado en la UI | Verificación visual inmediata en pruebas | Requiere tocar el frontend (tipo `BillItem` + render) | ✅ Seleccionada |

### 3.3 Decisiones arquitectónicas (ADRs)

**ADR-001**: Extraer `invoice_number` con una función pura tolerante en el emisor del webhook
- **Contexto**: `Raw["invoiceId"]` puede ser string o `{label, show, value, formattedValue}`;
  `Raw["numFactura"]` es string; `Raw["invoice_number"]` puede ser string directo.
- **Decisión**: Nueva función `extractInvoiceNumber(raw map[string]any) string` que pruebe las
  claves `invoice_number`, `numFactura`, `invoiceId` y, para valores tipo mapa, prefiera
  `value` (si escalar) o `formattedValue`. Si no hay valor escalar, devuelve "" (se omite).
- **Consecuencias**: Claro/DISNORTE/Tigo resuelven el número limpio desde un solo lugar.

**ADR-002**: Exponer `invoice_number` en `BillPayload` y en el historial
- **Contexto**: La UI no muestra el número de factura; el usuario no puede detectar el bug.
- **Decisión**: `BillPayload` gana `InvoiceNumber string \`json:"invoice_number,omitempty"\``,
  poblado en `BillsService.FetchBills` con `extractInvoiceNumber`. La UI muestra el campo.
- **Consecuencias**: El historial (fetch + listado) incluye el número de factura para todos los
  plugins; las pruebas lo verifican visualmente.

**ADR-003**: Cero migraciones y cero deps
- **Contexto**: El `raw` JSON ya contiene la factura completa; derivar el número no requiere
  columna nueva.
- **Decisión**: La derivación es en memoria en la capa de servicios; `bills.raw` no cambia.
- **Consecuencias**: Sin cambios de esquema ni deps en iHost.

## 4. Diseño Técnico

### 4.1 Diagrama de arquitectura

```
[plugins: tigo.nicaragua]           [plugins: claro.nicaragua]      [plugins: disnorte.dissur.nicaragua]
  Raw["invoiceId"] = {label,show,       Raw["numFactura"] = "INV-..."    Raw["numFactura"] = "<NUM_RECIBO>"
       value, formattedValue}
        \                                    |                                   /
         v                                   v                                  v
  [services.BillPayload]  ← + invoice_number (extractInvoiceNumber, tolerante)
         |
         +--> [POST /webhooks/{uuid}]  →  { year, month, amount, status, invoice_number }   (valor limpio)
         |
         +--> [GET /api/accounts/{id}/bills]  →  historial con invoice_number por factura
                                                    ↓
                                        [AccountBillsPage.tsx]  ← muestra invoice_number en cada bill
```

### 4.2 Componentes

#### 4.2.1 `internal/services/webhook.go`
- **Responsabilidad**: Construir el payload del webhook a partir de una factura.
- **Cambio**: Nueva función `extractInvoiceNumber(raw map[string]any) string` y uso en
  `buildWebhookPayload` (reemplaza el bloque `numFactura`/`invoiceId` actual).
- **Dependencias**: `encoding/json`, `fmt` (stdlib).

#### 4.2.2 `internal/services/bills.go`
- **Responsabilidad**: Orquestar la consulta de facturas y construir la respuesta/historial.
- **Cambio**: `BillPayload` agrega `InvoiceNumber`; en `FetchBills`, poblar
  `InvoiceNumber` con `extractInvoiceNumber(b.Raw)`.
- **Dependencias**: `internal/services` (mismo paquete; `extractInvoiceNumber` compartida).

#### 4.2.3 `frontend/src/pages/AccountBillsPage.tsx` y `frontend/src/api/index.ts`
- **Responsabilidad**: Mostrar el historial de facturas.
- **Cambio**: `BillItem` agrega `invoice_number?: string`; el render de cada factura muestra
  `invoice_number` (si existe) junto a `due_date`/`status`.
- **Dependencias**: Tailwind (estilos existentes), i18n (nuevo label si aplica).

### 4.3 Modelo de datos

```
Sin cambios de esquema.

BillPayload (respuesta/historial):
  - period: string        (existente)
  - amount: string        (existente)
  - due_date: string      (existente)
  - status: string        (existente)
  - raw: map[string]any   (existente, omitempty)
  - invoice_number: string (NUEVO, omitempty) ← derivado con extractInvoiceNumber
```

### 4.4 APIs / Contratos

#### Endpoint: `POST /api/accounts/{accountId}/bills:fetch` (existente)

**Response 201** (cada bill ahora puede incluir `invoice_number`):
```json
{
  "id": 43,
  "account_id": 7,
  "plugin_name": "tigo.nicaragua",
  "plugin_version": "1.0.0",
  "status": "ok",
  "bills": [
    { "period": "2026-08", "amount": "700.46", "due_date": "2026-09-19", "status": "pending", "invoice_number": "<NUM_FACTURA>" }
  ]
}
```

#### Endpoint: `GET /api/accounts/{accountId}/bills` (existente)

**Response 200**: mismo `raw` de siempre (no cambia); la UI deriva el `invoice_number` de
cada item del `raw` (que ya se serializa en el `bills` del fetch y en `raw` del historial).

#### Payload enviado al webhook (corregido)

```json
{
  "year": 2026,
  "month": 8,
  "amount": 700.46,
  "status": "pending",
  "invoice_number": "<NUM_FACTURA>"
}
```

### 4.5 Dependencias

- **Internas**: `internal/services` (webhook, bills), `frontend` (AccountBillsPage, api/index.ts).
- **Externas**: Ninguna nueva (solo stdlib).

## 5. Criterios de Aceptación

### 5.1 Funcionales

- [ ] CA-001: Dado un job con una factura Tigo (`Raw["invoiceId"]` como objeto
  `{label, show, value, formattedValue}`), cuando se construye el payload, entonces
  `invoice_number` es `"<NUM_FACTURA>"` (no el `map[...]`).
- [ ] CA-002: Dado un job con una factura Claro/DISNORTE (`Raw["numFactura"]` string), cuando
  se construye el payload, entonces `invoice_number` se mantiene correcto (regresión).
- [ ] CA-003: Dado un `POST /api/accounts/{id}/bills:fetch`, entonces cada factura de la
  respuesta incluye `invoice_number` (cuando existe en `Raw`).
- [ ] CA-004: Dado un `GET /api/accounts/{id}/bills`, entonces el historial en la UI muestra
  el `invoice_number` de cada factura (para todos los plugins).
- [ ] CA-005: Dada una factura sin número (Raw sin `invoiceId`/`numFactura`/`invoice_number`),
  entonces el campo se omite (no rompe el payload ni el historial).
- [ ] CA-DARK: Si la spec toca la UI (AccountBillsPage), los textos usan tokens del tema
  (`text-text-secondary`, `font-mono`) y se verifica legibilidad en darkmode.
- [ ] CA-BACK: N/A (sin páginas de detalle nuevas; el historial ya usa el layout existente).

### 5.2 No funcionales

- [ ] CA-NF-001: `go build ./...` compila sin errores.
- [ ] CA-NF-002: `go test ./internal/services/...` y `go test ./internal/plugins/tigo/...` pasan.
- [ ] CA-NF-003: `npm run build` (typecheck + build frontend) pasa.
- [ ] CA-NF-004: Sin dependencias nuevas en `go.mod` ni en `frontend/package.json`.

### 5.3 Testing

- **Unit tests**: `extractInvoiceNumber` (objeto `{value, formattedValue}`, string directo,
  ausente); `buildWebhookPayload` con fixture real de Tigo (`invoiceId` objeto) y con
  `numFactura` (Claro/DISNORTE); `BillPayload` serializa `invoice_number`.
- **Integration tests**: Flujo `fetch` → `buildWebhookPayload` con fixture Tigo y verificar
  que el payload llega con `invoice_number` limpio; historial con `invoice_number` poblado.
- **E2E tests**: Desde la UI, consultar facturas de una cuenta Tigo y verificar que el
  historial muestra `<NUM_FACTURA>` (no `map[...]`).
- **Carga/Performance**: Sin métricas nuevas; extracción local despreciable.

## 6. Plan de Implementación

### 6.1 Fases

| Fase | Descripción | Estimación | Dependencias |
|------|-------------|------------|--------------|
| 1 | `extractInvoiceNumber` + fix en `buildWebhookPayload` + tests | 0.25 día | Ninguna |
| 2 | `BillPayload.InvoiceNumber` poblado en `FetchBills` + test | 0.25 día | Fase 1 |
| 3 | Frontend: `BillItem.invoice_number` + render en AccountBillsPage + i18n | 0.25 día | Fase 2 |
| 4 | `go build`/`go test`/`npm run build` + verificación e2e local | 0.25 día | Fase 3 |

### 6.2 Milestones

- **MVP**: `invoice_number` limpio en el webhook (fix Tigo) + historial con `invoice_number`.
- **V1.0**: UI muestra `invoice_number` para todos los plugins + tests de regresión completos.

## 7. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| Algún plugin trae `invoiceId` como string (no objeto) | Media | Bajo | `extractInvoiceNumber` tolera ambos (string directo y mapa). |
| Regresión en Claro/DISNORTE | Baja | Medio | Se testea `numFactura` string (CA-002). |
| El `formattedValue` difiere del `value` (ej. incluye símbolo) | Baja | Bajo | Preferir `value` escalar; caer a `formattedValue` solo si `value` es compuesto. |
| UI sin espacio para el número de factura | Baja | Bajo | Texto truncado `truncate` con `font-mono`, junto al periodo; sin layout rígido. |
| Facturas viejas en `raw` sin el campo | Media | Bajo | `omitempty`/`??` omite el campo; no rompe render. |

## 8. Notas y Referencias

- Captura real: `/home/paulomcnally/Downloads/charles_tigo_session.chlz` (flujo 94:
  `GET .../mobile/billing/subscribers/<MSISDN>/invoices?_format=json`).
- Formato real de `invoiceId`: `{ "label": "Factura:", "show": true, "value": "<NUM_FACTURA>", "formattedValue": "<NUM_FACTURA>" }`.
- Archivos: `internal/services/webhook.go`, `internal/services/bills.go`,
  `frontend/src/pages/AccountBillsPage.tsx`, `frontend/src/api/index.ts` (+ `_test.go`).
- Relacionadas: SPEC-008 (parseBillPeriod tolerante), SPEC-007 (plugin tigo.nicaragua),
  SPEC-009 (plugin disnorte.dissur.nicaragua), SPEC-003 (plugin claro.nicaragua).
- Contrato del receptor: `/home/paulomcnally/github/p40la-ihost/docs/webhooks-api.md`
  (sección 4: `invoice_number` es string).

## 9. Historial de Cambios

| Fecha | Autor | Descripción |
|-------|-------|-------------|
| 2026-09-12 | paulomcnally | Creación inicial de la especificación: `invoice_number` de Tigo llega como `map[...]` (objeto `{label, show, value, formattedValue}` sin desenvolver en `buildWebhookPayload`), y el historial no muestra el número de factura, impidiendo detectar el bug en pruebas. Fix en el emisor + exposición del `invoice_number` en el historial para todos los plugins. Cero migraciones, cero deps. |
| 2026-09-12 | paulomcnally | Estado → `pending_execution` → `in_progress`. Inicio de desarrollo. |
| 2026-09-12 | paulomcnally | Implementación completa. `extractInvoiceNumber`/`scalarValue` en `webhook.go` (tolera `invoice_number` directo, `numFactura` string y `invoiceId` como `{label, show, value, formattedValue}`), `buildWebhookPayload` usa la extracción limpia. `BillPayload.InvoiceNumber` (`json:"invoice_number,omitempty"`) poblado en `FetchBills` (historial + fetch). Frontend: `BillItem.invoice_number?` + render en `AccountBillsPage.tsx` (font-mono, junto al periodo). Tests: fixture real de Tigo (objeto envuelto), `extractInvoiceNumber` (7 casos), `scalarValue`, regresión Claro/DISNORTE. Verificado end-to-end: fetch devuelve `<NUM_FACTURA>`, webhook recibió las 6 facturas con `invoice_number` limpio, historial expone el campo. `go build`/`go test`/`npm run build` OK. Estado → `pending_release`. |
| 2026-09-12 | paulomcnally | **Release** (cierre por decisión del usuario; el deploy a iHost lo realiza él). Código verificado end-to-end; el commit de implementación + release se realiza en esta sesión. Issue #11 cerrado con label `spec/released`. |