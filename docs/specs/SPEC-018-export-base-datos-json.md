---
title: "Export completo de la base de datos a JSON desde Settings"
id: "SPEC-018"
status: "released"
author: "opencode"
created: "2026-10-21"
updated: "2026-10-21"
github_issue: 20
---

# Export completo de la base de datos a JSON desde Settings

**ID**: SPEC-018  
**Estado**: released  
**Autor**: opencode  
**Creado**: 2026-10-21  
**Actualizado**: 2026-10-21

---

## 1. Resumen Ejecutivo

Hoy no existe forma de sacar la información del add-on: los datos viven únicamente en el
SQLite del iHost (`data/app.db`) y, para respaldarlos o diagnosticarlos, hay que entrar por
SSH/SFTP al dispositivo. Esto es incómodo en el día a día y riesgoso como estrategia de
respaldo (un `docker compose down -v`, una migración fallida o un iHost reinstalado se llevan
todo sin vuelta atrás). Se necesita una acción simple desde **Settings** que descargue *toda*
la base de datos como un único archivo **JSON en texto plano**, legible y portable.

El alcance es deliberadamente total: `users` (con `password_hash` bcrypt), `settings`
(incluye la `api_key` global de webhooks), `apps`, `accounts` (con las credenciales JSON de
los portales en claro), `plugins`, `bills` (con el `raw` completo de cada consulta),
`webhook_logs` (payloads y respuestas) y `schema_migrations`. Decisión confirmada por el
usuario: el export es un respaldo fiel, sin ofuscar ni enmascarar secretos. El archivo se
descarga como adjunto y **nunca** se renderiza en pantalla.

La restricción dura es el iHost: poca RAM, CPU modesta y SQLite. Por eso el endpoint debe
**streamear** el JSON directamente al `http.ResponseWriter` (encoder incremental sobre
`rows.Next()`), sin construir el documento completo en memoria ni materializar listas de
`bills`/`webhook_logs`. El impacto esperado es mínimo: sin dependencias nuevas (solo
`encoding/json` de la stdlib), sin migraciones y con memoria extra acotada a unos pocos
buffers por fila.

Consideraciones de UI obligatorias para formularios/modales:
- Todo `input`/`select`/`textarea` DEBE usar los tokens de color del tema (`bg-card`, `text-text`,
  `text-text-secondary`). Jamás colores hardcodeados (ej: `bg-white`, `dark:bg-[#2c2c2e]`).
- Todo formulario nuevo debe verificarse en darkmode antes de considerarse completo: texto y
  placeholders legibles.

## 2. Requerimientos

### 2.1 Requerimientos Funcionales (P0 - Obligatorios)

1. **REQ-001**: Endpoint autenticado `GET /api/settings/export` que responda la totalidad de la
   base de datos como JSON UTF-8 válido y lo entregue como descarga
   (`Content-Disposition: attachment; filename="p40la-export-<timestamp>.json"`).
2. **REQ-002**: El JSON debe incluir, sin omitir columnas ni filas, las tablas: `users`,
   `settings`, `apps`, `accounts`, `plugins`, `bills`, `webhook_logs` y `schema_migrations`.
3. **REQ-003**: Los secretos se exportan tal cual están almacenados: `users.password_hash`,
   `accounts.credentials` (JSON de credenciales en claro), los valores de `settings` (incluida
   la `api_key` de webhooks), `bills.raw` y `webhook_logs.payload/response`.
4. **REQ-004**: Envoltorio del export con metadatos: `format_version`, `app`, `exported_at`
   (RFC3339 UTC), `exported_by` (email del usuario autenticado), lista de migraciones aplicadas
   y `counts` por tabla, más las filas agrupadas en `tables`.
5. **REQ-005**: Botón **Exportar base de datos** en la página Settings que descarga el archivo
   mediante `fetch` autenticado (cookie de sesión) + `Blob`, con estado de carga y toast de
   éxito/error; nunca se muestra el JSON en pantalla.
6. **REQ-006**: Modal de confirmación previo a la descarga que advierte explícitamente que el
   archivo contiene **contraseñas hasheadas, credenciales de portales y la API key en texto
   plano** y no debe compartirse.
7. **REQ-007**: El endpoint debe streamear la respuesta (sin cargar todas las tablas en memoria)
   y tolerar tablas grandes (`bills`, `webhook_logs`) sin agotar la RAM del iHost.
8. **REQ-008**: Traducciones completas en `es` y `en` para todos los textos nuevos y cabeceras
   `Cache-Control: no-store` en la respuesta.

### 2.2 Requerimientos Funcionales (P1 - Importantes)

1. **REQ-009**: Parámetro opcional `?pretty=true` para indentar el JSON (útil para inspección
   manual); por defecto compacto para minimizar tamaño y CPU.
2. **REQ-010**: Registrar en el log del servidor una línea de auditoría por export
   (`usuario`, `tablas`, `filas totales`, `bytes`), sin volcar datos sensibles.
3. **REQ-011**: Los campos que contienen JSON anidado (`accounts.credentials`, `bills.raw`,
   `webhook_logs.payload`, `webhook_logs.response`) se embeben como objeto/array JSON cuando el
   contenido es válido, y como string cuando no lo es (nunca se pierde información).

### 2.3 Requerimientos Funcionales (P2 - Deseables)

1. **REQ-012**: Compresión opcional `?gzip=true` (`Content-Encoding: gzip`) para exports grandes.
2. **REQ-013**: Botón secundario "Copiar JSON al portapapeles" solo si el export pesa menos de
   ~200 KB (evita congelar la UI en iHost).
3. **REQ-014**: Checksum `sha256` del cuerpo en un header `X-Export-SHA256` para verificar la
   integridad de la descarga.

### 2.4 Requerimientos No Funcionales

- **Rendimiento**: primera fila emitida en < 500 ms; export típico (< 10 MB) en < 5 s en iHost;
  sin `OOM` con `bills`/`webhook_logs` de decenas de miles de filas (streaming obligatorio).
- **Memoria**: +10–20 MB de RSS como máximo durante el export (buffers de `bufio` + una fila).
- **Seguridad**: solo usuarios autenticados (cookie `p40la_ihost_automation_session`); el
  endpoint expone secretos por diseño, por lo que debe llevar `no-store`, advertir en UI y
  quedar auditado. No se agrega token en query string (evita filtrar secretos en logs).
- **Almacenamiento**: cero migraciones; cero tablas nuevas; ningún archivo temporal en disco.
- **Disponibilidad**: lectura en transacción `read-only`; no debe bloquear escrituras del
  scheduler más allá de lecturas cortas (SQLite WAL / snapshot consistente).
- **iHost**: solo stdlib (`encoding/json`, `bufio`, `net/http`, `database/sql`); sin
  dependencias nuevas ni binarios extra.

## 3. Investigación y Decisiones Técnicas

### 3.1 Contexto investigado

- Esquema real (migraciones `0001`–`0006`): `users`, `settings`, `apps`, `accounts`, `plugins`,
  `bills`, `webhook_logs` y la tabla interna `schema_migrations` creada por `internal/db`.
- `internal/storage/*` devuelve modelos tipados por tabla; para un export "de toda la data" eso
  obliga a mantener un modelo por tabla y arriesga perder columnas nuevas.
- El backend sirve la SPA desde el mismo binario; no hay endpoint de export ni de backup hoy.
- `accounts.credentials` se guarda como TEXT con JSON en claro (validado en
  `validateCredentials`), y `users.password_hash` es bcrypt: ambos son "secretos" pero
  imprescindibles para una restauración fiel.
- UI: `SettingsPage.tsx` es una tarjeta única (webhooks) que se extiende naturalmente con una
  segunda tarjeta; `frontend/src/api/index.ts` ya centraliza llamadas y `useToast` maneja avisos.

### 3.2 Opciones evaluadas

| Opción | Pros | Contras | Decisión |
|--------|------|---------|----------|
| A. Export por tablas con modelos tipados (`[]models.X`) | Tipado, reutiliza storage | Acopla el export al esquema; una columna nueva se pierde silenciosamente; carga todo en memoria | ❌ Rechazada |
| B. `SELECT *` genérico + `rows.Columns()` y stream JSON | Agnóstico al esquema, inmune a migraciones futuras, memoria O(1), pocas líneas | Requiere normalizar `time.Time`/`[]byte`/`NULL` a mano; menos tipado | ✅ Seleccionada |
| C. Copiar `data/app.db` y servir el binario SQLite | Fidelidad 100%, trivial | No es JSON, no es legible, requiere cerrar/bloquear la BD o usar backup API | ❌ Rechazada |
| D. Generar el archivo en el frontend consultando cada endpoint existente | Sin backend nuevo | Pierde `users`, `settings`, `webhook_logs` y campos ocultos (`credentials`); N requests; no es "toda la data" | ❌ Rechazada |
| E. Volcado a disco (`data/export.json`) y descarga posterior | Simple | Archivo temporal con secretos en disco, limpieza manual, doble I/O en iHost | ❌ Rechazada |

### 3.3 Decisiones arquitectónicas (ADRs)

**ADR-001**: Export genérico por `SELECT *` con nombres de columna dinámicos
- **Contexto**: el objetivo es "toda la data, incluidas columnas futuras", con mínimo
  mantenimiento y sin dependencias.
- **Decisión**: un único método de storage/streaming recorre una lista blanca de tablas, lee
  `rows.Columns()`, escanea a `[]any` y normaliza valores (`time.Time` → RFC3339 UTC,
  `[]byte` → string, `NULL` → `null`) antes de encodear con `json.Encoder`.
- **Consecuencias**: el export sobrevive a nuevas migraciones sin cambios de código; se pierde
  el tipado fuerte y hay que cubrir la normalización con tests.

**ADR-002**: Streaming directo a la respuesta HTTP, sin `json.Marshal` del documento completo
- **Contexto**: iHost tiene RAM limitada y `bills`/`webhook_logs` pueden crecer sin techo.
- **Decisión**: escribir el envoltorio y cada tabla con `json.Encoder` sobre el
  `http.ResponseWriter` envuelto en `bufio.Writer`; las filas se emiten a medida que se leen.
- **Consecuencias**: memoria constante; si el cliente corta la conexión la respuesta queda
  truncada (detectable por JSON inválido), mitigado con `X-Export-SHA256` (P2).

**ADR-003**: Secretos incluidos sin enmascarar, con advertencia explícita
- **Contexto**: el usuario pidió un respaldo fiel "en texto plano"; enmascarar credenciales
  haría el archivo inútil para restaurar.
- **Decisión**: exportar `password_hash`, `credentials`, `settings` y payloads tal cual; la UI
  exige confirmación con advertencia y la respuesta lleva `no-store`.
- **Consecuencias**: el archivo debe tratarse como material sensible; queda auditado en logs.

**ADR-004**: Envoltorio versionado con metadatos y conteos
- **Contexto**: un JSON plano de tablas sueltas no permite validar integridad ni evolución del
  formato.
- **Decisión**: `format_version: 1` + `app`, `exported_at`, `exported_by`, `schema_migrations`
  y `counts` antes de `tables`.
- **Consecuencias**: base para un futuro import (SPEC aparte) y para verificar que el archivo
  está completo.

## 4. Diseño Técnico

### 4.1 Diagrama de arquitectura

```
SettingsPage (React)
   │  POST confirmación (modal) → GET /api/settings/export
   ▼
export_handlers.go (authMiddleware)
   │  escribe header + Content-Disposition
   ▼
services.ExportService.Stream(ctx, w, opts)
   │  SELECT * FROM <tabla>  (por tabla, orden determinista por id)
   ▼
SQLite (data/app.db)  ── filas ──► json.Encoder (stream) ──► descarga .json
```

### 4.2 Componentes

#### 4.2.1 Handler de export
- **Responsabilidad**: validar sesión, parsear `pretty`/`gzip`, fijar headers
  (`Content-Type: application/json; charset=utf-8`, `Content-Disposition: attachment`,
  `Cache-Control: no-store`), delegar el stream y registrar la auditoría.
- **Interfaz**: `GET /api/settings/export[?pretty=true][&gzip=true]`.
- **Dependencias**: `ExportService`.
- **Ubicación**: `internal/api/export_handlers.go`, registrado en `internal/api/routes.go`
  con `authMiddleware`.

#### 4.2.2 Servicio de export
- **Responsabilidad**: armar el envoltorio, contar filas por tabla y emitir cada tabla en
  streaming con normalización de tipos.
- **Interfaz**: `Stream(ctx context.Context, w io.Writer, opts ExportOptions) (Stats, error)`.
- **Dependencias**: `*sql.DB` (solo lectura), `encoding/json`, `bufio`.
- **Ubicación**: `internal/services/export.go`.

#### 4.2.3 UI en Settings
- **Responsabilidad**: tarjeta "Respaldo" con descripción, advertencia y botón; modal de
  confirmación; descarga vía `fetch` + `Blob`; estados `downloading`/error; toast.
- **Interfaz**: `api.export.database(): Promise<Blob>` en `frontend/src/api/index.ts`.
- **Ubicación**: `frontend/src/pages/SettingsPage.tsx`, i18n en
  `frontend/public/i18n/{es,en}.json`.

### 4.3 Modelo de datos

No se agregan tablas ni columnas. Se exportan las existentes:

```
users: id, email, password_hash, created_at, updated_at
settings: key, value
apps: id, name, created_at, updated_at
accounts: id, app_id, identifier, label, credentials, created_at, updated_at,
          plugin_name, webhook_url, schedule_time, schedule_enabled, last_run_at,
          schedule_frequency, schedule_interval, schedule_days
plugins: id, name, version, source, description, status, created_at, updated_at
bills: id, account_id, plugin_name, plugin_version, source, status, error, raw,
       fetched_at, created_at
webhook_logs: id, account_id, webhook_url, status, payload, response, ran_at, http_status
schema_migrations: version (incluida como lista; no se re-exporta como tabla de datos)
```

### 4.4 APIs / Contratos

#### Endpoint: `GET /api/settings/export`

**Query params** (opcionales): `pretty=true`, `gzip=true`.

**Response 200** (`application/json; charset=utf-8`):
```json
{
  "export": {
    "format_version": 1,
    "app": "p40la-ihost-automation",
    "exported_at": "2026-10-21T14:03:11Z",
    "exported_by": "user@example.com",
    "schema_migrations": ["0001_create_users_and_settings.up.sql", "..."],
    "counts": {
      "users": 1,
      "settings": 3,
      "apps": 5,
      "accounts": 9,
      "plugins": 5,
      "bills": 120,
      "webhook_logs": 45
    },
    "tables": {
      "users": [
        {
          "id": 1,
          "email": "user@example.com",
          "password_hash": "$2a$10$...",
          "created_at": "2026-09-11T00:00:00Z",
          "updated_at": "2026-09-11T00:00:00Z"
        }
      ],
      "settings": [{ "key": "webhook.api_key", "value": "..." }],
      "apps": [{ "id": 1, "name": "Claro Nicaragua", "created_at": "...", "updated_at": "..." }],
      "accounts": [
        {
          "id": 1,
          "app_id": 1,
          "identifier": "A1B2C3",
          "label": "Internet Fibra",
          "plugin_name": "claro.nicaragua",
          "credentials": { "username": "...", "password": "..." },
          "webhook_url": "https://...",
          "schedule_enabled": 1,
          "created_at": "...",
          "updated_at": "..."
        }
      ],
      "plugins": [{ "id": 1, "name": "claro.nicaragua", "version": "1.0.0", "status": "active" }],
      "bills": [{ "id": 1, "account_id": 1, "raw": { "any": "json" }, "status": "ok" }],
      "webhook_logs": [{ "id": 1, "account_id": 1, "payload": "{...}", "response": "..." }]
    }
  }
}
```

**Headers**:
```
Content-Type: application/json; charset=utf-8
Content-Disposition: attachment; filename="p40la-export-20261021T140311Z.json"
Cache-Control: no-store
X-Export-SHA256: <hex>        # P2
```

**Response Error**:
```json
{ "error": "unauthorized", "message": "Sesión no válida" }
```
```json
{ "error": "internal_error", "message": "error al exportar tabla bills: ..." }
```

> Nota: cuando la respuesta ya empezó a streamearse, un error a mitad de camino no puede
> convertirse en un JSON de error; el cliente recibe un documento truncado (JSON inválido) y el
> servidor registra el fallo en el log.

### 4.5 Dependencias

- **Internas**: `internal/api/routes.go`, `internal/api/export_handlers.go`,
  `internal/services/export.go`, `internal/storage` (solo `*sql.DB`),
  `frontend/src/pages/SettingsPage.tsx`, `frontend/src/api/index.ts`,
  `frontend/public/i18n/{es,en}.json`.
- **Externas**: ninguna nueva (stdlib).

## 5. Criterios de Aceptación

### 5.1 Funcionales

- [x] CA-001: Dado un usuario autenticado en Settings, cuando pulsa "Exportar base de datos" y
      confirma el modal, entonces el navegador descarga un archivo
      `p40la-export-<timestamp>.json` con `Content-Type: application/json`.
- [x] CA-002: Dado el archivo descargado, cuando se valida con un parser JSON, entonces es
      válido y contiene las claves `users`, `settings`, `apps`, `accounts`, `plugins`, `bills`
      y `webhook_logs` dentro de `tables`.
- [x] CA-003: Dado el mismo momento de export, cuando se compara `counts` con
      `SELECT COUNT(*)` de cada tabla, entonces coinciden exactamente.
- [x] CA-004: Dado que existe al menos un usuario, una cuenta con credenciales y una
      `api_key` configurada, cuando se inspecciona el archivo, entonces `password_hash`,
      `credentials` (como objeto JSON) y el valor de `settings` aparecen en claro.
- [x] CA-005: Dado el modal de confirmación, cuando se muestra, entonces advierte que el
      archivo contiene credenciales, API key y hashes, y permite cancelar sin descargar nada.
- [ ] CA-006: Dado un export con miles de filas en `bills`/`webhook_logs`, cuando se ejecuta en
      iHost, entonces la RAM del proceso no crece proporcionalmente al tamaño del export
      (streaming verificado).
- [x] CA-007: Dado un request sin sesión, cuando se llama `GET /api/settings/export`, entonces
      responde `401` y no entrega datos.
- [x] CA-008: Dado `?pretty=true`, cuando se descarga, entonces el JSON está indentado; sin el
      parámetro llega compacto.
- [ ] CA-DARK: Los controles nuevos (botón, modal) usan tokens del tema (`bg-card`, `text-text`,
      `border-border`) y se verificó legibilidad en darkmode (texto y placeholders).
- [x] CA-BACK: No aplica: la feature no crea página de detalle ni formulario con lista padre
      (todo ocurre dentro de Settings).

### 5.2 No funcionales

- [ ] CA-NF-001: Export típico (< 10 MB) completado en < 5 s en iHost.
- [x] CA-NF-002: `go build ./...` y `npm run build` sin nuevas dependencias ni warnings.
- [x] CA-NF-003: `Cache-Control: no-store` presente y auditoría registrada por export.

### 5.3 Testing

- **Unit tests**: normalización de `time.Time`/`[]byte`/`NULL`; embedding de JSON válido vs
  string; `counts` correctos; error de tabla inexistente manejado.
- **Integration tests**: SQLite en memoria con datos sembrados → `ExportService.Stream`
  produce JSON válido y completo; endpoint con y sin sesión (401/200); `Content-Disposition`.
- **E2E tests**: flujo Settings → modal → descarga → validar archivo con `jq` en un iHost real.
- **Carga/Performance**: sembrar 50k `bills` y medir RSS de pico + tiempo total.

### 5.4 Verificación de la implementación (2026-10-21)

Implementado y verificado en local:

- `internal/services/export.go`: stream genérico por `SELECT *` con `json.Encoder` y
  reindentador `prettyJSONWriter` de memoria constante.
- `internal/api/export_handlers.go` + ruta `GET /api/settings/export` en `routes.go`; servicio
  cableado en `cmd/server/main.go`.
- `frontend/src/api/index.ts` (`settings.exportDatabase`) y tarjeta + modal en
  `frontend/src/pages/SettingsPage.tsx`; traducciones en `public/i18n/{es,en}.json`.
- Tests: `internal/services/export_test.go` (4 tests: contenido completo, secretos/JSON
  anidado, JSON inválido como string, `pretty` equivalente al compacto). `go test ./...` OK.
- E2E con servidor real (`PORT=8097`, `DATA_DIR` temporal): setup + login, `200` con headers
  correctos, `counts` == filas reales, `password_hash` y `settings` en claro, `?pretty=true`
  válido e indentado, sin sesión `401`.
- `go build ./...` y `npm run build` OK. Sin dependencias nuevas.

Pendiente: prueba de carga con `bills`/`webhook_logs` masivos y revisión visual en darkmode
en el iHost (CA-006, CA-NF-001, CA-DARK).

## 6. Plan de Implementación

### 6.1 Fases

| Fase | Descripción | Estimación | Dependencias |
|------|-------------|------------|--------------|
| 1 | `ExportService` genérico con streaming + normalización + tests unitarios | 0.5 día | Ninguna |
| 2 | Handler + ruta protegida + headers + auditoría | 0.25 día | Fase 1 |
| 3 | UI Settings (tarjeta, modal de advertencia, descarga Blob, i18n es/en) | 0.5 día | Fase 2 |
| 4 | Tests de integración, verificación en darkmode y prueba en iHost | 0.25 día | Fase 3 |
| 5 | P1/P2 (pretty, gzip, sha256, copiar al portapapeles) según necesidad | 0.5 día | Fase 4 |

### 6.2 Milestones

1. **MVP**: endpoint autenticado que descarga el JSON completo (compacto) + botón y modal en
   Settings.
2. **V1.0**: `pretty`, auditoría, tests completos, verificación en iHost y darkmode.

## 7. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| Consumo de RAM por `json.Marshal` del documento completo | Alta si no se controla | Alto (OOM en iHost) | Streaming con `json.Encoder` sobre `bufio.Writer` (ADR-002) y test de carga |
| Fuga del archivo con secretos (compartido por error) | Media | Alto | Modal de advertencia, `no-store`, nombre con timestamp, auditoría en logs |
| Filas con JSON inválido en `credentials`/`raw`/`payload` | Media | Medio | Embedding condicional: JSON válido → objeto; inválido → string (REQ-011) |
| Lectura larga bloqueando escrituras del scheduler | Media | Medio | Transacción de solo lectura + orden por `id`; SQLite con WAL |
| Descarga cortada por el cliente deja archivo inválido | Baja | Medio | `X-Export-SHA256` (P2) y toast de error; el archivo truncado falla al parsear |
| Exposición del endpoint a terceros en la LAN | Baja | Alto | `authMiddleware` + cookie de sesión; sin token en URL |
| Crecimiento del navegador al manejar el Blob | Baja | Bajo | Blob + `URL.createObjectURL` y revocación inmediata; sin render en pantalla |

## 8. Notas y Referencias

- Release: versión 0.1.11 (2026-10-21). Deploy a iHost por el usuario.
- `migrations/0001`–`0006` y `internal/db/migrations.go` (tabla `schema_migrations`).
- `internal/storage/settings.go` (`GetAll`) y `internal/storage/apps.go` (credenciales en claro).
- `internal/api/routes.go` (registro con `authMiddleware`).
- `frontend/src/pages/SettingsPage.tsx` (tarjeta existente de webhooks).
- SPEC-017 (endpoint server-to-server por token): referencia de autenticación alternativa,
  descartada aquí para no exponer secretos en query string.

## 9. Historial de Cambios

| Fecha | Autor | Descripción |
|-------|-------|-------------|
| 2026-10-21 | opencode | Creación inicial de la especificación (export total a JSON desde Settings, secretos incluidos, descarga como archivo, streaming para iHost). |
| 2026-10-21 | opencode | Implementación completa (MVP + `pretty` + auditoría) y paso a `pending_release`: servicio de streaming, endpoint autenticado, UI con modal, i18n es/en y 4 tests unitarios. Verificado con `go test ./...`, `npm run build` y E2E local. |
| 2026-10-21 | opencode | **Release**: código + spec + tracker commiteados, versión 0.1.11, issue #20 cerrado con label `spec/released`. Deploy a iHost lo realiza el usuario (pendiente: prueba de carga y revisión visual en darkmode). |
