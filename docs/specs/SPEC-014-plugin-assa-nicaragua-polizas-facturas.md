---
title: "Plugin assa.nicaragua: autenticación, pólizas y facturas"
id: "SPEC-014"
status: "released"
author: "paulomcnally"
created: "2026-09-14"
updated: "2026-09-15"
github_issue: 15
---

# Plugin assa.nicaragua: autenticación, pólizas y facturas

**ID**: SPEC-014  
**Estado**: released  
**Autor**: paulomcnally  
**Creado**: 2026-09-14  
**Actualizado**: 2026-09-15

---

## 1. Resumen Ejecutivo

Se necesita un plugin de integración llamado `assa.nicaragua` que consulte las
**pólizas de seguro** y sus **facturas (primas) pendientes** de **ASSA Compañía de
Seguros, S.A.** (Nicaragua) usando el portal `https://<DOMINIO_ASSA>` (ASP.NET
WebForms + Ext.NET, captura real de Charles Proxy en
`/home/paulomcnally/Downloads/mi_assa_net.chlz`). El usuario capturó una sesión real
del portal (iPhone Safari, 2026-09-14) que incluye el tráfico de **autenticación**
(`POST /ni/miassanet-NI/login_v.aspx` con `txt_Usuario` + `txt_Clave`) y el tráfico de
**pólizas/facturas** (`Consulta_Polizas.aspx` y `Unidades.aspx`).

El flujo de autenticación es **sesión de cookies ASP.NET**: una primera request al
portal fija `ASP.NET_SessionId`, `__Secure-SessionId` y `__VerificationToken`; el login
`POST /ni/miassanet-NI/login_v.aspx` envía los campos del formulario WebForms
(`__VIEWSTATE`, `__EVENTVALIDATION`, `__RequestVerificationToken`, `tokenCaptcha`,
`txt_Usuario`, `txt_Clave`, `TipoDispotivivo`) y responde `302 Found` con
`Set-Cookie: .ASPXAUTH=<ticket>` y `Location: Default_p.aspx?var=<VAR>&Tipo=M`. El
parámetro `var` (ej. `QCBKDORFFTUIUYRYUZQRLSUVYRGOXVDN`) es el token de sesión que
acompaña **todas** las consultas posteriores.

**Riesgo principal de la captura — reCAPTCHA v3 en el login**: el formulario exige
un `tokenCaptcha` de reCAPTCHA v3 (site key `6LfX6q4rAAAAALhMI8kSeDpKIagKrgXzvmQX47cD`).
Un análisis inicial sugirió que el token enviado era idéntico al valor pre-renderizado
del HTML (lo que permitiría login sin JS), pero esa comparación se hizo contra el
**cuerpo del 302** (que el servidor re-renderiza con un **eco de los valores
enviados**), no contra la página GET. La evidencia real de la captura demuestra que
el navegador **sí ejecutó reCAPTCHA**: hay un `POST /recaptcha/api2/reload` a Google
que terminó 4 ms antes del POST de login. **El plugin en Go no puede generar tokens
de reCAPTCHA v3** (requiere JS del navegador). La viabilidad depende de si el servidor
valida el token con Google (siteverify) o solo exige el campo no vacío; esto se
verifica en una **Fase 0** de prueba (enviar el `tokenCaptcha` extraído del HTML de
una página fresca) antes de implementar el resto. **Fase 0 ejecutada (2026-09-14,
prueba real con curl + credenciales de la captura): LOGIN SIN CAPTCHA VIABLE** — el
formulario de login exige `tokenCaptcha` solo cuando el POST incluye el campo vacío
(*"Por favor complete el CAPTCHA para continuar"*) o el postback WebForms completo;
pero un **POST directo sin `tokenCaptcha`** (campos `txt_Usuario` + `txt_Clave` +
`botonLoginAux` + `TipoDispotivivo`) responde **302 + `Set-Cookie: .ASPXAUTH` + `var`**
y la sesión queda **completa** (el asegurado se puebla en `Consulta_Polizas.aspx`).
El formulario público de `<DOMINIO_ASSA_WEB>` (`txt_usuario`/`txt_contrasena`) también
autentica, pero la sesión queda **sin contexto de asegurado** (pólizas vacías) — usar
los campos móviles. Las pólizas y polifacturas se leen con `Tipo=` **vacío** en la URL
(el grid viene embebido en el HTML; con `Tipo=M`/`Tipo=D` el sitio lo carga por
DirectMethod de Ext.NET y queda vacío sin AJAX). **La integración es viable en Go
puro, sin captcha y sin AJAX** (ver §7 y §6.1).

Las facturas se leen de `GET /ni/sca/Unidades.aspx?var=<VAR>&pol=<POLIZA>&ipol=<IDEPOL>`:
la respuesta HTML embebe grids de Ext.NET en `proxy:{data:[...]}` con los recibos
(`NUMRECIBO`, `MES_PAGADO`, `MONTO`), las **polifacturas** (`POLIFACTURA`,
`FECHA`, `VENCIMIENTO`, `MONTO`, `STATUS: PENDIENTE`) — las facturas/primas a enviar
al webhook — y el estado de cuenta (`PORVENCER`, `CORRIENTE`, `DIAS01_30`...).
El listado de pólizas se obtiene de `Consulta_Polizas.aspx` (grid con `IDEPOL`,
`POLIZA`, `RAMODECLARATIVO`, `DESCRIPCION`, `PRIMAFACTURADA`, `CODMONEDA`, ...).

Este plugin sigue el contrato de infraestructura de SPEC-003/007/009/012 (interfaz
`Plugin`, registro, tabla `bills`, asociación `accounts.plugin_name`, job/webhook
existente de SPEC-004/008/010). **No requiere cambios de esquema.** El resultado
esperado: plugin `assa.nicaragua` registrado, `FetchBills` autentica con
`username`+`password` del portal (credenciales de la cuenta), lista las pólizas del
asegurado, filtra la póliza del `identifier`, consulta sus polifacturas pendientes y
normaliza cada una a `[]plugins.Bill` con periodo `YYYY-MM` (de `FECHA`), monto real
(`MONTO`), vencimiento (`VENCIMIENTO`) y `invoice_number` = `POLIFACTURA`. El
job/webhook existente reenvía las facturas con el payload correcto (`year`, `month`,
`amount`, `status`, `invoice_number`).

**Importante sobre credenciales**: la captura expone el usuario
(`<CEDULA>`, cédula del asegurado) y las pólizas del usuario (aquí anonimizadas)
(`<POLIZA>` AUTOMÓVIL, `<POLIZA2>` AUTOMÓVIL). **No deben
hardcodearse en el código.** Se usan únicamente para poblar `accounts.credentials` en
la base SQLite local de desarrollo (vía `seed-apps.sh` con variables de entorno), y el
plugin los lee de ahí en runtime. Nada se persiste ni se loguea.

## 2. Requerimientos

### 2.1 Requerimientos Funcionales (P0 - Obligatorios)

1. **REQ-001**: Plugin `assa.nicaragua` que implementa la interfaz `Plugin` existente (`Name`, `Version`, `Source`, `Description`, `CredentialSchema`, `FetchBills`), se registra en `main.go` y aparece en el catálogo `plugins` vía `SyncCatalog` (sin migración nueva).
2. **REQ-002**: Sesión de login replicando la captura (**condicionado al resultado de la Fase 0 — validación del `tokenCaptcha`**):
   - `GET https://<DOMINIO_ASSA>/ni/miassanet-NI/login_v.aspx` → 200 HTML con el formulario WebForms: `__RequestVerificationToken`, `__VIEWSTATE`, `__VIEWSTATEGENERATOR`, `__EVENTVALIDATION`, `tokenCaptcha`, `__VerificationToken`/`ASP.NET_SessionId`/`__Secure-SessionId` en cookies.
   - `POST https://<DOMINIO_ASSA>/ni/miassanet-NI/login_v.aspx` (form-urlencoded) con esos campos + `txt_Usuario=<USER>`, `txt_Clave=<PASS>`, `botonLoginAux=Iniciar sesión`, `TipoDispotivivo=M` → 302 con `Set-Cookie: .ASPXAUTH` y `Location: /ni/miassanet-NI/Default_p.aspx?var=<VAR>&Tipo=M`.
   - El `var` de la `Location` y las cookies se conservan en el jar del cliente para las consultas siguientes.
   - **Fase 0 (obligatoria, previa a la implementación)**: verificar si el servidor acepta el `tokenCaptcha` extraído del HTML sin ejecutar el JS de reCAPTCHA v3 (ver ADR-002 y §7). Si no lo acepta, el login no es reproducible en Go puro y la integración queda bloqueada.
3. **REQ-003**: Listado de pólizas: `GET /ni/sca/Consulta_Polizas.aspx?var=<VAR>&Tipo=` (**`Tipo` vacío es clave**: con `Tipo=M`/`Tipo=D` el grid viene vacío y se carga por DirectMethod) → HTML con grid Ext.NET embebido `proxy:{data:[...]}`: `IDEPOL`, `POLIZA`, `CODPROD`, `RAMODECLARATIVO`, `FECINIVIG`, `FECFINVIG`, `PRODUCTO`, `DESCRIPCION`, `PRIMAFACTURADA`, `TCORREDOR`, `CODMONEDA`.
4. **REQ-004**: Detalle de póliza: `GET /ni/sca/Unidades.aspx?var=<VAR>&pol=<POLIZA>&ipol=<IDEPOL>` (sin `Tipo`) → HTML con grids embebidos: unidades/titulares (`IDEPOL`, `UNIDAD`, `TITULAR`, `STATUS`, `ACREEDOR`), recibos (`NUMRECIBO`, `FECCOBRO`, `MES_PAGADO`, `MONTO`, `PREIMPRESO`), **polifacturas** (`POLIFACTURA`, `FECHA`, `VENCIMIENTO`, `MONTO`, `STATUS`) y estado de cuenta (`PORVENCER`, `CORRIENTE`, `DIAS01_30`...).
5. **REQ-005**: Normalización a `[]plugins.Bill` con **toda la data de facturación de la póliza del `identifier`**: polifacturas pendientes (grid del portal) + recibos pagados (historial). Para las polifacturas:
   - `Period`: `YYYY-MM` derivado de `FECHA` (`"2026-03-31T00:00:00"` → `"2026-03"`), compatible con `parseBillPeriod` (SPEC-008).
   - `Amount`: `MONTO` formateado sin ceros de más (ej. `"54.63"`).
   - `DueDate`: `VENCIMIENTO` → `YYYY-MM-DD` (`"2026-10-02T00:00:00"` → `"2026-10-02"`).
   - `Status`: `pending` para `STATUS: PENDIENTE` (las polifacturas pendientes de prima); `paid` para `PAGADA` si aparecieran.
   - `Raw`: debe incluir `invoice_number` = `POLIFACTURA` (ej. `<NUM_POLIFACTURA>`, clave leída por `buildWebhookPayload`) y opcionalmente `fecha`, `vencimiento`, `monto`, `poliza`, `idepol`, `descripcion`, `moneda`, `recibos`, `estadoCuenta`.
   - **Recibos pagados** (grid `NUMRECIBO`): `Period` = `YYYY-MM` de `MES_PAGADO`, `Amount` = `MONTO`, `DueDate` = `FECCOBRO` (fecha de cobro), `Status` = `paid`, `Raw["invoice_number"]` = `NUMRECIBO` + `preimpreso`, `fecCobro`, `mesPagado`, `poliza`, `idepol`, `moneda`, `descripcion`.
6. **REQ-006**: `invoice_number` en el webhook: `Raw["invoice_number"]` = `POLIFACTURA` → el `buildWebhookPayload` existente emite `invoice_number` correcto. Payload final `{year, month, amount, status, invoice_number}` según `docs/webhooks-api.md`.
7. **REQ-007**: Seed local para pruebas: `scripts/seed-apps.sh` soporta el plugin ASSA (`PLUGIN_NAME=assa.nicaragua`, `IDENTIFIERS=<POLIZA>`, `CREDENTIALS='{"username":"...","password":"..."}'`) y descripción por defecto. Credenciales reales vía entorno, nunca hardcodeadas. Para la prueba local se configura la cuenta real del usuario en SQLite.

### 2.2 Requerimientos Funcionales (P1 - Importantes)

1. **REQ-008**: Manejo de errores estructurado mapeado a los errores del dominio (`ErrAuthFailed`, `ErrUpstream`, `ErrAPISchemaChanged`, `ErrAuthExpired`), persistidos como `bills` con `status: error` y `plugin_version`, sin exponer credenciales. Casos: login rechazado (el HTML re-renderiza el formulario con `lblerror`/`lblLogin`), token `var` expirado (timeout de sesión 20 min), grid ausente (cambio de schema).
2. **REQ-009**: `CredentialSchema` (SPEC-011): `username` (text, required, no secret) y `password` (password, required, secret), para el formulario dinámico.
3. **REQ-010**: El `identifier` de la cuenta es el número de **póliza** (`POLIZA`, ej. `<POLIZA>`). `FetchBills` filtra la póliza del listado de `Consulta_Polizas.aspx` (si no aparece → `ErrServiceNotFound`). El valor `var` de sesión no se persiste entre consultas (cada `FetchBills` re-autentica).

### 2.3 Requerimientos Funcionales (P2 - Deseables)

1. **REQ-011**: "Pagos en Línea" (`GET /ni/pagos/prima/pagosPendientesPrima.aspx?var=<VAR>&Tipo=M`) para consultar primas pendientes de pago como fuente alternativa/adicional (visible en el menú del portal, no capturado en esta sesión).
2. **REQ-012**: Parser de grid Ext.NET reutilizable (`proxy:{data:[...]}`) como helper dentro del plugin, y reportar los recibos pagados (`NUMRECIBO`/`PREIMPRESO`) como historial en `Raw` (P2; el webhook solo envía pendientes).
3. **REQ-013**: Configuración `TipoDispotivivo` variable (`M`/`D`/`T`) y `Tipo=M` en query para replicar distintos dispositivos si el portal lo exige.

### 2.4 Requerimientos No Funcionales

- **Rendimiento**: Timeouts obligatorios (≤ 15 s) en llamadas HTTP salientes. Una consulta completa (login + 2 páginas) < 5 s.
- **Seguridad**: `username`/`password` jamás se exponen en respuestas de API ni en `raw`; viven solo en el JSON de credenciales de la cuenta y en memoria durante la consulta. No hardcodear credenciales en el código. El ticket `.ASPXAUTH` y el `var` solo viven en memoria.
- **Almacenamiento**: Reusa la tabla `bills`; cero migraciones nuevas.
- **Disponibilidad**: Todas las rutas detrás de `authMiddleware`; `/health` sin cambios.
- **iHost**: Cero dependencias nuevas (Go stdlib `net/http`). Parsing de HTML con regexp sobre `proxy:{data:[...]}` + `encoding/json` (stdlib). Sin persistencia de cookies en disco.

## 3. Investigación y Decisiones Técnicas

### 3.1 Contexto investigado (captura Charles Proxy real, 2026-09-14)

Se capturó una sesión real del portal ASSA Nicaragua (`https://<DOMINIO_ASSA>`)
desde iPhone Safari mediante Charles Proxy (`/home/paulomcnally/Downloads/mi_assa_net.chlz`).
Backend **ASP.NET WebForms (.NET)** + **Ext.NET** (grids `proxy:{data:[...]}`). Flujo
verificado en el orden del portal:

- **`GET /ni/miassanet-NI/login_v.aspx`** (implícito en la captura; el formulario
  aparece en el cuerpo del 302) → formulario con:
  - `__RequestVerificationToken` = `YWaKVun4AuoOwfHREapcTYE5QW1mBSSZibv4w0GE/o33NXq78G8gTg==`
    (también como cookie `__VerificationToken`).
  - `__VIEWSTATE`, `__VIEWSTATEGENERATOR` (`D5F24D1C`), `__EVENTVALIDATION`.
  - `tokenCaptcha` con `value` que la captura muestra poblado (2425 chars) — **pero
    el GET de la página no fue capturado**: el valor visto está en el cuerpo del 302,
    que el servidor re-renderiza con un **eco de los valores enviados** en el POST.
  - JS de login: `obtenerCaptcha()` — con `'S' === 'N'` (captcha desactivado) usa el
    `token` del servidor; en la captura real la rama es falsa y se ejecuta
    `grecaptcha.execute('6LfX6q4rAAAAALhMI8kSeDpKIagKrgXzvmQX47cD', {action:'submit'})`
    cuyo callback pisa `tokenCaptcha.value` y hace click en `botonLoginAux`. **No hay
    `var token` definida en la página** (la rama del servidor no aplica).
  - Evidencia del token fresco: `POST www.google.com/recaptcha/api2/reload?k=6LfX...`
    (entrada 36 de la captura, terminó `23:24:56.830`) seguido 4 ms después por el
    POST de login (entrada 38, inició `23:24:56.829`). El navegador generó el token
    vía JS de Google y lo envió; el valor en el cuerpo del 302 es el mismo token
    (eco del formulario).
- **`POST /ni/miassanet-NI/login_v.aspx`** — form-urlencoded:
  `__RequestVerificationToken`, `__VIEWSTATE`, `__VIEWSTATEGENERATOR`,
  `__VIEWSTATEENCRYPTED` (vacío), `__EVENTVALIDATION`, `txt_Usuario=<CEDULA>`,
  `txt_Clave=<PASS>`, `tokenCaptcha=<token>` (fresco, generado por reCAPTCHA v3 vía
  JS del navegador), `botonLoginAux=Iniciar sesión`,
  `TipoDispotivivo=M` → **302 Found** con:
  - `Set-Cookie: .ASPXAUTH=D2C680B10ACFBFABF1C08C337C4A9EF1...; path=/; HttpOnly; SameSite=Lax`.
  - `Location: /ni/miassanet-NI/Default_p.aspx?var=QCBKDORFFTUIUYRYUZQRLSUVYRGOXVDN&Tipo=M`.
  - Headers: `Referer` = login, `Origin` = `https://<DOMINIO_ASSA>`, UA iPhone.
- **`GET /ni/miassanet-NI/Default_p.aspx?var=<VAR>&Tipo=M`** → dashboard (bienvenida).
- **`GET /ni/sca/Consulta_Polizas.aspx?var=<VAR>&Tipo=M`** → "Consulta de Pólizas":
  datos del asegurado (`<NOMBRE_ASEGURADO>`, cédula `<CEDULA_SIN_GUIONES>`,
  celular `<CELULAR>`) + grid `Poliza` con 2 pólizas:
  - `{IDEPOL: <IDEPOL>, POLIZA: "<POLIZA>", RAMODECLARATIVO: "AUTOMÓVIL",
    FECINIVIG: 2026-05-23, FECFINVIG: 2027-05-22, DESCRIPCION: "AUTOMÓVIL",
    PRIMAFACTURADA: 109.21, CODMONEDA: "U$"}`
  - `{IDEPOL: <IDEPOL2>, POLIZA: "<POLIZA2>", RAMODECLARATIVO: "AUTOMÓVIL",
    FECINIVIG: 2025-10-20, FECFINVIG: 2026-10-19, DESCRIPCION: "AUTOMÓVIL",
    PRIMAFACTURADA: 67.37, CODMONEDA: "U$"}`
- **`GET /ni/sca/Unidades.aspx?var=<VAR>&pol=<POLIZA>&ipol=<IDEPOL>`** → grids:
  - Unidades: `{IDEPOL: <IDEPOL>, UNIDAD: 1, TITULAR: "<NOMBRE_ASEGURADO>", STATUS: "ACTIVO", ACREEDOR: "<ACREEDOR>"}`
  - Recibos (4): `{NUMRECIBO: <NUM_RECIBO>, FECCOBRO: 2026-09-02, MES_PAGADO: 2026-09-02,
    MONTO: 54.63, PREIMPRESO: "129588"}` ... (mensuales, U$54.63).
  - **Polifacturas (2 pendientes)**: `{POLIFACTURA: <NUM_POLIFACTURA>, FECHA: 2026-03-31,
    VENCIMIENTO: 2026-10-02, MONTO: 54.63, STATUS: "PENDIENTE"}` y
    `{POLIFACTURA: <NUM_POLIFACTURA>, FECHA: 2026-03-31, VENCIMIENTO: 2026-11-02, MONTO: 54.58,
    STATUS: "PENDIENTE"}`.
  - Estado de cuenta: `{PORVENCER: 54.58, CORRIENTE: 54.63, ...}`.
- **`GET /ni/sca/Unidades.aspx?var=<VAR>&pol=<POLIZA2>&ipol=<IDEPOL2>`** → póliza
  RENOVADA (2025-10-20 → 2026-10-19), 12 recibos (U$67.39), 1 polifactura pendiente:
  `{POLIFACTURA: <NUM_POLIFACTURA>, FECHA: 2025-10-20, VENCIMIENTO: 2026-09-30, MONTO: 67.37,
  STATUS: "PENDIENTE"}`, balance `{PORVENCER: 0, CORRIENTE: 67.37, ...}`.

**Cookies del flujo** (request típico autenticado): `ASP.NET_SessionId`,
`.ASPXAUTH=<ticket>`, `__Secure-SessionId`, `__VerificationToken`, más analíticas
(`_fbp`, `_ga`, `_gcl_au`) que no son necesarias.

**Headers típicos de request** (verificados): `Host: <DOMINIO_ASSA>`,
`User-Agent: Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) AppleWebKit/605.1.15
(KHTML, like Gecko) Version/26.6 Mobile/15E148 Safari/604.1`,
`Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8`,
`Accept-Language: es-419,es;q=0.9`, `Referer` = página anterior del portal,
`Sec-Fetch-Site: same-origin`, `Sec-Fetch-Mode: navigate`.

**Datos de la cuenta (anonimizados)** (para seed local vía entorno, NO en código):
- `username` = `<CEDULA>` (credencial → `accounts.credentials`)
- `password` = `<PASS>` (credencial → `accounts.credentials`)
- Pólizas: `<POLIZA>` (IDEPOL <IDEPOL>, AUTOMÓVIL) y `<POLIZA2>`
  (IDEPOL <IDEPOL2>, AUTOMÓVIL)
- Facturas pendientes reales (poliza `<POLIZA>`): POLIFACTURA `<NUM_POLIFACTURA>` (U$54.63,
  vence 2026-10-02; U$54.58, vence 2026-11-02).

### 3.2 Opciones evaluadas

| Opción | Pros | Contras | Decisión |
|--------|------|---------|----------|
| Sesión de cookies ASP.NET + `var` en query (flujo real del portal) | Verificado en captura; flujo simple sin WAF JS challenge | Requiere mantener jar de cookies + re-autenticar por consulta; sesión expira a los 20 min; login exige `tokenCaptcha` de reCAPTCHA v3 | ✅ Seleccionada (flujo exacto de la captura) |
| API JSON/REST nativa | Más limpia | No existe: el portal es WebForms/Ext.NET con grids embebidos en HTML | ❌ Rechazada |
| Resolver reCAPTCHA v3 (protocolo `/recaptcha/api2/*`) | Permitiría token fresco | Complejo, frágil, anti-ToS, requiere JS del navegador (inviable en Go/iHost) | ❌ Rechazada (ver Fase 0 y §7) |
| Enviar el `tokenCaptcha` del HTML sin ejecutar JS | Sin dependencias; posible si el servidor no valida el token con Google | **No verificado**: la comparación "token == pre-renderizado" fue un artefacto del eco del 302; el navegador real generó el token vía JS | ⚠️ Hipótesis a probar en Fase 0 (si falla → bloqueo documentado) |
| Parser HTML con librería (`goquery`) | Parsing robusto | Dep nueva en iHost; el target es un patrón JSON estable `proxy:{data:[...]}` | ❌ Rechazada (regexp + json, stdlib) |
| Usar `pagosPendientesPrima.aspx` como fuente única | Datos de primas explícitos | No capturado en esta sesión; `Unidades.aspx` ya trae polifacturas + recibos + balance | ❌ Rechazada para el MVP (P2, REQ-011) |
| Migración/schema nuevo para ASSA | ... | Reusa todo lo existente | ❌ Rechazada (cero migraciones) |

### 3.3 Decisiones arquitectónicas (ADRs)

**ADR-001**: Autenticación por sesión ASP.NET (cookies + `var` en query)
- **Contexto**: La captura muestra login WebForms con `__VIEWSTATE`/`__EVENTVALIDATION`
  y respuesta 302 con `.ASPXAUTH` + `var` en la `Location`.
- **Decisión**: `FetchBills` hace `GET login_v.aspx` (obtiene form fields + cookies),
  `POST login_v.aspx` inmediatamente con esos fields (incluyendo el `tokenCaptcha`
  extraído del HTML), sigue el 302 manualmente extrayendo `var` de la `Location` y
  conserva todas las cookies en el jar. Re-autentica en cada consulta (las sesiones
  expiran a los 20 min; no se persiste el ticket).
- **Consecuencias**: Cada consulta son 2 llamadas de auth + 2 de datos (~4 requests);
  el `var` es por-sesión y no se guarda. **La viabilidad del login depende del
  resultado de la Fase 0** (ver ADR-002 y §7).

**ADR-002**: El `tokenCaptcha` del HTML se envía tal cual (hipótesis, validar en Fase 0)
- **Contexto**: El formulario exige `tokenCaptcha` de reCAPTCHA v3. Un análisis inicial
  concluyó que el POST enviaba el valor pre-renderizado del HTML; análisis posterior
  demostró que eso era un **artefacto del eco del 302** (el servidor re-renderiza el
  formulario con los valores enviados) y que el navegador generó un token fresco vía
  `grecaptcha.execute` (POST `/recaptcha/api2/reload` 4 ms antes del login). Un plugin
  Go no puede ejecutar ese JS.
- **Decisión**: **Fase 0 obligatoria** antes de implementar: `GET login_v.aspx` →
  extraer `tokenCaptcha` del HTML → `POST` de login de inmediato. Si el servidor
  acepta el token sin validarlo con Google (siteverify) → el plugin es viable y el
  `tokenCaptcha` se envía tal cual. Si lo rechaza (login falla con `lblerror`/
  `lblLogin` poblado o sin `.ASPXAUTH`) → la integración **no es viable** en Go puro:
  documentar el bloqueo (§7) y evaluar alternativas (sesión manual, sin automatizar).
- **Consecuencias**: La spec se mantiene como diseño condicionado a la Fase 0; no se
  invierte en el resto del plugin hasta validar el login.

**ADR-003**: Las credenciales (`username`, `password`) viven en `accounts.credentials`
- **Contexto**: SPEC-002 ya guarda credenciales JSON por cuenta; SPEC-011 genera el
  formulario dinámico desde `CredentialSchema`.
- **Decisión**: `creds["username"]` (obligatorio) + `creds["password"]` (obligatorio,
  `Secret: true`). Nunca hardcodear los valores reales; el seed los toma de variables
  de entorno. Para la prueba local se cargan en SQLite.
- **Consecuencias**: El plugin lee las credenciales en runtime y no expone nada.

**ADR-004**: Cero migraciones nuevas; reuso total de `plugins`/`bills`/webhook
- **Contexto**: SPEC-003/004/007/008/010/012 dejaron catálogo, tabla `bills`,
  asociación `plugin_name`, scheduler/webhook con `parseBillPeriod` tolerante y
  `extractInvoiceNumber`.
- **Decisión**: El plugin ASSA se registra en `main.go` y `SyncCatalog` lo persiste.
  Emite `Period` en `YYYY-MM` (compatible con `parseBillPeriod`) y
  `Raw["invoice_number"]` = `POLIFACTURA`.
- **Consecuencias**: Menos superficie de cambio y riesgo en iHost.

**ADR-005**: Parsing de grids Ext.NET con regexp + `encoding/json` (stdlib)
- **Contexto**: Los datos vienen embebidos en HTML como `proxy:{data:[...]}` (JSON
  válido). No hay JSON-LD ni endpoints REST.
- **Decisión**: Capturar cada bloque `proxy:{data:[<json>]}` con regexp y deserializar
  con `json.Unmarshal` a structs tipadas (`poliza`, `unidad`, `recibo`,
  `polifactura`, `balance`).
- **Consecuencias**: Sin deps nuevas; si Ext.NET cambiara el patrón, `api_changed`.

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
                                                          [internal/plugins: Registry + assa.nicaragua]
                                                                                                   |
                      GET  /ni/miassanet-NI/login_v.aspx (form fields + cookies) -----------------> <DOMINIO_ASSA>
                      POST /ni/miassanet-NI/login_v.aspx (txt_Usuario/txt_Clave/tokenCaptcha) --> (ASP.NET WebForms + Ext.NET)
                      GET  /ni/sca/Consulta_Polizas.aspx?var=<VAR>&Tipo=M (grid de pólizas) -----> |
                      GET  /ni/sca/Unidades.aspx?var=<VAR>&pol=<POLIZA>&ipol=<IDEPOL> -----------> |
                                                                                                   |
                                                                                                   v
                                                      [SQLite: plugins + accounts(plugin_name + credentials) + bills]
                                                                                                   |
                                            [Scheduler/Webhook existente (SPEC-004/008/010) reusa bills, sin cambios]
```

### 4.2 Componentes

#### 4.2.1 Plugin `assa.nicaragua` (`internal/plugins/assa/nicaragua/nicaragua.go`)
- **Responsabilidad**: Autenticar sesión ASP.NET (login + cookies + `var`), listar
  pólizas, filtrar la del `identifier`, consultar sus polifacturas y normalizarlas a `plugins.Bill`.
- **Interfaz**: Implementa `Plugin`. `FetchBills(ctx, creds, identifier)` con
  `creds = {"username": "...", "password": "..."}` e `identifier` = póliza (ej. `<POLIZA>`).
- **Dependencias**: Solo stdlib (`net/http`, `net/http/cookiejar`, `net/url`,
  `encoding/json`, `encoding/base64`, `regexp`, `strings`, `time`).
- **Ubicación**: `internal/plugins/assa/nicaragua/nicaragua.go`; registro en `cmd/server/main.go`.

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
  - account.identifier   = número de póliza (ej. "<POLIZA>")
  - account.plugin_name  = "assa.nicaragua"

plugins / bills (SPEC-003, sin cambios)
  - assa.nicaragua se registra en runtime (SyncCatalog → UpsertPlugin)
  - bills: una fila por consulta con plugin_version + source (https://<DOMINIO_ASSA>)
```

### 4.4 APIs / Contratos

#### Endpoint: `POST /api/accounts/{accountId}/bills:fetch` (existente)

Ejecuta la consulta del plugin asociado a la cuenta. Para ASSA, el plugin
(**precondición**: Fase 0 validada, ver ADR-002):
1. `GET https://<DOMINIO_ASSA>/ni/miassanet-NI/login_v.aspx` → form fields
   (`__VIEWSTATE`, `__EVENTVALIDATION`, `__RequestVerificationToken`, `tokenCaptcha`)
   + cookies de sesión (jar).
2. `POST .../login_v.aspx` form-urlencoded (incluye `tokenCaptcha` del paso 1) → 302
   con `Set-Cookie: .ASPXAUTH` y `Location: ...Default_p.aspx?var=<VAR>&Tipo=M`
   (extrae `var`). Si el servidor re-renderiza el formulario con `lblerror`/`lblLogin`
   poblado y **sin** `.ASPXAUTH` → `ErrAuthFailed`.
3. `GET .../ni/sca/Consulta_Polizas.aspx?var=<VAR>&Tipo=` (Tipo vacío) → grid de
   pólizas embebido; selecciona la póliza == `identifier` (si no existe → `ErrServiceNotFound`).
4. `GET .../ni/sca/Unidades.aspx?var=<VAR>&pol=<POLIZA>&ipol=<IDEPOL>` → grids
   (polifacturas pendientes principalmente).
5. Normaliza cada polifactura a `plugins.Bill` (REQ-005) y persiste en `bills`.

**Response 201** (igual que SPEC-003/007/009/012):
```json
{
  "id": 45, "account_id": 8, "plugin_name": "assa.nicaragua",
  "plugin_version": "1.0.0", "status": "ok",
  "bills": [
    { "period": "2026-03", "amount": "54.63", "due_date": "2026-10-02", "status": "pending" },
    { "period": "2026-03", "amount": "54.58", "due_date": "2026-11-02", "status": "pending" }
  ]
}
```

**Response Error** (códigos): `invalid_request` (400), `not_found` (404), `auth_expired` (401),
`auth_failed` (401), `upstream_error` (502), `api_changed` (502).

#### Endpoint: `GET /api/accounts/{accountId}/bills` (existente)

**Response 200**: idéntico al de SPEC-003; cada fila trae `plugin_name: "assa.nicaragua"`,
`plugin_version`, `source` y `status`.

> El `username`/`password`, el ticket `.ASPXAUTH` y el `var` NUNCA aparecen en
> respuestas ni en `raw`.

#### Payload enviado al webhook (formato del contrato `docs/webhooks-api.md`)

Derivado por `buildWebhookPayload` (`internal/services/webhook.go`):
```json
{
  "year": 2026,
  "month": 3,
  "amount": 54.63,
  "status": "pending",
  "invoice_number": "<NUM_POLIFACTURA>"
}
```

`year`/`month` salen de `Period` (`2026-03`), `amount` de `MONTO`, `status` de
`STATUS` y `invoice_number` de `Raw["invoice_number"]` (vía `extractInvoiceNumber`,
SPEC-010). El formato es idéntico al de Claro/DISNORTE/Tigo/ENACAL.

### 4.5 Dependencias

- **Internas**: `internal/plugins` (interfaz/registry), `internal/services` (`BillsService`), `internal/storage` (plugins/bills).
- **Externas**: **Ninguna nueva**. Solo stdlib.

## 5. Criterios de Aceptación

### 5.1 Funcionales

- [x] CA-001: **Fase 0 (Gate)** — **SUPERADA**: `POST login_v.aspx` con `txt_Usuario`/`txt_Clave`/`botonLoginAux`/`TipoDispotivivo=M` **sin `tokenCaptcha`** → **302 + `Set-Cookie: .ASPXAUTH` + `var`**; `Consulta_Polizas.aspx` muestra el asegurado (contexto completo). El login es viable en Go puro sin reCAPTCHA.
- [x] CA-002: Dado un `GET /api/plugins` autenticado, aparece `assa.nicaragua` 1.0.0 con su `source` y descripción (verificado por API en local).
- [x] CA-003: Dado un `PUT /api/accounts/{id}/plugin` con `assa.nicaragua`, la cuenta queda asociada (verificado: `GET /api/accounts/14/plugin` → `{plugin_name: assa.nicaragua, version: 1.0.0}`).
- [x] CA-004: Dado un `POST /api/accounts/{id}/bills:fetch` con credenciales válidas en SQLite e `identifier` = póliza, el plugin autentica (login sin captcha + `var`), consulta `Consulta_Polizas.aspx` + `Unidades.aspx` y persiste `bills` con `status: ok`, `plugin_version` y `source` poblados (E2E real por API OK: 2 facturas).
- [x] CA-005: Toda la data de facturación se normaliza: polifacturas pendientes (`Period` de `FECHA`, `Amount` = `MONTO`, `DueDate` = `VENCIMIENTO`, `Status` de `STATUS`) y recibos pagados (`Period` de `MES_PAGADO`, `Amount` = `MONTO`, `DueDate` = `FECCOBRO`, `Status` = `paid`). Verificado con datos reales: 6 bills (2 pending + 4 paid).
- [x] CA-006: El `Raw` de cada bill incluye `invoice_number` = `POLIFACTURA` (`<NUM_POLIFACTURA>`), y `buildWebhookPayload` emite el payload `{year, month, amount, status, invoice_number}` correcto (E2E real).
- [x] CA-007: Dado un `password` inválido, la consulta devuelve `auth_failed` (test unitario `TestFetchBills_LoginRejected`).
- [x] CA-008: Dado un identifier de póliza inexistente entre las pólizas del asegurado, la consulta devuelve `service_not_found` (test unitario `TestFetchBills_PolizaNotFound`).
- [x] CA-009: Las credenciales (`username`/`password`) y el token de sesión (`var`, `.ASPXAUTH`) NUNCA aparecen en respuestas de API ni en `raw` (el `raw` expuesto solo trae datos de la factura).
- [x] CA-010: El seed local crea app + cuenta ASSA (app id 5, account id 14, identifier `02B000000`) con credenciales de entorno, y la consulta end-to-end desde la API local funciona.
- [ ] CA-DARK: N/A (no toca formularios frontend; reusa `CredentialSchema` de SPEC-011).
- [ ] CA-BACK: N/A (sin páginas nuevas).

### 5.2 No funcionales

- [ ] CA-NF-001: `go build ./...` compila sin errores.
- [ ] CA-NF-002: `go test ./internal/plugins/assa/...` y `go test ./internal/services/...` pasan.
- [ ] CA-NF-003: Sin dependencias nuevas en `go.mod`.
- [ ] CA-NF-004: Toda llamada saliente a ASSA tiene timeout ≤ 15 s y no bloquea otras peticiones.
- [ ] CA-NF-005: Una consulta completa (auth + pólizas + unidades) < 5 s.

### 5.3 Testing

- **Unit tests**: Parsing de form fields del login, extracción de `var` de la `Location`,
  parsing de grids `proxy:{data:[...]}` (pólizas, unidades, recibos, polifacturas,
  balance), normalización a `Bill` (periodo de `FECHA`, monto, vencimiento, estado),
  mapeo de errores (`auth_failed`, `api_changed`, `service_not_found`).
- **Integration tests**: Flujo con fixture (HTML embebido): `GET login` → `POST login`
  (302 + cookies) → `Consulta_Polizas` → `Unidades` → `bills` persistido; y
  `buildWebhookPayload` con `Raw["invoice_number"]` produce
  `{year, month, amount, status, invoice_number}`.
- **Fase 0 (Gate)**: prueba manual/script contra el portal real: `GET login_v.aspx` →
  extraer `tokenCaptcha` → `POST` de login con credenciales reales; verificar 302 +
  `.ASPXAUTH` (aceptado) o formulario con error (bloqueado).
- **E2E tests**: Desde la UI, consultar facturas de una cuenta ASSA con credenciales reales (local).
- **Carga/Performance**: Una consulta < 5 s; sin picos de RAM (sin deps, sin pools).

## 6. Plan de Implementación

### 6.1 Fases

| Fase | Descripción | Estimación | Dependencias |
|------|-------------|------------|--------------|
| 0 | **Validación del captcha (Gate)** — **EJECUTADA: SUPERADA**. `POST login_v.aspx` con `txt_Usuario`+`txt_Clave`+`botonLoginAux`+`TipoDispotivivo=M` **sin `tokenCaptcha`** → 302 + `.ASPXAUTH` + `var`, sesión con asegurado completo. Login viable en Go puro | 0.25 día | Ninguna |
| 1 | Espec con datos reales de la captura (este documento) | 0.25 día | Ninguna |
| 2 | Plugin `assa.nicaragua` v1.0.0: login WebForms (form fields + `tokenCaptcha` + cookies + `var`), grid parser (`proxy:{data:[...]}`), pólizas, unidades/polifacturas, normalización a `Bill`, errores de dominio | 1 día | Fase 0 y 1 |
| 3 | Registro en `main.go` + `SyncCatalog`; verificación de `buildWebhookPayload` con `invoice_number` (formato del webhook) | 0.25 día | Fase 2 |
| 4 | Seed local de app+cuenta ASSA (credenciales por entorno, configuradas en SQLite para prueba local) | 0.25 día | Fase 2 |
| 5 | Tests unitarios/integración + `go build`/`go test`/`npm run build` | 0.5 día | Fase 4 |
| 6 | Validación end-to-end local con las credenciales reales del usuario | 0.5 día | Fase 5 |

### 6.2 Milestones

- **MVP**: Plugin `assa.nicaragua` v1.0.0 con login sin captcha (`txt_Usuario`/
  `txt_Clave`/`botonLoginAux`/`TipoDispotivivo=M`, **sin `tokenCaptcha`** → 302 +
  `.ASPXAUTH` + `var`), lista de pólizas, polifacturas pendientes, normalización a
  `Bill`, seed local (cuenta real en SQLite) y webhook con `invoice_number` correcto.
  Flujo completo verificado end-to-end con curl (2026-09-14): login sin captcha →
  `Consulta_Polizas.aspx?var=<VAR>&Tipo=` (grid embebido) → `Unidades.aspx` con
  polifacturas PENDIENTE embebidas.
- **V1.1** (opcional): Pagos en línea `pagosPendientesPrima.aspx` (REQ-011),
  recibos como historial en `Raw` (REQ-012), `TipoDispotivivo` configurable (REQ-013).

## 7. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| **RESUELTO (Fase 0, 2026-09-14)**: el servidor exige `tokenCaptcha` solo si el campo viaja en el POST (vacío → *"Por favor complete el CAPTCHA para continuar"*; postback WebForms sin token → error). Un **POST directo sin el campo** (`txt_Usuario`+`txt_Clave`+`botonLoginAux`+`TipoDispotivivo=M`) → 302 + `.ASPXAUTH` + `var`, sesión con asegurado completo. El login automatizado en Go puro **es viable** sin resolver reCAPTCHA | Resuelto | — | Evidencia: prueba real con curl (2026-09-14). El formulario público de `<DOMINIO_ASSA_WEB>` también autentica (campos `txt_usuario`/`txt_contrasena`) pero deja la sesión sin contexto de asegurado (pólizas vacías) — **usar los campos móviles**. Datos con `Tipo=` vacío (embebidos en HTML); con `Tipo=M`/`D` el grid requiere DirectMethod. |
| El token del HTML sí es aceptado (el servidor solo exige el campo no vacío) | Media | Bajo | Es la vía que valida la Fase 0; el plugin envía el `tokenCaptcha` tal cual y re-autentica en cada consulta. |
| Sesión ASP.NET expira (timeout 20 min según `MySessionTimeout` del HTML) | Media | Bajo | `FetchBills` re-autentica en cada consulta; el `var`/`.ASPXAUTH` no se persisten. |
| ASSA cambia el layout de los grids Ext.NET (`proxy:{data:[...]}`) | Media | Alto | Versionamiento obligatorio (SPEC-003): bump + stamp en `bills`; error `api_changed` avisa al usuario. Parser acotado a un patrón JSON estable. |
| `__VIEWSTATE`/`__EVENTVALIDATION` cambian de formato o se exige el submit por `cmd_Login` | Baja | Medio | El POST real usó `botonLoginAux` (botón oculto); replicar el POST tal cual de la captura. Test de fixture + verificación E2E. |
| WAF/rate-limit del portal bloquea IPs de datacenter (iHost) | Media | Medio | Las llamadas fueron aceptadas desde IP residencial sin JS challenge. Mitigación: respetar headers del portal, no martillar la API, jobs diarios (SPEC-004). |
| El `password` cambia o se bloquea | Media | Medio | Error `auth_failed` claro en la UI; actualizar credenciales de la cuenta. |
| Póliza renovada con `STATUS: RENOVADA` y sin `FECFINVIG` vigente | Baja | Bajo | El filtro es por número de póliza (`identifier`); la polifactura pendiente sigue apareciendo (verificado en la póliza de ejemplo). |

## 8. Notas y Referencias

- Portal: `https://<DOMINIO_ASSA>` (ASP.NET WebForms + Ext.NET, "Mi ASSANET - Nicaragua").
- Login: `POST /ni/miassanet-NI/login_v.aspx` (form WebForms + `tokenCaptcha` de
  reCAPTCHA v3 → 302 + `.ASPXAUTH` + `var`). **Validación del captcha en Fase 0.**
- Pólizas: `GET /ni/sca/Consulta_Polizas.aspx?var=<VAR>&Tipo=M` (grid `Poliza`).
- Detalle/polifacturas: `GET /ni/sca/Unidades.aspx?var=<VAR>&pol=<POLIZA>&ipol=<IDEPOL>`.
- Pagos en línea (P2): `GET /ni/pagos/prima/pagosPendientesPrima.aspx?var=<VAR>&Tipo=M`.
- Captura real (Charles, `.chlz`): `/home/paulomcnally/Downloads/mi_assa_net.chlz` (2026-09-14).
- Datos de la cuenta (anonimizados) (solo seed local vía entorno): `username` `<CEDULA>`,
  `password` `<PASS>`, pólizas `<POLIZA>`/`<POLIZA2>`.
- Reutiliza: SPEC-002 (cuentas/credenciales), SPEC-003 (infraestructura de plugins/bills/endpoints), SPEC-004 (scheduler/webhook), SPEC-008 (`parseBillPeriod` tolerante), SPEC-010 (`extractInvoiceNumber`), SPEC-011 (`CredentialSchema` + formulario dinámico), SPEC-012 (patrón de plugin con captura real).
- Repo: `https://github.com/paulomcnally/p40la-ihost-automation`

## 9. Historial de Cambios

| Fecha | Autor | Descripción |
|-------|-------|-------------|
| 2026-09-14 | paulomcnally | Creación inicial de la especificación basada en la captura real de Charles Proxy del portal ASSA Nicaragua (`mi_assa_net.chlz`). Flujo confirmado: `GET login_v.aspx` (form fields + cookies) → `POST login_v.aspx` (form WebForms → 302 + `Set-Cookie: .ASPXAUTH` + `var` en `Location`) → `GET Consulta_Polizas.aspx?var=...` (grid de pólizas) → `GET Unidades.aspx?var=...&pol=...&ipol=...` (polifacturas PENDIENTE con `POLIFACTURA`, `MONTO`, `VENCIMIENTO`). Cero migraciones, cero deps. Credenciales nunca en código (solo `accounts.credentials` en SQLite local vía seed por entorno). Webhook con formato `{year, month, amount, status, invoice_number}` vía `Raw["invoice_number"]` = `POLIFACTURA`. |
| 2026-09-15 | paulomcnally | **Release** (cierre por decisión del usuario; el deploy a iHost lo realiza él). Verificado por el usuario en local (bills:fetch con 6 facturas: 2 pendientes + 4 pagadas). Issue #15 cerrado con label `spec/released`. Commits: implementación + release (docs + tracker). |
| 2026-09-15 | paulomcnally | **Cambio iterativo (pedido del usuario)**: incluir también las facturas **pagadas** (el objetivo es traer toda la data de facturación). El portal solo expone las polifacturas `PENDIENTE` en su grid; las cuotas pagadas viven en el grid de **recibos** (`NUMRECIBO`/`MES_PAGADO`/`MONTO`/`PREIMPRESO`). `FetchBills` ahora devuelve polifacturas (pending) + recibos (`paid`): `Period` de `MES_PAGADO`, `Amount` = `MONTO`, `DueDate` = `FECCOBRO`, `Raw["invoice_number"]` = `NUMRECIBO`. Verificado E2E real: 6 bills (2 pending + 4 paid). Tests actualizados (`TestFetchBills_RealFlow`, `TestFetchBills_SinGrids`). |
| 2026-09-15 | paulomcnally | **Implementación completa**. Plugin `assa.nicaragua` v1.0.0 en `internal/plugins/assa/nicaragua/nicaragua.go`: login sin captcha (`POST login_v.aspx` con body en el orden exacto `txt_Usuario`→`txt_Clave`→`botonLoginAux`→`TipoDispotivivo` — el servidor rechaza otros órdenes, verificado contra la API real; 302 + `.ASPXAUTH` + `var`), pólizas vía `Consulta_Polizas.aspx?var=&Tipo=` vacío (grid embebido), facturas/recibos/balance vía `Unidades.aspx` (grids embebidos), normalización a `Bill` (period `YYYY-MM` de `FECHA`, amount `MONTO`, due_date de `VENCIMIENTO`, status de `STATUS`, `Raw["invoice_number"]` = `POLIFACTURA`), errores de dominio (`ErrAuthFailed`, `ErrServiceNotFound`, `ErrAPISchemaChanged`). `CredentialSchema` (username/password). Registro en `cmd/server/main.go`. Seed en `scripts/seed-apps.sh` (credenciales por entorno). Tests unitarios (8) con fixtures reales anonimizados. **Verificado end-to-end contra la API real**: `bills:fetch` devolvió las 2 polifacturas reales (`2026-03` U$54.63 vence 2026-10-02 pending; `2026-03` U$54.58 vence 2026-11-02 pending; invoice_number `<NUM_POLIFACTURA>`). `go build`, `go vet`, `go test ./...` y `npm run build` OK. Cero deps nuevas. Estado → `pending_release`. |
| 2026-09-14 | paulomcnally | **Flujo completo verificado end-to-end (curl + sesión de login sin captcha)**: `GET Consulta_Polizas.aspx?var=<VAR>&Tipo=` (**Tipo vacío**) devuelve el grid de pólizas **embebido** (`proxy:{data:[...]}`, 2 pólizas) con asegurado completo; `GET Unidades.aspx?var=<VAR>&pol=<POLIZA>&ipol=<IDEPOL>` devuelve los 4 grids embebidos (unidades, recibos, polifacturas PENDIENTE, balance). El DirectMethod de Ext.NET era un falso camino: solo se usa cuando `Tipo=M`/`Tipo=D` (grid vacío). El plugin usa siempre `Tipo=` vacío → **sin AJAX, solo HTML embebido**. |
| 2026-09-14 | paulomcnally | **Fase 0 ejecutada (Gate): SUPERADA — login viable sin captcha**. Pruebas reales con curl: (1) POST con `tokenCaptcha` vacío → 200 "Por favor complete el CAPTCHA para continuar"; (2) postback WebForms completo sin token → `ManejoErrorGenerico.aspx`; (3) formulario público de `<DOMINIO_ASSA_WEB>` (`txt_usuario`/`txt_contrasena`, sin captcha) → **302 + `.ASPXAUTH` + `var`** pero sesión **sin contexto de asegurado** (pólizas vacías); (4) **POST directo con campos móviles** (`txt_Usuario`/`txt_Clave`/`botonLoginAux`/`TipoDispotivivo=M`, sin `tokenCaptcha`) → **302 + `.ASPXAUTH` + `var`** y `Consulta_Polizas.aspx` con asegurado completo. El plugin usará el flujo (4). Pendiente de investigación en Fase 2: el grid de pólizas del sitio actual se carga vía DirectMethod Ext.NET (`_methodName_`/`submitDirectEventConfig`), a diferencia de la captura que lo embebía en el HTML. |
| 2026-09-14 | paulomcnally | **Corrección del análisis del captcha (revisión del usuario)**: el "hallazgo clave" inicial (token enviado == pre-renderizado, login sin JS) era una **inferencia incorrecta**: se comparó el POST contra el cuerpo del 302, que el servidor re-renderiza con un eco de los valores enviados. Evidencia real: la captura muestra `POST /recaptcha/api2/reload` (entrada 36, terminó 23:24:56.830) 4 ms antes del POST de login (entrada 38, inició 23:24:56.829) — el navegador generó el token vía JS de reCAPTCHA v3; no existe `var token` en la página. Se añade **Fase 0 (Gate)** obligatoria: probar el login con el `tokenCaptcha` del HTML sin JS; si el servidor lo rechaza, la integración queda bloqueada (sin MVP). Se actualizan: Resumen Ejecutivo, REQ-002, §3.1, tabla de opciones, ADR-001/002, §4.4, CA-001, §5.3, fases, milestones y §7. |