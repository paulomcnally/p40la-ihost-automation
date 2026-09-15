# 📋 Especificaciones Técnicas

Este directorio contiene todas las especificaciones técnicas del proyecto `p40la-ihost-automation`.

## 📊 Resumen

| Métrica | Valor |
|---------|-------|
| **Total de specs** | 15 |
| **En draft** | 0 🟡 |
| **Pending execution** | 0 🔵 |
| **In progress** | 0 🟣 |
| **Pending release** | 0 🟠 |
| **Released** | 14 🟢 |
| **Canceladas** | 1 ⚫ |
| **Último ID usado** | SPEC-015 |

---

## 📑 Índice de Specifications

| ID | Título | Estado | Fecha Creación | Autor |
|----|--------|--------|----------------|-------|
| SPEC-015 | Plugins en módulo privado con dominios ofuscados | released | 2026-09-15 | paulomcnally |
| SPEC-014 | Plugin assa.nicaragua: autenticación, pólizas y facturas | released | 2026-09-14 | paulomcnally |
| SPEC-013 | Selector de plugin en creación de cuenta + formulario dinámico de credenciales | released | 2026-09-13 | paulomcnally |
| SPEC-012 | Plugin enacal.nicaragua: autenticación y facturas | released | 2026-09-12 | paulomcnally |
| SPEC-011 | Formularios de credenciales por plugin + reveal con password | released | 2026-09-12 | paulomcnally |
| SPEC-010 | Fix invoice_number Tigo + invoice_number en historial | released | 2026-09-12 | paulomcnally |
| SPEC-009 | Plugin disnorte.dissur.nicaragua: autenticación y recibos | released | 2026-09-12 | paulomcnally |
| SPEC-008 | Fix plugin tigo.nicaragua: formato de periodo y montos en webhook | released | 2026-09-12 | paulomcnally |
| SPEC-007 | Plugin tigo.nicaragua: lista de facturas vía id_token + refresh | released | 2026-09-12 | paulomcnally |
| SPEC-006 | Fix duplicados de facturas en webhook Claro Nicaragua | cancelled | 2026-09-12 | paulomcnally |
| SPEC-005 | Aislar cookies de sesión entre proyectos | released | 2026-09-12 | paulomcnally |
| SPEC-004 | Jobs diarios de facturas con envío a webhook | released | 2026-09-11 | paulomcnally |
| SPEC-003 | Plugin claro.nicaragua: consulta de facturas | released | 2026-09-11 | paulomcnally |
| SPEC-002 | Menú y CRUD de Apps con cuentas y credenciales | released | 2026-09-11 | paulomcnally |
| SPEC-001 | Cascote base: autenticación, dashboard y settings vacío | released | 2026-09-11 | paulomcnally |

---

## 🔄 Flujo de Estados

```
     ┌─────────┐     ┌─────────────────┐     ┌───────────┐     ┌───────────────┐     ┌──────────┐
     │  draft  │────▶│ pending_execution │────▶│ in_progress│────▶│pending_release│────▶│ released │
     └─────────┘     └─────────────────┘     └───────────┘     └───────────────┘     └──────────┘
          │                  │                      │                  │                │
          ▼                  ▼                      ▼                  ▼                ▼
     ┌─────────┐       ┌─────────┐            ┌─────────────┐      ┌───────────┐      ┌──────────┐
     │cancelled│       │cancelled│            │pending_execution│  │in_progress│      │in_progress│
     └─────────┘       └─────────┘            └─────────────┘      └───────────┘      └──────────┘
```

### Leyenda de Estados

| Estado | Emoji | Descripción |
|--------|-------|-------------|
| `draft` | 🟡 | En proceso de redacción/investigación |
| `pending_execution` | 🔵 | Lista para desarrollo, no iniciada |
| `in_progress` | 🟣 | Actualmente en desarrollo |
| `pending_release` | 🟠 | Desarrollo completo, lista para staging/release |
| `released` | 🟢 | Subida a iHost o producción |
| `cancelled` | ⚫ | Cancelada o descartada |

---

## 📝 Cómo usar este sistema

### Crear una nueva spec

Usa la skill `spec-manager`:

```
/spec create "Título de la nueva funcionalidad"
```

O pide al asistente: *"Crear spec para [requerimiento]"*

### Cambiar estado de una spec

```
/spec status SPEC-001 pending_execution
```

O pide al asistente: *"Cambiar SPEC-001 a in_progress"*

### Ver detalle de una spec

```
/spec show SPEC-001
```

---

## 📁 Estructura del directorio

```
docs/specs/
├── README.md              # Este archivo - tracking y contador
├── templates/
│   └── spec-template.md   # Template para nuevas specs
└── SPEC-XXX-*.md         # Archivos de specifications
```

---

*Última actualización de este tracker: 2026-09-15 — SPEC-015 (Plugins en módulo privado con dominios ofuscados) **released**: contrato + 5 implementaciones movidos al repo privado `p40la-ihost-automation-plugins` v1.0.0 (dominios cifrados AES-256-GCM vía `secret.Must`/`cmd/gensecret`, `all.RegisterAll`, CI propio); repo público con aliases en `internal/plugins`, `main.go` con `pluginall.RegisterAll`, Dockerfile con BuildKit secret `gh_token` + `git` en builder, workflow con `secrets.PLUGINS_TOKEN`, compose con `.gh_token`, `seed-apps.sh` exige `PLUGIN_SOURCE`, `tigo-auth.sh` movido al privado, reglas en AGENTS.md. Verificado: build/test OK, `strings` del binario sin dominios, imagen Docker con secret OK. Issue #16 cerrado.

*Última actualización de este tracker: 2026-09-15 — SPEC-014 (Plugin assa.nicaragua: autenticación, pólizas y facturas) **released** (cerrada por decisión del usuario; el deploy a iHost lo realiza él): plugin `assa.nicaragua` v1.0.0 (`internal/plugins/assa/nicaragua/`), login sin captcha (`POST login_v.aspx` con orden exacto `txt_Usuario`→`txt_Clave`→`botonLoginAux`→`TipoDispotivivo`, sin `tokenCaptcha`; 302 + `.ASPXAUTH` + `var`), pólizas con `Tipo=` vacío (`Consulta_Polizas.aspx`, grid embebido), facturas vía `Unidades.aspx` (polifacturas PENDIENTE + recibos pagados = toda la data de facturación) → webhook `{year, month, amount, status, invoice_number}` con `Raw["invoice_number"]`. Registrado en `main.go`, seed en `seed-apps.sh`, cuenta real (póliza `02B000000`) en SQLite local. Verificado end-to-end contra la API real (6 facturas: 2 pending + 4 paid). Cero migraciones/deps. Issue #15 cerrado.

*Última actualización de este tracker: 2026-09-12 — SPEC-012 (Plugin enacal.nicaragua: autenticación y facturas) **released** (cerrada por decisión del usuario; el deploy a iHost lo realiza él): plugin `enacal.nicaragua` v1.0.0 (`internal/plugins/enacal/nicaragua/`), auth `POST /soe/api/account/authenticate` ({username,password} → {"success":true}) + `Authorization: Basic`, cookies del WAF F5 en jar, facturas vía `GET /soe/api/InfoCuentas/facturas/?CuentaNro=<NIC>` con `Raw["numFactura"]` → webhook `{year, month, amount, status, invoice_number}`. Registrado en `main.go`, seed en `seed-apps.sh`, cuenta real (NIC <NIC>) en SQLite local. Verificado end-to-end contra la API real (6 facturas). Cero migraciones/deps. Issue #13 cerrado.

Anterior — SPEC-011 (Formularios de credenciales por plugin + reveal con password) **released** (cerrada por decisión del usuario; el deploy a iHost lo realiza él): `CredentialSchema()` en los 3 plugins, `GET /api/plugins/{name}/schema`, `POST /api/accounts/{id}/credentials:reveal` (bcrypt), `GetAccount` sin credenciales en claro, `AccountModal` con formulario dinámico + reveal con password + fallback textarea. Verificado end-to-end. Issue #12 cerrado.

Anterior — SPEC-010 (Fix invoice_number Tigo + invoice_number en historial) **released** (cerrada por decisión del usuario; el deploy a iHost lo realiza él): `extractInvoiceNumber`/`scalarValue` tolerantes en webhook.go, `BillPayload.invoice_number` en fetch+historial, UI de AccountBillsPage muestra el número. Verificado end-to-end (fetch y webhook con `<NUM_FACTURA>` limpio). Issue #11 cerrado.

Anterior — SPEC-008 (Fix plugin tigo.nicaragua: formato de periodo y montos en webhook) **released** (cerrada por decisión del usuario; el deploy a iHost lo realiza él): `parseBillPeriod` tolerante (DD-MM-YYYY + YYYY-MM) en webhook.go, `invoice_number` para Tigo (`Raw["invoiceId"]`), `billAmount` prefiere `invoiceAmount` en el plugin Tigo (facturas pagadas con monto real). Código verificado (`go build`/`go test` OK). Issue #9 cerrado.

Anterior — SPEC-007 (Plugin tigo.nicaragua: lista de facturas) **released** (commit a7e2941): plugin v1.0.0 con auth por `refresh_token` (id_token RS256 como Bearer), endpoints `accounts`/`balance`/`invoices` confirmados con la captura real de Charles, deps `utls`/`x/net` eliminadas. Verificado end-to-end: `bills:fetch` devolvió las 6 facturas reales.