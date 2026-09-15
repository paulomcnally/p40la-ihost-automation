---
title: "Selector de plugin en creación de cuenta + formulario dinámico de credenciales"
id: "SPEC-013"
status: "released"
author: "paulomcnally"
created: "2026-09-13"
updated: "2026-09-13"
github_issue: 14
---

# Selector de plugin en creación de cuenta + formulario dinámico de credenciales

**ID**: SPEC-013  
**Estado**: released  
**Autor**: paulomcnally  
**Creado**: 2026-09-13  
**Actualizado**: 2026-09-13

---

## 1. Resumen Ejecutivo

El modal de cuenta (`AccountModal` en `frontend/src/pages/AppDetailPage.tsx`) solo carga
el `CredentialSchema` del plugin cuando se **edita** una cuenta existente que ya tiene
`plugin_name` asociado (`useEffect` con `if (!initial) return`). Al **crear** una cuenta
nueva el schema queda `null` y el formulario cae al fallback del textarea JSON libre
(`values.json_raw`), aunque la app corresponda a un plugin con schema definido
(claro, tigo, disnorte, enacal). El usuario lo verificó en local: el formulario de
"Crear nueva cuenta" en `/apps/4` (ENACAL) pide el JSON en texto plano en lugar del
formulario dinámico `username` + `contraseña`.

Este problema es una brecha de UX y de seguridad aparente: la persona espera que el
formulario refleje el plugin de la app (cada app del sistema se siembra uno-a-uno con
su plugin) y que las credenciales secretas no se escriban como JSON crudo. La causa de
fondo es que el `plugin_name` vive en la **cuenta**, no en la app, y en creación todavía
no existe.

La solución: agregar un **selector de plugin** al modal de creación. Al elegir un plugin
se carga su `CredentialSchema` (`GET /api/plugins/{name}/schema`, ya existente) y se
renderiza el formulario dinámico de campos (texto/contraseña según `Secret`). Al guardar,
el plugin elegido se asocia a la cuenta creada. Si no se elige plugin, se mantiene el
fallback del textarea JSON (comportamiento actual, necesario para apps/cuentas genéricas).
El backend valida que el plugin exista en el catálogo antes de asociarlo.

## 2. Requerimientos

### 2.1 Requerimientos Funcionales (P0 - Obligatorios)

1. **REQ-001**: En `AccountModal` en modo creación (`initial == null`), mostrar un selector (dropdown) de plugin con las opciones del catálogo (`GET /api/plugins`) más la opción "Sin plugin". Al seleccionar un plugin, cargar su schema (`GET /api/plugins/{name}/schema`) y renderizar los campos dinámicos (igual que en edición); sin plugin → textarea JSON (fallback actual).
2. **REQ-002**: Al guardar una cuenta nueva con plugin seleccionado, el `plugin_name` queda asociado a la cuenta creada y visible en la lista (badge existente). El endpoint `POST /api/apps/{id}/accounts` acepta `plugin_name` opcional y lo valida contra el catálogo (`plugins`); si no existe devuelve `plugin_not_found` (404/400).
3. **REQ-003**: Los campos secretos del schema en creación se renderizan como `type="password"` (sin botón reveal, porque no hay credenciales previas). Al editar, el comportamiento de reveal con contraseña (SPEC-011) no cambia.
4. **REQ-004**: En modo edición el comportamiento actual se mantiene intacto (schema desde `initial.plugin_name`, reveal, etc.). El selector de plugin no aparece en edición (el plugin se gestiona desde la página de facturas, `PUT /api/accounts/{id}/plugin`).
5. **REQ-005**: Traducciones nuevas en `frontend/public/i18n/{es,en}.json` (fuente de verdad): etiqueta del selector, opción "Sin plugin", estado de carga. `npm run build` sincroniza a `src/i18n`.

### 2.2 Requerimientos Funcionales (P1 - Importantes)

1. **REQ-006**: Si el plugin seleccionado no tiene schema (o falla la carga), mostrar los campos como fallback textarea (mismo fallback actual) sin romper el guardado.
2. **REQ-007**: Mantener consistencia con `buildCredentials`: cuando hay schema se construye el objeto desde los campos; sin schema se parsea `json_raw`. La validación de campos requeridos del schema aplica también en creación.

### 2.3 Requerimientos Funcionales (P2 - Deseables)

1. **REQ-008**: Preseleccionar el plugin "natural" de la app en el selector si la app tiene una sola cuenta con plugin (heurística opcional). No bloquea el MVP.

### 2.4 Requerimientos No Funcionales

- **Rendimiento**: Una llamada extra a `GET /api/plugins` + `GET /api/plugins/{name}/schema` al abrir el modal en creación; despreciable en iHost.
- **Seguridad**: El `plugin_name` enviado en create se valida contra el catálogo (nunca se persiste un nombre arbitrario). Las credenciales siguen sin exponerse; el password solo viaja en el body del `POST` (HTTPS).
- **Almacenamiento**: Cero migraciones nuevas; reusa `accounts.plugin_name`.
- **Disponibilidad**: Rutas detrás de `authMiddleware`; `/health` sin cambios.
- **iHost**: Sin dependencias nuevas (Go stdlib + endpoints existentes). Sin cambios de esquema.

## 3. Investigación y Decisiones Técnicas

### 3.1 Contexto investigado

- `frontend/src/pages/AppDetailPage.tsx:32-45`: el `useEffect` de carga de schema retorna
  si `!initial` → en creación `schema` queda `null` → fallback textarea (líneas 219-232).
- El catálogo de plugins está disponible en `GET /api/plugins` (autenticado) y el schema
  en `GET /api/plugins/{name}/schema` (SPEC-011).
- `internal/api/apps_handlers.go:135-163`: `accountRequest` solo acepta `identifier`,
  `label`, `credentials`. `CreateAccount` delega en `AppsService.CreateAccount`
  (`internal/services/apps.go:101`) que valida credenciales JSON pero no toca plugin.
- `internal/storage/apps.go:121`: `CreateAccount` inserta sin `plugin_name`.
- `internal/storage/plugins.go:89`: `SetAccountPlugin` existe para asociar el plugin
  post-creación (usado hoy desde la página de facturas).
- Asociación app↔plugin: las apps se siembran una por proveedor (ver `scripts/seed-apps.sh`),
  pero el plugin vive en `accounts.plugin_name` (nullable), por eso en creación no se conoce.

### 3.2 Opciones evaluadas

| Opción | Pros | Contras | Decisión |
|--------|------|---------|----------|
| Selector de plugin en el modal + `plugin_name` opcional en `POST /api/apps/{id}/accounts` | Operación atómica, validada en backend; el formulario dinámico sale gratis reusando schema existente | Cambio pequeño en handler + service + storage | ✅ Seleccionada |
| Crear sin plugin y llamar después `PUT /api/accounts/{id}/plugin` desde el frontend | Cero cambios backend | Dos llamadas (parcialidad si falla la segunda); validación solo indirecta | ❌ Rechazada |
| Inferir el plugin desde la app (columna `apps.plugin_name`) | Selector innecesario | Migración nueva + rompe el modelo "plugin por cuenta" (una app puede tener cuentas con plugins distintos) | ❌ Rechazada |
| No hacer nada (mantener textarea en creación) | Cero trabajo | Brecha de UX/seguridad reportada por el usuario | ❌ Rechazada |

### 3.3 Decisiones arquitectónicas (ADRs)

**ADR-001**: El `plugin_name` es opcional en la creación
- **Contexto**: Existen apps/cuentas genéricas sin plugin (textarea JSON) y el flujo actual
  permite asociar el plugin después desde la página de facturas.
- **Decisión**: `POST /api/apps/{id}/accounts` acepta `plugin_name` opcional; si viene, se
  valida contra el catálogo `plugins` y se persiste en el INSERT. Si no viene, se crea la
  cuenta sin plugin (compatibilidad total).
- **Consecuencias**: Un solo endpoint cubre ambos flujos; la validación evita plugins inventados.

**ADR-002**: Selector de plugin solo en modo creación
- **Contexto**: En edición el plugin ya está asociado a la cuenta y el schema se deriva de él;
  cambiar el plugin de una cuenta con historial de `bills` es responsabilidad de la página de
  facturas (`PUT /api/accounts/{id}/plugin`).
- **Decisión**: El dropdown se muestra únicamente cuando `initial == null`. En edición, cero cambios.
- **Consecuencias**: Superficie de cambio mínima; sin riesgo de romper el flujo de reveal de SPEC-011.

## 4. Diseño Técnico

### 4.1 Diagrama de arquitectura

```
[AccountModal (creación)] --GET /api/plugins--> [Go API]
        |                                              |
        |--GET /api/plugins/{name}/schema-->          |
        |                                              v
        |--POST /api/apps/{id}/accounts {plugin_name}--> [AppsService.CreateAccount]
        |                                              |
        v                                              v
[selector + campos dinámicos o textarea]        [PluginsStorage: valida catálogo + INSERT con plugin_name]
```

### 4.2 Componentes

#### 4.2.1 Backend: `POST /api/apps/{id}/accounts`
- **Responsabilidad**: Aceptar `plugin_name` opcional, validarlo contra el catálogo y persistirlo al crear la cuenta.
- **Interfaz**: `accountRequest` suma `PluginName *string json:"plugin_name,omitempty"`.
- **Dependencias**: `PluginsStorage.GetPlugin` (catálogo). El `AppsService.CreateAccount` recibe el `pluginName` opcional y valida.
- **Ubicación**: `internal/api/apps_handlers.go`, `internal/services/apps.go`, `internal/storage/apps.go`.

#### 4.2.2 Frontend: `AccountModal` (AppDetailPage.tsx)
- **Responsabilidad**: En creación, cargar catálogo de plugins, mostrar selector, cargar schema al elegir y renderizar campos dinámicos; enviar `plugin_name` en el POST.
- **Interfaz**: Reusa `api.plugins.list()`, `api.plugins.schema()`, `api.apps.createAccount()` (extendido con `plugin_name`).
- **Dependencias**: Endpoints existentes; componentes y tokens de tema actuales.

### 4.3 Modelo de datos

```
Sin cambios de esquema.

accounts.plugin_name (columna existente, nullable)
  - creación con plugin → se setea en el INSERT
  - creación sin plugin  → NULL (flujo actual)
```

### 4.4 APIs / Contratos

#### Endpoint: `POST /api/apps/{id}/accounts`

**Request** (nuevo campo opcional):
```json
{
  "identifier": "<NIC>",
  "label": "Casa",
  "plugin_name": "enacal.nicaragua",
  "credentials": { "username": "x", "password": "y" }
}
```

**Response 201** (igual a hoy; `plugin_name` poblado si se envió):
```json
{
  "id": 14,
  "app_id": 4,
  "identifier": "<NIC>",
  "label": "Casa",
  "plugin_name": "enacal.nicaragua"
}
```

**Response Error**:
```json
{
  "error": "plugin_not_found",
  "message": "plugin no encontrado"
}
```

**Compatibilidad**: `plugin_name` ausente → mismo comportamiento que hoy (cuenta sin plugin).

### 4.5 Dependencias

- **Internas**: `internal/plugins` (schema), `internal/storage` (plugins/apps), `internal/api` (handlers).
- **Externas**: Ninguna nueva.

## 5. Criterios de Aceptación

### 5.1 Funcionales

- [ ] CA-001: Dado un modal de creación, cuando se abre, hay un selector de plugin con las opciones del catálogo + "Sin plugin".
- [ ] CA-002: Dado un plugin seleccionado en creación, cuando se elige, el formulario muestra los campos dinámicos del `CredentialSchema` (secretos como `type="password"`, sin botón reveal) en vez del textarea JSON.
- [ ] CA-003: Dado "Sin plugin" seleccionado en creación, el formulario muestra el textarea JSON (fallback actual) y guarda sin `plugin_name`.
- [ ] CA-004: Dado un plugin seleccionado y guardado, la cuenta creada tiene `plugin_name` persistido y visible como badge en la lista de cuentas.
- [ ] CA-005: Dado un `plugin_name` inexistente en el `POST /api/apps/{id}/accounts`, la API responde `plugin_not_found` y no crea la cuenta.
- [ ] CA-006: Dado el modo edición, el comportamiento no cambia (schema desde `initial.plugin_name`, reveal con contraseña de SPEC-011 intacto, sin selector).
- [ ] CA-007: Dado un plugin con campos requeridos vacíos en creación, el guardado se bloquea con el mensaje existente de validación.
- [ ] CA-DARK: Los inputs/select del modal usan tokens del tema (`bg-card`, `text-text`, `text-text-secondary`) y se verificó legibilidad en darkmode.
- [ ] CA-BACK: N/A (no crea páginas nuevas; el modal ya usa el header existente).

### 5.2 No funcionales

- [ ] CA-NF-001: `go build ./...` compila sin errores.
- [ ] CA-NF-002: `go test ./...` pasa (agregar test del handler/service si aplica).
- [ ] CA-NF-003: `npm run build` pasa (typecheck + i18n sync).
- [ ] CA-NF-004: Sin dependencias nuevas en `go.mod` ni `package.json`.

### 5.3 Testing

- **Unit tests**: Service `CreateAccount` con y sin `plugin_name`; validación de `plugin_not_found` (handler/service).
- **Integration tests**: `POST /api/apps/{id}/accounts` con `plugin_name` válido → cuenta creada con plugin; con plugin inexistente → error sin INSERT.
- **E2E tests**: En local (`/apps/4`), crear cuenta ENACAL con el selector → formulario dinámico → guardado → badge con plugin; luego `bills:fetch` funciona.
- **Carga/Performance**: Sin impacto medible.

## 6. Plan de Implementación

### 6.1 Fases

| Fase | Descripción | Estimación | Dependencias |
|------|-------------|------------|--------------|
| 1 | Esta spec | 0.25 día | Ninguna |
| 2 | Backend: `plugin_name` opcional en `POST /api/apps/{id}/accounts` (handler + service + storage + validación catálogo + test) | 0.5 día | Fase 1 |
| 3 | Frontend: selector de plugin en creación + carga de schema + campos dinámicos + envío de `plugin_name` + i18n | 0.5 día | Fase 2 |
| 4 | Verificación: `go build`, `go test`, `npm run build`, prueba E2E local en `/apps/4` | 0.25 día | Fase 3 |

### 6.2 Milestones

- **MVP**: Crear cuenta con selector de plugin → formulario dinámico → plugin persistido al crear. Fallback textarea intacto.
- **V1.1** (opcional): REQ-008 preselección heurística.

## 7. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| Romper el flujo de edición/reveal (SPEC-011) | Baja | Alto | REQ-004: cero cambios en modo edición; verificar E2E edición + reveal |
| Romper el fallback textarea para apps sin plugin | Baja | Medio | REQ-003/CA-003: "Sin plugin" mantiene el flujo actual exacto |
| Plugin inválido persistido | Baja | Medio | Validación backend contra catálogo (CA-005) |
| Carga de schema lenta en iHost | Baja | Bajo | Endpoint ya existente; loading state en el modal |

## 8. Notas y Referencias

- Reportado por el usuario: modal "Crear nueva cuenta" en `/apps/4` muestra el textarea JSON (ENACAL).
- Código relevante: `frontend/src/pages/AppDetailPage.tsx` (AccountModal), `internal/api/apps_handlers.go`, `internal/services/apps.go`, `internal/storage/apps.go`, `internal/storage/plugins.go`.
- Reutiliza: SPEC-011 (`CredentialSchema` + `GET /api/plugins/{name}/schema` + reveal), SPEC-002 (cuentas/credenciales), endpoints `GET /api/plugins` y `PUT /api/accounts/{id}/plugin`.
- Repo: `https://github.com/paulomcnally/p40la-ihost-automation`

## 9. Historial de Cambios

| Fecha | Autor | Descripción |
|-------|-------|-------------|
| 2026-09-13 | paulomcnally | Creación inicial. Problema reportado: el modal de creación muestra el textarea JSON en vez del formulario dinámico del plugin porque el schema solo se carga en edición (`if (!initial) return` en `AppDetailPage.tsx`). Solución: selector de plugin en creación + `plugin_name` opcional validado en `POST /api/apps/{id}/accounts`. |
| 2026-09-13 | paulomcnally | Implementación completa. Backend: `accountRequest.PluginName` opcional en `POST /api/apps/{id}/accounts`, `AppsService.CreateAccount` valida contra el catálogo (`PluginsStorage.GetPlugin`) y persiste `plugin_name` en el INSERT (`internal/storage/apps.go`); error `plugin_not_found` (404) en `mapAppsError`; `main.go` inyecta `pluginsStorage` en `NewAppsService`. Frontend: `AccountModal` en modo creación carga el catálogo (`GET /api/plugins`), muestra selector "Sin plugin"/catálogo, carga el schema al elegir (`GET /api/plugins/{name}/schema`) y renderiza campos dinámicos (secretos como `type="password"`, sin reveal); envía `plugin_name` en el POST; edición intacta. i18n es/en: `apps.plugin`, `apps.plugin_none`. Tests: `apps_test.go` (sin plugin, plugin válido, plugin inexistente, plugin vacío). Verificado: `go build`, `go vet`, `go test ./...`, `npm run build` OK; E2E local (server temporal) confirma cuenta con plugin persistido y `plugin_not_found` sin INSERT. Estado → `pending_release`. |
| 2026-09-13 | paulomcnally | Ajuste iterativo post-revisión del usuario: el selector pasa a usar el componente `Dropdown` del sistema (no `<select>` nativo) y el área de credenciales queda oculta hasta elegir un plugin (`credentials_plugin_hint`); sin plugin, `buildCredentials` devuelve `{}` y la cuenta se crea sin credenciales/plugin. Verificado por el usuario en local (`http://localhost:8089/apps/4`). |
| 2026-09-13 | paulomcnally | **Release** (cerrada por decisión del usuario; el deploy a iHost lo realiza él). Probado en local por el usuario. Issue #14 cerrado con label `spec/released`. Commits: implementación + release (docs + tracker). |