---
title: "Fix plugin tigo.nicaragua: formato de periodo y montos en webhook"
id: "SPEC-008"
status: "released"
author: "paulomcnally"
created: "2026-09-12"
updated: "2026-09-12"
github_issue: 9
---

# Fix plugin tigo.nicaragua: formato de periodo y montos en webhook

**ID**: SPEC-008  
**Estado**: released  
**Autor**: paulomcnally  
**Creado**: 2026-09-12  
**Actualizado**: 2026-09-12

---

## 1. Resumen Ejecutivo

Al correr el job diario que envía facturas al webhook de `p40la-ihost`
(`http://192.168.1.191:8088/webhooks/e55feb30-...`), las facturas del plugin
**Tigo** fallan con el error:

```
periodo inválido "2026-03": parsing time "2026-03" as "02-01-2006": cannot parse "26-03" as "-"
```

La causa: el plugin `tigo.nicaragua` normaliza el `billingPeriod` de la API
(`"08/2026"`) a **`YYYY-MM`** (`"2026-08"`, `nicaragua.go:349` vía
`normalizePeriod`), pero el emisor del webhook (`internal/services/webhook.go`)
solo acepta el formato **`DD-MM-YYYY`** (`parseBillPeriod`, layout `"02-01-2006"`,
diseñado para Claro). Cada factura Tigo se cuenta como `failed` en
`webhook_logs` y nunca llega al receptor. Sí, la sospecha del usuario es
correcta: **el plugin Tigo no respeta el formato de periodo que sí respeta Claro**.

El segundo problema: **las facturas pagadas aparecen con monto 0**. En
`fetchBills` el monto es `firstKey(inv, "dueAmount", "invoiceAmount")`
(`nicaragua.go:350`): para una factura con `hasPayment: true`, `dueAmount` es
`0` y el plugin lo reporta tal cual, sin caer al fallback `invoiceAmount` (el
monto real, ej. C$699.99). El patrón correcto ya existe en la propia ruta de
saldo (`balanceAsBill`, `nicaragua.go:155-158`) y fue aplicado históricamente en
el plugin Claro (SPEC-003, v1.3.0: usar el monto real, derivar el estado aparte).

Esta spec corrige ambos problemas: hace al emisor del webhook **tolerante al
formato de periodo** (acepta `DD-MM-YYYY` y `YYYY-MM`, sin romper Claro) y hace
que el plugin Tigo reporte el **monto real** en facturas pagadas. Sin deps
nuevas y sin cambios de esquema: la corrección es acotada a `webhook.go` y al
plugin Tigo, con tests de regresión.

## 2. Requerimientos

### 2.1 Requerimientos Funcionales (P0 - Obligatorios)

1. **REQ-001**: `parseBillPeriod` (`internal/services/webhook.go`) debe aceptar **ambos formatos**:
   - `DD-MM-YYYY` (formato de Claro, layout actual `"02-01-2006"`), y
   - `YYYY-MM` (formato del plugin Tigo). En este caso se usa día 1 del mes.
   El resultado (year, month) es idéntico; el contrato del receptor
   (`year`/`month` ints, `docs/webhooks-api.md` del proyecto hermano) no cambia.
2. **REQ-002**: El plugin `tigo.nicaragua` (`internal/plugins/tigo/nicaragua/nicaragua.go`,
   `fetchBills`) debe reportar el **monto real** de cada factura. Para facturas
   pagadas (`hasPayment: true`, `dueAmount: 0`) el `amount` debe ser el de
   `invoiceAmount` (ej. `699.99`), no `0`. Orden de preferencia:
   `invoiceAmount` (monto real) → fallback `dueAmount` si el primero está vacío.

### 2.2 Requerimientos Funcionales (P1 - Importantes)

1. **REQ-003**: Tests unitarios de regresión que cubran:
   - `parseBillPeriod` con `"2026-03"` (YYYY-MM) y con `"03-03-2026"` (DD-MM-YYYY).
   - `fetchBills` con una factura pagada (fixture ya existente en
     `nicaragua_test.go:118-119`: `dueAmount 0` + `invoiceAmount 699.99` +
     `hasPayment true`) → `amount == "699.99"`, `status == "paid"`.
2. **REQ-004**: `buildWebhookPayload` (`webhook.go:241-243`) debe incluir
   `invoice_number` también para Tigo, leyendo `b.Raw["invoiceId"]` además de
   `b.Raw["numFactura"]` (hoy solo funciona para Claro).

### 2.3 Requerimientos Funcionales (P2 - Deseables)

1. **REQ-005**: Alinear el periodo de `balanceAsBill` (`nicaragua.go:164`) para
   consistencia interna (hoy usa `time.Now().UTC().Format("2006-01")`). Con
   REQ-001 no es bloqueante, pero mantener un solo formato de periodo en el
   plugin reduce confusión futura.

### 2.4 Requerimientos No Funcionales

- **Rendimiento**: Sin impacto (parsing local, una regex extra o un layout más).
- **Seguridad**: Sin cambios (no se tocan credenciales ni headers).
- **Almacenamiento**: Cero migraciones; sin cambios de esquema.
- **Disponibilidad**: La corrección no bloquea otras peticiones.
- **iHost**: Cero dependencias nuevas (solo stdlib `time`). El parseo de
  `YYYY-MM` usa `time.Parse("2006-01", ...)`.

## 3. Investigación y Decisiones Técnicas

### 3.1 Contexto investigado

- **`internal/services/webhook.go:247-254`**: `parseBillPeriod` solo interpreta
  `DD-MM-YYYY` (`time.Parse("02-01-2006", ...)`). Al recibir `"2026-03"` (YYYY-MM),
  Go consume `"20"` como día y falla al esperar `-` → error exacto del reporte.
- **`internal/services/webhook.go:225-245`**: `buildWebhookPayload` falla antes de
  enviar; la factura se registra como `failed` en `webhook_logs` (`webhook.go:157-163`).
- **`internal/plugins/tigo/nicaragua/nicaragua.go:349`**: `period :=
  normalizePeriod(...)` → `"08/2026"` → `"2026-08"` (YYYY-MM), intencional y
  testeado (`nicaragua_test.go:133`).
- **`internal/plugins/tigo/nicaragua/nicaragua.go:350`**: `amount :=
  firstKey(inv, "dueAmount", "invoiceAmount")` → para factura pagada
  `dueAmount` existe con `0` → `Amount = "0"`. Sin fallback (a diferencia de
  `balanceAsBill`, `nicaragua.go:155-158`).
- **`internal/plugins/claro/nicaragua/nicaragua.go:206-222`**: Claro pasa el
  periodo crudo (`fechaEmision`, DD-MM-YYYY) y usa `montoFactura` (monto real);
  el estado se deriva de `balanceRestante`. SPEC-003 (historial, línea 353)
  documenta que este exacto bug de "pagadas en 0" ya se corrigió en Claro v1.3.0.
- **Contrato del receptor** (`/home/paulomcnally/github/p40la-ihost/docs/webhooks-api.md`,
  sección 4): el webhook recibe `year` y `month` como **ints**, no una fecha. El
  parseo local de la cadena es una capa interna del emisor; relajarla no cambia
  el contrato externo.

### 3.2 Opciones evaluadas

| Opción | Pros | Contras | Decisión |
|--------|------|---------|----------|
| Hacer `parseBillPeriod` tolerante (DD-MM-YYYY y YYYY-MM) | Corrige el error sin tocar el formato del plugin; Claro no se rompe; el contrato externo (year/month ints) no cambia | El emisor acepta dos formatos (defensivo) | ✅ Seleccionada |
| Cambiar el plugin Tigo a emitir DD-MM-YYYY | Un solo formato en todo el pipeline | El periodo Tigo no tiene día (`"08/2026"`), habría que inventar día 1; duplicaría la corrección en `balanceAsBill`; cambiaría la representación persistida en `bills` | ❌ Rechazada (más superficie, sin beneficio frente a REQ-001) |
| Plugin Tigo: preferir `invoiceAmount` con fallback a `dueAmount` | Usa el monto real (patrón de Claro y `balanceAsBill`); resuelve el monto 0 | Ninguna relevante | ✅ Seleccionada |
| Mantener `dueAmount` y añadir campo extra | No requiere cambios | Reproduce el bug; el webhook sigue recibiendo monto 0 | ❌ Rechazada |

### 3.3 Decisiones arquitectónicas (ADRs)

**ADR-001**: El emisor del webhook es tolerante al formato de periodo
- **Contexto**: Claro envía `DD-MM-YYYY` y Tigo `YYYY-MM`; el receptor solo
  necesita `year`/`month` ints. Forzar un solo formato de origen exige cambios
  en plugin + datos persistidos.
- **Decisión**: `parseBillPeriod` intenta primero `DD-MM-YYYY` y, si falla,
  `YYYY-MM` (día 1). Ambos plugins siguen funcionando sin cambiar sus formatos.
- **Consecuencias**: Menor superficie de cambio; riesgo de regresión nulo en
  Claro (su formato se intenta primero).

**ADR-002**: El plugin Tigo reporta el monto real (`invoiceAmount`)
- **Contexto**: Las facturas pagadas tienen `dueAmount: 0`; `invoiceAmount` es el
  monto real (C$699.99). Es el mismo fix que recibió Claro.
- **Decisión**: `fetchBills` usa `invoiceAmount` y cae a `dueAmount` solo si el
  primero está vacío. El estado se sigue derivando de `hasPayment`.
- **Consecuencias**: Las facturas pagadas llegan al webhook con su monto real.

## 4. Diseño Técnico

### 4.1 Diagrama de arquitectura

```
[plugins: tigo.nicaragua]          [plugins: claro.nicaragua]
  Period: "2026-08" (YYYY-MM)        Period: "DD-MM-YYYY"
  Amount: invoiceAmount (real)        Amount: montoFactura (real)
        \                                   /
         v                                 v
  [services.webhook.parseBillPeriod]  ← tolerante (ambos formatos)
         |
         v
  [POST /webhooks/{uuid}]  →  { year, month, amount, status, invoice_number }
         |
         v
  [p40la-ihost]  SQLite (bills)
```

### 4.2 Componentes

#### 4.2.1 `internal/services/webhook.go`
- **Responsabilidad**: Construir el payload del webhook a partir de una factura.
- **Cambio**: `parseBillPeriod` acepta `DD-MM-YYYY` y `YYYY-MM`.
- **Cambio**: `buildWebhookPayload` lee `invoice_number` de `Raw["invoiceId"]`
  (Tigo) además de `Raw["numFactura"]` (Claro).
- **Dependencias**: `time` (stdlib).

#### 4.2.2 `internal/plugins/tigo/nicaragua/nicaragua.go`
- **Responsabilidad**: Normalizar `data.invoiceList[]` a `[]plugins.Bill`.
- **Cambio**: En `fetchBills`, `amount` prefiere `invoiceAmount` con fallback a
  `dueAmount` (patrón de `balanceAsBill`).
- **Dependencias**: stdlib.

### 4.3 Modelo de datos

```
Sin cambios de esquema. El periodo sigue guardándose tal cual lo genera cada
plugin (Tigo: "2026-08"; Claro: "DD-MM-YYYY"). Solo cambia su interpretación
al construir el payload del webhook.
```

### 4.4 APIs / Contratos

#### Función: `parseBillPeriod(period string) (int, int, error)`

- `"03-03-2026"` (Claro) → `(2026, 3)`
- `"2026-03"` (Tigo) → `(2026, 3)` ← nuevo
- `""` / inválido → error (factura marcada `failed`, comportamiento actual)

#### Payload enviado al webhook (sin cambios de contrato)

```json
{
  "year": 2026,
  "month": 3,
  "amount": 699.99,
  "status": "paid",
  "invoice_number": "<NUM_FACTURA>"
}
```

### 4.5 Dependencias

- **Internas**: `internal/services` (webhook), `internal/plugins/tigo/nicaragua`.
- **Externas**: Ninguna nueva (solo stdlib).

## 5. Criterios de Aceptación

### 5.1 Funcionales

- [ ] CA-001: Dado un job con una factura Tigo de periodo `"2026-03"`, cuando se
  construye el payload, entonces no falla y envía `year: 2026`, `month: 3`.
- [ ] CA-002: Dado un job con una factura Tigo pagada (`hasPayment: true`,
  `dueAmount: 0`, `invoiceAmount: 699.99`), cuando se construye el payload,
  entonces envía `amount: 699.99` (no `0`) y `status: "paid"`.
- [ ] CA-003: Dado un job con una factura Claro de periodo `DD-MM-YYYY`, cuando
  se construye el payload, entonces sigue funcionando sin cambios (regresión).
- [ ] CA-004: Dado el webhook de Tigo con `Raw["invoiceId"]`, cuando se construye
  el payload, entonces `invoice_number` queda poblado.
- [ ] CA-DARK: N/A (no toca formularios frontend).
- [ ] CA-BACK: N/A (sin páginas nuevas).

### 5.2 No funcionales

- [ ] CA-NF-001: `go build ./...` compila sin errores.
- [ ] CA-NF-002: `go test ./internal/services/...` y `go test ./internal/plugins/tigo/...` pasan.
- [ ] CA-NF-003: Sin dependencias nuevas en `go.mod`.

### 5.3 Testing

- **Unit tests**: `parseBillPeriod` (ambos formatos + inválido); `fetchBills` con
  factura pagada y pendiente; `buildWebhookPayload` con `invoiceId`.
- **Integration tests**: Flujo `fetch` → `buildWebhookPayload` con fixture Tigo
  (periodo `2026-03`, factura pagada) y verificar que el payload llega con
  `year/month/amount` correctos.
- **E2E tests**: Job diario real enviando a `http://192.168.1.191:8088/webhooks/...`
  y verificar que las facturas Tigo aparecen en `p40la-ihost` con montos reales.
- **Carga/Performance**: Sin métricas nuevas; parsing local despreciable.

## 6. Plan de Implementación

### 6.1 Fases

| Fase | Descripción | Estimación | Dependencias |
|------|-------------|------------|--------------|
| 1 | `parseBillPeriod` tolerante (YYYY-MM fallback) + tests | 0.25 día | Ninguna |
| 2 | Fix de monto en `fetchBills` (preferir `invoiceAmount`) + tests | 0.25 día | Fase 1 |
| 3 | `invoice_number` para Tigo (`Raw["invoiceId"]`) en `buildWebhookPayload` | 0.1 día | Fase 1 |
| 4 | `go build ./...` + `go test ./internal/...` + verificación e2e del job | 0.25 día | Fase 3 |

### 6.2 Milestones

- **MVP**: `parseBillPeriod` tolerante + monto real en facturas pagadas Tigo.
- **V1.0**: `invoice_number` para Tigo + tests de regresión completos.

## 7. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| Ambigüedad entre formatos en `parseBillPeriod` | Baja | Bajo | `DD-MM-YYYY` se intenta primero; ambos se distinguen sin ambigüedad real (longitud/guiones). Test cubre ambos. |
| Alguna factura Tigo sin `invoiceAmount` | Baja | Medio | Fallback a `dueAmount` mantiene comportamiento actual para esos casos. |
| Regresión en Claro | Baja | Medio | El formato Claro se intenta primero y se agrega test de regresión (CA-003). |
| Facturas ya enviadas con monto 0 quedan en `p40la-ihost` | Media | Bajo | El webhook hace upsert por `(servicio, año, mes)`; reenviar el período con el monto real corrige la fila (comportamiento del receptor). |

## 8. Notas y Referencias

- Error reportado: `periodo inválido "2026-03": parsing time "2026-03" as "02-01-2006": cannot parse "26-03" as "-"`.
- Webhook destino: `http://192.168.1.191:8088/webhooks/e55feb30-a7fc-4f48-b26f-9765be39b8dd`.
- Archivos: `internal/services/webhook.go`, `internal/plugins/tigo/nicaragua/nicaragua.go` (+ `_test.go`), `internal/plugins/claro/nicaragua/nicaragua.go` (referencia).
- Contrato del receptor: `/home/paulomcnally/github/p40la-ihost/docs/webhooks-api.md` (sección 4: `year`/`month` ints).
- Precedente: SPEC-003 (Claro v1.3.0 corrigió el mismo bug de montos en facturas pagadas).
- Relacionadas: SPEC-007 (plugin tigo.nicaragua), SPEC-004 (jobs diarios + webhook).

## 9. Historial de Cambios

| Fecha | Autor | Descripción |
|-------|-------|-------------|
| 2026-09-12 | paulomcnally | Creación inicial de la especificación: bug de periodo (Tigo `YYYY-MM` vs emisor `DD-MM-YYYY`) y monto 0 en facturas pagadas del plugin Tigo. |
| 2026-09-12 | paulomcnally | Estado → `pending_execution` → `in_progress`. Inicio de desarrollo. |
| 2026-09-12 | paulomcnally | Implementación completa: `parseBillPeriod` tolerante (DD-MM-YYYY + YYYY-MM) en `webhook.go`, `invoice_number` para Tigo (`Raw["invoiceId"]`), `billAmount` en el plugin Tigo (prefiere `invoiceAmount`). Tests: `webhook_test.go` nuevo + cobertura de factura pagada en `nicaragua_test.go`. `go build ./...` y `go test ./...` OK. Estado → `pending_release`. |
| 2026-09-12 | paulomcnally | **Cierre por decisión del usuario**: el release/deploy a iHost lo realiza el usuario. Issue #9 cerrado con label `spec/released`. Código verificado (`go build`/`go test` OK), pendiente del deploy manual del usuario. |