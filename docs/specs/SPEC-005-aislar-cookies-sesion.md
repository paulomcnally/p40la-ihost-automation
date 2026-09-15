---
title: "Aislar cookies de sesión entre proyectos"
id: "SPEC-005"
status: "released"
author: "paulomcnally"
created: "2026-09-12"
updated: "2026-09-12"
github_issue: 5
---

# Aislar cookies de sesión entre proyectos

**ID**: SPEC-005
**Estado**: released
**Autor**: paulomcnally
**Creado**: 2026-09-12
**Actualizado**: 2026-09-12

---

## 1. Resumen Ejecutivo

Al ejecutar localmente el proyecto hermano `p40la-ihost` (puerto `8088`) y este proyecto `p40la-ihost-automation` (puerto `8089`) en el mismo navegador, iniciar sesión en uno cierra la sesión del otro, y viceversa. El usuario reporta que al hacer login en `p40la-ihost` se pierde la sesión en esta app en Chrome.

La causa es una colisión de cookies: ambas aplicaciones setean la cookie de sesión con el mismo nombre (`session`), con `Path: "/"` y sin dominio restrictivo. Las cookies no están aisladas por puerto: el navegador las comparte por host. Al hacer login en la app `p40la-ihost`, la cookie `session` se sobrescribe y, como los secretos de firma son distintos por proyecto, la sesión de la otra app se vuelve inválida (además el logout con `MaxAge: -1` la borra por completo).

La solución es aislar el espacio de nombres de cookies de este proyecto, usando un nombre de cookie propio y configurable (ej. `p40la_ihost_automation_session`) que no colisione con el de la app hermana. Esto no requiere migración de datos ni nuevos almacenamientos, es un cambio pequeño de configuración y código en Go, de bajo impacto en memoria/SQLite (cumple con las restricciones de iHost: cero dependencias nuevas).

## 2. Requerimientos

### 2.1 Requerimientos Funcionales (P0 - Obligatorios)

1. **REQ-001**: La cookie de sesión de `p40la-ihost-automation` debe tener un nombre propio y distinto al de `p40la-ihost` (`session`) para evitar colisiones en el navegador cuando ambas apps corren en el mismo host.
2. **REQ-002**: El nombre de la cookie debe ser configurable vía variable de entorno (ej. `COOKIE_NAME`) con un valor por defecto seguro para este proyecto, sin requerir cambios de código para reconfigurarlo.
3. **REQ-003**: Todos los puntos de la app que leen o escriben la cookie de sesión (servicio de autenticación, middleware de auth, handlers de login/logout) deben usar la misma fuente de verdad (config) y nunca el string hardcodeado.

### 2.2 Requerimientos Funcionales (P1 - Importantes)

1. **REQ-004**: Al cambiar el nombre de la cookie, las sesiones activas con el nombre anterior se invalidan una sola vez (login de nuevo) sin errores en crasheos; documentar el efecto.

### 2.3 Requerimientos Funcionales (P2 - Deseables)

1. **REQ-005**: Documentar en `AGENTS.md` o README que la cookie de sesión es específica por proyecto y que `p40la-ihost` usa `session`.

### 2.4 Requerimientos No Funcionales

- **Rendimiento**: Sin impacto medible; solo cambia un string en headers.
- **Seguridad**: Mantener `HttpOnly`, `SameSiteStrict`, `Secure` según config; no debilitar ninguna propiedad de la cookie.
- **Almacenamiento**: Sin cambios en SQLite ni disco.
- **Disponibilidad**: Sin cambios de rutas; solo afecta a autenticación.
- **iHost**: Sin dependencias nuevas, sin librerías extra; solo Go estándar. Consumo de RAM/CPU despreciable.

## 3. Investigación y Decisiones Técnicas

### 3.1 Contexto investigado

- `p40la-ihost-automation` define `cookieName = "session"` en `internal/services/auth.go:26` y lo usa para crear/borrar la cookie (`createSessionCookie`, `Logout`). El middleware lee `r.Cookie("session")` en `internal/api/middleware.go:19`.
- `p40la-ihost` (repo hermano) usa exactamente el mismo nombre `session` (`internal/services/auth.go:26` y `internal/api/middleware.go:34,77`) con `Path: "/"`.
- Ambas apps corren en `localhost` en puertos distintos (8088/8089). Las cookies HTTP se comparten por host y path, no por puerto → colisión confirmada.

### 3.2 Opciones evaluadas

| Opción | Pros | Contras | Decisión |
|--------|------|---------|----------|
| Renombrar la cookie a un nombre fijo propio (ej. `p40la_ihost_automation_session`) | Simple, cero config | Fijo por código, menos flexible | ❌ Rechazada |
| Hacer el nombre configurable vía env `COOKIE_NAME` con default propio | Flexible, hereda patrón de config existente (`getEnv`) | Mínimo overhead de config | ✅ Seleccionada |
| Cambiar el `Path` de la cookie a un subpath distinto por app | Aislaría por path | Path restrictivo rompe rutas SPA/API; frágil | ❌ Rechazada |
| Cambiar dominio a un subdominio | Aísla | Requiere DNS/subdominios en dev local; inviable en localhost | ❌ Rechazada |

### 3.3 Decisiones arquitectónicas (ADRs)

**ADR-001**: Nombre de cookie configurable vía entorno
- **Contexto**: Dos apps del mismo host comparten `session`; necesitamos un nombre propio sin romper despliegues existentes.
- **Decisión**: Agregar `CookieName` a `Config`, cargado de `COOKIE_NAME` con default `p40la_ihost_automation_session`. El servicio `AuthService` y el middleware leen el nombre desde config.
- **Consecuencias**: Positivas: aislado por default y reconfigurable; negativo: una única invalidación de sesiones activas al primer deploy.

## 4. Diseño Técnico

### 4.1 Diagrama de arquitectura

```
[Browser] --cookie: p40la_ihost_automation_session--> [p40la-ihost-automation :8089]
[Browser] --cookie: session------------------------> [p40la-ihost :8088]
```

### 4.2 Componentes

#### 4.2.1 Config (`internal/config/config.go`)
- **Responsabilidad**: Exponer `CookieName string` cargado de `COOKIE_NAME` (default `p40la_ihost_automation_session`).
- **Interfaz**: Nuevo campo en `Config`.
- **Dependencias**: Ninguna nueva.
- **Ubicación**: `internal/config/config.go`.

#### 4.2.2 AuthService (`internal/services/auth.go`)
- **Responsabilidad**: Usar `s.cfg.CookieName` en lugar de la constante `cookieName` al crear/expirar cookies.
- **Interfaz**: Sin cambios públicos.
- **Dependencias**: `config.Config`.
- **Ubicación**: `internal/services/auth.go`.

#### 4.2.3 Middleware de auth (`internal/api/middleware.go`)
- **Responsabilidad**: Leer la cookie por su nombre configurado (`r.Cookie(s.cfg.CookieName)`).
- **Interfaz**: Sin cambios públicos; recibe config ya inyectada.
- **Dependencias**: `config.Config`.
- **Ubicación**: `internal/api/middleware.go`.

### 4.3 Modelo de datos

Sin cambios. No hay migración SQL.

### 4.4 APIs / Contratos

Sin cambios en la API HTTP. Solo cambian headers `Set-Cookie`/request cookies internamente.

### 4.5 Dependencias

- **Internas**: `internal/config`, `internal/services`, `internal/api` (middleware).
- **Externas**: Ninguna. Solo Go estándar (`net/http`).

## 5. Criterios de Aceptación

### 5.1 Funcionales

- [ ] CA-001: Dado un navegador con sesión activa en `p40la-ihost` (cookie `session`) y en esta app, cuando se hace login en `p40la-ihost-automation`, entonces la sesión de `p40la-ihost` permanece válida y viceversa.
- [ ] CA-002: Dado el default de config, cuando la app arranca sin `COOKIE_NAME`, entonces la cookie setea se llama `p40la_ihost_automation_session`.
- [ ] CA-003: Dado `COOKIE_NAME=misession` en el entorno, cuando la app arranca, entonces la cookie se llama `misession` (configurado sin tocar código).
- [ ] CA-004: Dado un usuario con sesión previa con cookie `session`, cuando se actualiza a la nueva versión, entonces se le pide login una única vez (sesión antigua inválida, sin crash).
- [ ] CA-005: Dado login + logout, cuando el usuario hace logout, entonces la cookie `p40la_ihost_automation_session` se borra con `MaxAge: -1`.

### 5.2 No funcionales

- [ ] CA-NF-001: La cookie conserva `HttpOnly`, `SameSite: Strict` y `Secure` según config; no se degradan propiedades de seguridad.

### 5.3 Testing

- **Unit tests**: Ampliar/ajustar `internal/services/auth_test.go` para verificar que el nombre de cookie usado es `s.cfg.CookieName` (creación, logout y validación).
- **Integration tests**: Flujo login → request protegido → logout con nombre configurado y con nombre custom.
- **E2E tests**: Manual en Chrome: sesión simultánea en `p40la-ihost` (8088) y esta app (8089) sin que se cierren mutuamente.
- **Carga/Performance**: No aplica.

## 6. Plan de Implementación

### 6.1 Fases

| Fase | Descripción | Estimación | Dependencias |
|------|-------------|------------|--------------|
| 1 | Agregar `CookieName` a `Config` con env `COOKIE_NAME` y default propio | 0.1 días | Ninguna |
| 2 | Usar `cfg.CookieName` en `AuthService` y en middleware | 0.1 días | Fase 1 |
| 3 | Actualizar tests de auth y verificar `go build ./...` | 0.2 días | Fase 2 |
| 4 | Documentar en `AGENTS.md`/README la cookie por proyecto | 0.1 días | Fase 3 |

### 6.2 Milestones

1. **MVP**: Fases 1-3: cookie aislada y configurable.
2. **V1.0**: Fase 4: documentación completa.

## 7. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| Sesiones activas se invalidan una vez al cambiar el nombre | Alta | Bajo | Comunicar en changelog; es un único re-login |
| Queda algún string `"session"` hardcodeado sin migrar | Media | Medio | Búsqueda global (`grep`) de `"session"` en handlers/middleware antes de release |
| Otro proyecto futuro use el mismo nombre nuevo | Baja | Bajo | Documentar el default en AGENTS.md |

## 8. Notas y Referencias

- Repo hermano: `p40la-ihost` (cookie `session`, puerto 8088).
- Archivos a tocar: `internal/config/config.go`, `internal/services/auth.go`, `internal/api/middleware.go`, `internal/services/auth_test.go`.
- Las cookies no se aíslan por puerto en navegadores: el espacio de nombres es host+path.

## 9. Historial de Cambios

| Fecha | Autor | Descripción |
|-------|-------|-------------|
| 2026-09-12 | paulomcnally | Creación inicial de la especificación |
| 2026-09-12 | paulomcnally | Implementación: `CookieName` en config (env `COOKIE_NAME`, default `p40la_ihost_automation_session`), uso en AuthService y middleware, tests de auth, doc en AGENTS.md |
| 2026-09-12 | paulomcnally | Release: commit `ad71a58` (SPEC-005). Cookie de sesión aislada como `p40la_ihost_automation_session` en main. |