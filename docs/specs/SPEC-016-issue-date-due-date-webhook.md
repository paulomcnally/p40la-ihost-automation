---
title: "issue_date y due_date en webhook por plugin"
id: "SPEC-016"
status: "released"
author: "paulomcnally"
created: "2026-09-19"
updated: "2026-09-19"
github_issue: 18
---

# issue_date y due_date en webhook por plugin

**ID**: SPEC-016  
**Estado**: released  
**Autor**: paulomcnally  
**Creado**: 2026-09-19  
**Actualizado**: 2026-09-19

---

## 1. Resumen Ejecutivo

La guía de integración de webhooks del proyecto hermano
(`p40la-ihost/docs/webhooks-api.md`, v1.2, 2026-09-19) agregó dos campos
opcionales al contrato del payload: **`issue_date`** (fecha de emisión de la
factura, cuándo el proveedor la generó) y **`due_date`** (fecha de vencimiento,
cuándo hay que pagarla antes de que suspendan el servicio). Ambos se persisten
en p40la-ihost normalizados a `YYYY-MM-DD` (SPEC-081, ya released en el
proyecto hermano). Este proyecto — el cliente automatizado que empuja facturas
al webhook — aún no los envía: el payload actual de `buildWebhookPayload` solo
incluye `year`, `month`, `amount`, `status` e `invoice_number` (omitempty).

Esta spec agrega ambos campos al payload del webhook **por plugin**, obteniendo
las fechas de las APIs externas cuando estas las exponen. La regla central es:
**si la API del proveedor no tiene el dato, el campo simplemente no se envía**
(el webhook los trata como opcionales/aditivos; en actualizaciones conserva el
valor previo si el campo viene ausente). La implementación toca dos repos: el
módulo privado `p40la-ihost-automation-plugins` (contrato + cada plugin expone
las fechas que su API provee) y este repo (mapeo al payload del webhook).

El resultado esperado es que cada factura enviada al webhook cargue, cuando
exista, la fecha de emisión y de vencimiento reales del proveedor — datos
valiosos para que la app destino muestre vencimientos y antigüedad de cada
factura sin recalcular nada.
## 2. Requerimientos

### 2.1 Requerimientos Funcionales (P0 - Obligatorios)

1. **REQ-001**: Agregar campo `IssueDate` al contrato `plugins.Bill` del módulo
   privado (junto al `DueDate` existente), con la misma semántica: fecha
   normalizada `YYYY-MM-DD`, vacío si la API no la expone. No es obligatorio
   para ningún plugin.
2. **REQ-002**: Cada plugin (claro, tigo, enacal, disnorte, assa) debe poblar
   `IssueDate` y `DueDate` SOLO si su API externa expone el dato real, según la
   investigación de la sección 3. Si no lo expone, el campo queda vacío.
3. **REQ-003**: `buildWebhookPayload` en este repo debe agregar `issue_date` y
   `due_date` al payload **solo cuando el valor existe** y es una fecha válida
   (`YYYY-MM-DD`). Nunca enviar campos vacíos ni fechas placeholder.
4. **REQ-004**: El contrato del webhook (docs/webhooks-api.md de p40la-ihost)
   no cambia: los campos ya son opcionales. Este repo solo cumple el contrato.

### 2.2 Requerimientos Funcionales (P1 - Importantes)

1. **REQ-005**: Incluir `issue_date`/`due_date` también en el JSON del historial
   de consultas (`BillPayload` y su serialización en `Raw`), para que la UI y
   los logs de webhook muestren qué fechas se enviaron.
2. **REQ-006**: Normalizar formatos de fecha por plugin (ej. ENACAL `DD/MM/YYYY`
   → `YYYY-MM-DD`, Claro `DD-MM-YYYY`, ASSA `YYYY-MM-DDTHH:MM:SS`) con
   tolerancia: si el formato cambia, no enviar el campo en vez de fallar.

### 2.3 Requerimientos Funcionales (P2 - Deseables)

1. **REQ-007**: Mostrar `issue_date`/`due_date` en la UI de detalle de facturas
   de una cuenta (AccountBillsPage) si ya se muestran otros campos del payload.

### 2.4 Requerimientos No Funcionales

- **Rendimiento**: Cero impacto: solo se agregan 1-2 strings al JSON del
  payload. Sin llamadas extra a las APIs.
- **Seguridad**: No se exponen datos nuevos sensibles; las fechas son públicas
  en los portales. Sin nuevas credenciales ni almacenamiento.
- **Almacenamiento**: Sin migraciones de SQLite en este repo; el contrato del
  webhook (p40la-ihost) ya tiene las columnas (SPEC-081). El `Raw` JSON ya se
  persiste en `bill_records`.
- **Disponibilidad**: Si la API de un proveedor deja de enviar una fecha, el
  campo se omite y el webhook conserva el valor previo (semántica aditiva).
- **iHost**: Sin dependencias nuevas; cambios de strings puros, compatibles con
  memoria limitada.

## 3. Investigación y Decisiones Técnicas

### 3.1 Contexto investigado

Se auditaron las 5 implementaciones del módulo privado
(`github.com/paulomcnally/p40la-ihost-automation-plugins` v1.0.0) contra los
datos reales que devuelve cada API externa (capturas de Charles y respuestas
reales registradas en los tests):

| Plugin | Fuente (endpoint) | Campos de fecha en la API |
|--------|-------------------|---------------------------|
| claro.nicaragua | `RetrieveBillHistoryList` | `fechaEmision`, `fechaVencimiento` |
| tigo.nicaragua | `/mobile/billing/subscribers/{msisdn}/invoices` | `dueDate` (envuelto en `{value, formattedValue}`). Sin fecha de emisión en la captura (invoiceList trae `billingPeriod`, `invoiceId`, `dueAmount`, `invoiceAmount`, `hasPayment`, `dueDate`). Regla de negocio del proveedor (informada por el usuario): ciclo de facturación de 30 días → emisión el **día 20 del mes anterior al vencimiento** (`dueDate - 1 mes + 1 día`) |
| enacal.nicaragua | `/soe/api/InfoCuentas/facturas/` | `Emision`, `Vence` (formato `DD/MM/YYYY`) |
| disnorte.dissur.nicaragua | `/ovdev/api/cliente/traerrecibopagina/` | `FechaRecibo` (fecha del recibo); `FechaVencimiento` viene en `0001-01-01T00:00:00` (fecha cero de .NET, la API **no lo expone** — comentario en `nicaragua_test.go`). Regla de negocio del proveedor (informada por el usuario): el vencimiento es **20 días después** de la fecha del recibo |
| assa.nicaragua | `Unidades.aspx` (grids Ext.NET) | Polifacturas: `FECHA`, `VENCIMIENTO`. Recibos pagados (grid `grd_Pagos`, tab "Cobros"): `NUMRECIBO`, `FECCOBRO` (fecha de pago), `MES_PAGADO`, `MONTO`, `PREIMPRESO`. Verificado en el HTML del portal: la columna "Fecha de Vencimiento" mapea a `dataIndex: MES_PAGADO` (el vencimiento de cada recibo es `MES_PAGADO`, no `FECCOBRO`). No hay fecha de emisión en recibos; regla de negocio (usuario): **emisión = vencimiento** (`issue_date = MES_PAGADO`) |

### 3.2 Matriz de disponibilidad por plugin (decisión de diseño)

| Plugin | `issue_date` (emisión) | `due_date` (vencimiento) |
|--------|------------------------|--------------------------|
| claro.nicaragua | ✅ `fechaEmision` | ✅ `fechaVencimiento` |
| tigo.nicaragua | ✅ regla de negocio: día 20 del mes anterior al vencimiento (`dueDate - 1 mes + 1 día`; la API no trae fecha de emisión) | ✅ `dueDate` |
| enacal.nicaragua | ✅ `Emision` (normalizar DD/MM/YYYY) | ✅ `Vence` (normalizar DD/MM/YYYY) |
| disnorte.dissur.nicaragua | ✅ `FechaRecibo` (fecha de emisión del recibo) | ✅ regla de negocio: `FechaRecibo + 20 días` (la API no lo expone; `FechaVencimiento` es `0001-01-01`) |
| assa.nicaragua | ✅ `FECHA` en polifacturas; ✅ `MES_PAGADO` en recibos (regla: emisión = vencimiento) | ✅ `VENCIMIENTO` en polifacturas; ✅ `MES_PAGADO` en recibos (columna "Fecha de Vencimiento" del portal) |

Notas de la investigación:

- **Claro**: el plugin ya usa `fechaEmision` como `Period` y `fechaVencimiento`
  como `DueDate`. Ambos campos ya viajan en `Raw`; solo falta exponerlos.
- **Tigo**: el historial (`invoiceList`) no trae fecha de emisión en la captura
  real (6 facturas, claves: `billingPeriod`, `invoiceId`, `dueAmount`,
  `invoiceAmount`, `hasPayment`, `dueDate`). `dueDate` sí existe (objeto
  envuelto `{value, formattedValue}`) y ya se usa como `DueDate`. Para
  `issue_date`, el usuario confirmó la **regla de negocio del proveedor**:
  ciclo de facturación de 30 días. Validado contra 3 ejemplos reales del
  usuario (MSDFC-30999382/31182590/31369430), **"dueDate - 30 días" no es
  exacto**: falla cuando el mes anterior tiene 30 días (ej. emisión 20-Jun-2026
  y vencimiento 19-Jul-2026 → solo 29 días). La regla exacta que cumple los 3
  ejemplos es **emisión = día 20 del mes anterior al vencimiento**, es decir
  `IssueDate = dueDate - 1 mes + 1 día` (vencimiento 2026-09-19 → emisión
  2026-08-20, que coincide con el `startDateTime` del `billingPeriod`).
- **ENACAL**: `Emision` y `Vence` ya están parseados en `Raw` ("emision",
  "vence") y `DueDate` usa `Vence`. Formato `DD/MM/YYYY` → normalizar a
  `YYYY-MM-DD`.
- **DISNORTE**: la API devuelve `FechaVencimiento` en `0001-01-01T00:00:00`
  (fecha cero de .NET): NO es un dato real y **no debe enviarse** tal cual.
  `FechaRecibo` es la fecha real de emisión del recibo y ya se usa como `DueDate`
  (workaround actual); con esta spec pasa a `IssueDate`. Para el vencimiento, el
  usuario confirmó la **regla de negocio del proveedor**: la fecha de vencimiento
  es **20 días después** de la fecha del recibo (ej. recibo `19/11/2025` →
  vence `09/12/2025`). El plugin calcula `DueDate = FechaRecibo + 20 días`.
- **ASSA**: las polifacturas (pendientes) tienen `FECHA` (emisión) y
  `VENCIMIENTO` (vencimiento) reales. Los recibos pagados viven en el grid
  `grd_Pagos` (tab "Cobros"): el HTML del portal mapea la columna **"Fecha de
  Vencimiento" a `dataIndex: MES_PAGADO`** (verificado con la tabla renderizada:
  recibo 5256918 pagado el 03/08/2026 con vencimiento 02/08/2026 = `MES_PAGADO`).
  Por lo tanto los recibos sí tienen `due_date` = `MES_PAGADO` (dato directo,
  ya disponible en `Raw["mesPagado"]`; hoy el plugin usa `FECCOBRO` como DueDate,
  lo cual es incorrecto según el propio portal). No hay fecha de emisión para
  recibos; el usuario definió la **regla de negocio: emisión = vencimiento**
  (`issue_date = MES_PAGADO`) — el recibo se emite el mismo día que vence.

### 3.3 Opciones evaluadas

| Opción | Pros | Contras | Decisión |
|--------|------|---------|----------|
| A: Campos explícitos `IssueDate`/`DueDate` en el contrato + normalización por plugin | Contrato claro, tests directos, tolerancia por plugin | Cambio de contrato en repo privado (requiere tag semver) | ✅ Seleccionada |
| B: Solo extraer de `Raw` en este repo (sin tocar contrato) | No toca repo privado | Lógica de proveedor filtrándose al cliente; formatos variados difíciles de normalizar sin know-how del proveedor; duplica la regla de "dominios/know-how en el privado" (SPEC-015) | ❌ Rechazada |
| C: Enviar siempre `issue_date`/`due_date` con valor vacío cuando no hay dato | Payload uniforme | Viola la regla del usuario ("si no lo tienen, no lo enviamos") y el contrato aditivo; p40la-ihost podría sobrescribir con vacíos | ❌ Rechazada |

### 3.4 Decisiones arquitectónicas (ADRs)

**ADR-016-001**: El know-how de qué campo expone cada API vive en el repo
privado (SPEC-015).
- **Contexto**: El contrato del webhook (issue_date/due_date) se documenta en
  p40la-ihost; los plugins del repo privado son los únicos que conocen los
  nombres reales de campos de cada proveedor.
- **Decisión**: El contrato `plugins.Bill` gana el campo `IssueDate`; cada
  plugin normaliza a `YYYY-MM-DD` con su propio conocimiento del formato. Este
  repo solo mapea `IssueDate`/`DueDate` → payload.
- **Consecuencias**: Tag semver nuevo en el repo privado + `go get` en este
  repo. El repo público no gana lógica de fechas por proveedor.

**ADR-016-002**: Campos ausentes → no enviar (nunca placeholder).
- **Contexto**: DISNORTE devuelve `0001-01-01T00:00:00` como vencimiento; Tigo
  no expone emisión; ASSA recibos no tienen fechas.
- **Decisión**: Solo se envían fechas reales parseables; campos vacíos o
  placeholder se omiten del payload JSON (`omitempty` + validación de formato).
- **Consecuencias**: La app destino conserva valores previos en actualizaciones
  (semántica aditiva de SPEC-081); no hay datos falsos en SQLite.

## 4. Diseño Técnico

### 4.1 Diagrama de arquitectura

```
[Plugin claro/tigo/enacal/disnorte/assa]  --FetchBills-->  []plugins.Bill{Period, Amount, DueDate, IssueDate, Status, Raw}
        |
        v
[este repo: BillsService.FetchBills]  -->  BillPayload{Period, Amount, DueDate, IssueDate, Status, InvoiceNumber, Raw}
        |
        v
[WebhookService.buildWebhookPayload]  -->  {year, month, amount, status, invoice_number?, issue_date?, due_date?}
        |
        v
[POST /webhooks/{uuid} (p40la-ihost)]  --SQLite-->  bills (issue_date, due_date) [SPEC-081 ya en prod]
```

### 4.2 Componentes

#### 4.2.1 Contrato `plugins.Bill` (repo privado)
- **Responsabilidad**: Definir los campos que todo plugin debe poblar.
- **Interfaz**: Se agrega `IssueDate string json:"issue_date"` junto al
  `DueDate` existente.
- **Dependencias**: Ninguna.
- **Ubicación**: `p40la-ihost-automation-plugins/plugins/types.go`.

#### 4.2.2 Cada plugin (repo privado)
- **Responsabilidad**: Poblar `IssueDate`/`DueDate` con fechas reales
  normalizadas a `YYYY-MM-DD` (reutilizar `dateToISO`/`dateOnly` existentes).
- **Cambios por plugin**:
  - claro: `IssueDate = fechaEmision` (ya es el `Period`; normalizar).
  - tigo: `IssueDate = DueDate - 1 mes + 1 día` (regla de negocio del proveedor
    validada con 3 ejemplos reales: emisión = día 20 del mes anterior al
    vencimiento; no usar `-30 días` simple, falla con meses de 30 días);
    `DueDate` ya ok.
  - enacal: `IssueDate = dateToISO(Emision)`.
  - disnorte: `IssueDate = dateOnly(FechaRecibo)`; `DueDate = FechaRecibo + 20
    días` (regla de negocio del proveedor, confirmada por el usuario; hoy usa
    FechaRecibo como workaround, lo cual quedaba corto por 20 días).
  - assa: polifacturas → `IssueDate = dateOnly(FECHA)`,
    `DueDate = dateOnly(VENCIMIENTO)`; recibos → `IssueDate = dateOnly(MES_PAGADO)`
    (regla del usuario: emisión = vencimiento) y
    `DueDate = dateOnly(MES_PAGADO)` (la "Fecha de Vencimiento" del portal;
    corrige el uso actual de `FECCOBRO`).
- **Ubicación**: cada `internal/plugins/<proveedor>/<pais>/nicaragua.go` (en el
  repo privado: `claro|tigo|enacal|disnorte|assa/.../nicaragua.go`).

#### 4.2.3 `BillPayload` y `buildWebhookPayload` (este repo)
- **Responsabilidad**: Adaptar el bill del plugin al contrato del webhook.
- **Interfaz**: `BillPayload` agrega `IssueDate string json:"issue_date"`.
  `buildWebhookPayload` agrega `issue_date` y `due_date` solo si el valor es
  una fecha `YYYY-MM-DD` válida (función nueva `validISODate` o reutilizar
  `time.Parse("2006-01-02", ...)`); ambos con `omitempty`/condicional.
- **Ubicación**: `internal/services/bills.go` y `internal/services/webhook.go`.

### 4.3 Modelo de datos

```
Entidad: BillPayload (este repo, sin persistencia nueva)
- Period: string          (ya existe)
- Amount: string          (ya existe)
- DueDate: string         (ya existe; ahora se envía como due_date)
- IssueDate: string       (NUEVO; se envía como issue_date si es fecha válida)
- Status: string          (ya existe)
- InvoiceNumber: string   (ya existe)
- Raw: map[string]any     (ya existe; incluye las claves originales del proveedor)

Entidad: plugins.Bill (repo privado, contrato)
- IssueDate: string  (NUEVO, json:"issue_date"; vacío si no disponible)
```

No hay migraciones de SQLite en este repo: `bill_records.raw` ya guarda el JSON
completo y el webhook log guarda el payload enviado.

### 4.4 APIs / Contratos

#### Contrato: `POST /webhooks/{uuid}` (p40la-ihost, no cambia)

**Request (nuevo, opcional)**:
```json
{
  "year": 2026,
  "month": 9,
  "amount": 520,
  "invoice_number": "INV-2026-09",
  "issue_date": "2026-09-10",
  "due_date": "2026-10-05"
}
```

**Regla**: `issue_date`/`due_date` solo si el plugin los obtuvo y son
`YYYY-MM-DD` válidos. Ej. factura de Tigo → sin `issue_date`; recibo de ASSA
→ sin ninguno.

### 4.5 Dependencias

- **Internas**: `internal/services/webhook.go` (`buildWebhookPayload`),
  `internal/services/bills.go` (`BillPayload`), `internal/plugins` (re-export
  del contrato).
- **Externas**: módulo privado `p40la-ihost-automation-plugins` (nuevo tag
  semver con el campo `IssueDate` + cambios por plugin). Sin librerías nuevas.

## 5. Criterios de Aceptación

### 5.1 Funcionales

- [ ] CA-001: Dado un bill de Claro con `fechaEmision`/`fechaVencimiento`,
  cuando `buildWebhookPayload` genera el payload, entonces incluye
  `issue_date` y `due_date` en `YYYY-MM-DD`.
- [ ] CA-002: Dado un bill de ENACAL con `Emision`/`Vence` en `DD/MM/YYYY`,
  cuando se genera el payload, entonces incluye `issue_date`/`due_date`
  normalizados a `YYYY-MM-DD`.
- [ ] CA-003: Dado un bill de DISNORTE con `FechaRecibo` = `2025-11-19`, cuando
  se genera el payload, entonces incluye `issue_date` = `2025-11-19` y
  `due_date` = `2025-12-09` (fecha del recibo + 20 días).
- [ ] CA-004: Dado un bill de Tigo con `dueDate` = `2026-09-19`, cuando se genera
  el payload, entonces incluye `due_date` = `2026-09-19` e `issue_date` =
  `2026-08-20` (día 20 del mes anterior, no `-30 días` simple que daría
  `2026-08-20` para este caso pero falla en meses de 30 días, ej. due
  `2026-07-19` → issue `2026-06-20`).
- [ ] CA-005: Dado un recibo pagado de ASSA con `MES_PAGADO` = `2026-08-02` y
  `FECCOBRO` = `2026-08-03`, cuando se genera el payload, entonces incluye
  `due_date` = `2026-08-02` (MES_PAGADO, la "Fecha de Vencimiento" del portal)
  e `issue_date` = `2026-08-02` (regla: emisión = vencimiento); dada una
  polifactura ASSA, incluye `issue_date` = `FECHA` y `due_date` = `VENCIMIENTO`.
- [ ] CA-006: Dado un bill sin fechas o con placeholder (`0001-01-01...`),
  cuando se genera el payload, entonces el campo correspondiente se omite
  (nunca un string vacío ni fecha cero).
- [ ] CA-007: Dado un `Raw` con fecha en formato inválido, cuando se genera el
  payload, entonces no falla el job: omite el campo y sigue con los demás.
- [ ] CA-008: El payload enviado se registra completo en `webhook_logs`
  (incluyendo `issue_date`/`due_date` cuando se envían).

### 5.2 No funcionales

- [ ] CA-NF-001: Cero dependencias nuevas y cero migraciones en este repo.
- [ ] CA-NF-002: `go build ./... && go test ./...` OK en ambos repos.

### 5.3 Testing

- **Unit tests** (este repo): `buildWebhookPayload` con bills de cada plugin
  (con y sin fechas, con placeholder, con formato inválido).
- **Unit tests** (repo privado): cada plugin devuelve `IssueDate`/`DueDate`
  correctos según su fixture real (claro con `fechaEmision`, enacal con
  `Emision` DD/MM/YYYY, disnorte con +20 días sobre `FechaRecibo`, tigo sin `issue_date`, assa
  polifactura vs recibo).
- **Integration tests**: job completo de una cuenta Claro/ENACAL real enviando
  al webhook de p40la-ihost y verificando que la factura queda con
  `issue_date`/`due_date` en SQLite.
- **E2E tests**: desde la UI, ejecutar fetch de una cuenta y verificar los
  campos en el historial/payload.

## 6. Plan de Implementación

### 6.1 Fases

| Fase | Descripción | Estimación | Dependencias |
|------|-------------|------------|--------------|
| 1 | Repo privado: agregar `IssueDate` al contrato + poblar en los 5 plugins con sus fixtures de test | 0.5 día | Ninguna |
| 2 | Repo privado: tests + tag `v1.1.0` (o versión siguiente) | 0.25 día | Fase 1 |
| 3 | Este repo: `go get ...-plugins@vX.Y.Z` + `BillPayload.IssueDate` + `buildWebhookPayload` con `issue_date`/`due_date` condicionales | 0.25 día | Fase 2 |
| 4 | Este repo: tests + verificación `go build`/`go test`, `strings` sin dominios | 0.25 día | Fase 3 |
| 5 | Verificación end-to-end contra p40la-ihost real (1 cuenta Claro + 1 ENACAL) | 0.5 día | Fase 4 |

### 6.2 Milestones

1. **MVP**: Payload envía `issue_date`/`due_date` cuando la API los expone
   (fases 1-4).
2. **V1.0**: Verificación end-to-end con cuentas reales y revisión en la UI de
   p40la-ihost (fase 5).

## 7. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| La API de Tigo sí expone emisión con otro nombre y no se detecta | Media | Bajo | Los campos son opcionales; si aparece la captura, se mapea en una iteración (P2). No bloquea el release. |
| `MES_PAGADO` de ASSA no es el vencimiento en todos los casos (el portal lo etiqueta como "Fecha de Vencimiento", pero su nombre sugiere "mes cubierto por el pago") | Media | Medio | Validar contra el recibo PDF real (se puede descargar vía `cmd="recibo"`) antes del release; si difiere, se documenta y se ajusta. |
| La regla "emisión = vencimiento" de ASSA recibos no es exacta (el recibo podría emitirse antes del vencimiento) | Baja | Bajo | Regla definida por el usuario; si el PDF del recibo muestra una fecha de emisión distinta, se ajusta en iteración. |
| Regla de negocio de Tigo (`dueDate - 1 mes + 1 día`) deja de ser exacta (ej. el proveedor cambia el día de emisión o el ciclo) | Baja | Bajo | La regla está validada con 3 ejemplos reales; si cambia, se ajusta en una iteración. El plugin no debe usar `-30 días` simple (falla con meses de 30 días). |
| `FechaVencimiento` de DISNORTE deja de venir en cero y pasa a ser real | Media | Medio | El plugin solo envía el campo si parsea una fecha válida (no la fecha cero). Si la API empieza a exponerla real, se prefiere el campo de la API sobre la regla de +20 días (iteración). |
| Formato de fecha cambia en alguna API (ej. ENACAL pasa a ISO) | Baja | Bajo | Normalización tolerante por plugin (REQ-006): si no parsea, no envía; no falla el job. |
| El webhook de p40la-ihost rechaza campos nuevos | Muy baja | Alto | Verificar contra el doc v1.2/SPEC-081 (ya released): los campos son opcionales y el handler los acepta. |

## 8. Notas y Referencias

- Contrato del webhook: https://github.com/paulomcnally/p40la-ihost/blob/main/docs/webhooks-api.md (v1.2, campos `issue_date`/`due_date`, SPEC-081).
- SPEC-081 en p40la-ihost: columnas `issue_date`/`due_date` en `bills` (migración 0030).
- Repo privado de plugins: `github.com/paulomcnally/p40la-ihost-automation-plugins` (SPEC-015).
- Este repo: `internal/services/webhook.go:224` (`buildWebhookPayload`), `internal/services/bills.go:22` (`BillPayload`).

## 9. Historial de Cambios

| Fecha | Autor | Descripción |
|-------|-------|-------------|
| 2026-09-19 | paulomcnally | Creación inicial de la especificación |
| 2026-09-19 | paulomcnally | DISNORTE: el usuario confirmó la regla de negocio del proveedor — el vencimiento es **FechaRecibo + 20 días** (ej. recibo 19/11/2025 → vence 09/12/2025). Se actualiza matriz, diseño (plugin calcula DueDate), CA-003 y riesgos |
| 2026-09-19 | paulomcnally | TIGO: el usuario confirmó la regla de negocio del proveedor — ciclo de facturación de 30 días; emisión = **día 20 del mes anterior al vencimiento** (`dueDate - 1 mes + 1 día`). Validado con 3 ejemplos reales (MSDFC-30999382/31182590/31369430): `-30 días` simple falla en meses de 30 días (ej. 20-Jun → 19-Jul = 29 días). Se actualiza matriz, diseño, CA-004 y riesgos |
| 2026-09-19 | paulomcnally | ASSA recibos: verificado con el HTML real del portal (grid `grd_Pagos`, tab "Cobros") que la columna "Fecha de Vencimiento" mapea a `dataIndex: MES_PAGADO` — los recibos SÍ tienen `due_date` = `MES_PAGADO` (dato directo; corrige el `FECCOBRO` usado hoy). El usuario definió la regla de negocio: **emisión = vencimiento** (`issue_date` = `MES_PAGADO`). Se actualiza matriz, diseño, CA-005 y riesgos |
| 2026-09-19 | paulomcnally | Inicio de desarrollo: estado draft → in_progress (implementación de los 5 plugins en repo privado + payload en este repo) |
| 2026-09-19 | paulomcnally | Desarrollo completado: repo privado v1.1.0 (contrato `IssueDate` + reglas Tigo/DISNORTE/ASSA + tests), este repo (`BillPayload.IssueDate` + `buildWebhookPayload` con `issue_date`/`due_date` condicionales + tests CA-001..CA-007). Verificado E2E contra las APIs reales de los 5 plugins: claro (issue 2026-06-22/due 2026-07-16), tigo (issue 2026-08-20/due 2026-09-19, incluido el caso crítico 20-Jun→19-Jul), enacal (issue 2026-08-29/due 2026-09-20), disnorte (issue 2024-05-20/due 2024-06-09 = +20 días), assa (polifactura issue 2026-03-31/due 2026-10-02; recibo issue=due 2026-09-02). `go build`/`go test` OK en ambos repos; `strings` sin dominios en claro. Estado → pending_release |
| 2026-09-19 | paulomcnally | **Released**: commit en este repo (SPEC-016: release) subido a `main`. Repo privado `p40la-ihost-automation-plugins` v1.1.0 (commit `d1feef6`) ya en main con tag. Deploy a iHost pendiente de decisión del usuario |
