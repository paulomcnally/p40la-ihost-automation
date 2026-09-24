---
title: "Endpoint de run manual de webhook por cuenta (webhook:run) con auth por token"
id: "SPEC-017"
status: "released"
author: "opencode"
created: "2026-09-24"
updated: "2026-09-24"
github_issue: 19
---

# Endpoint de run manual de webhook por cuenta (webhook:run) con auth por token

**ID**: SPEC-017  
**Estado**: released  
**Autor**: opencode  
**Creado**: 2026-09-24  
**Actualizado**: 2026-09-24

---

## 1. Resumen Ejecutivo

`p40la-ihost` (proyecto hermano) necesita disparar la sincronización manual de facturas desde su propia UI y su bot de Telegram (SPEC-092 del repo p40la-ihost). Hoy la única vía HTTP para ejecutar el job de una cuenta (`WebhookService.RunJob` = llamado externo al plugin + entrega al webhook) es `POST /api/accounts/{accountId}/webhook:test`, que está detrás de `AuthMiddleware` (sesión), inapropiada para integraciones server-to-server sin login.

Esta spec agrega **`POST /api/accounts/{accountId}/webhook:run`**: el mismo job de `RunJob` pero autenticado por **token** en el header `X-Webhook-Key` (la api_key de webhooks del propio automation), sin requerir sesión. `p40la-ihost` guarda `automation_base_url` + `automation_api_key` y llama a este endpoint de forma stateless.

**Consideraciones iHost**: backend-only, sin dependencias nuevas (stdlib `crypto/subtle`). Una ruta nueva + middleware de token. Memoria/CPU despreciables.

## 2. Requerimientos

### 2.1 Requerimientos Funcionales (P0 - Obligatorios)

1. **REQ-001**: Nuevo endpoint `POST /api/accounts/{accountId}/webhook:run` que ejecuta `WebhookService.RunJob(ctx, accountID)` (fetch del plugin + envío al webhook) y responde `{ "delivered": n, "failed": m }`.
2. **REQ-002**: Autenticación por token: header `X-Webhook-Key` debe coincidir con la api_key de webhooks (`settings.webhook_api_key`) usando comparación en tiempo constante (`crypto/subtle`). Sin token o token incorrecto → `401`.
3. **REQ-003**: El endpoint se registra en `routes.go` FUERA de `AuthMiddleware`.
4. **REQ-004**: Reutiliza el mapeo de errores existente (`mapWebhookError`) para errores de dominio (cuenta inexistente, webhooks deshabilitados, sin api_key, sin webhook_url, sin plugin).

### 2.2 Requerimientos Funcionales (P1 - Importantes)

1. **REQ-005**: Tests del endpoint: `401` sin header/key incorrecta, `200` con `{ delivered, failed }`, propagación de errores de `RunJob`.

### 2.3 Requerimientos No Funcionales

- **Rendimiento**: 1 ruta + 1 check de token; sin impacto medible.
- **Seguridad**: Token en header, comparación constante. El token coincide con la api_key de webhooks (una sola clave para entrada/salida).
- **iHost**: Sin dependencias externas nuevas.

## 3. Investigación y Decisiones Técnicas

### 3.1 Contexto investigado

- `WebhookService.RunJob` (`internal/services/webhook.go`) ya hace el fetch del plugin y la entrega al webhook; valida `IsWebhookEnabled` y `GetWebhookAPIKey`.
- `webhook:test` (`internal/api/webhook_handlers.go`) ejecuta `RunJob` pero está bajo `AuthMiddleware`.
- La api_key de webhooks se guarda en settings (`SettingWebhookAPIKey`) y se gestiona via `GET/PUT /api/settings/webhook`.
- Es parte de SPEC-092 del repo `p40la-ihost` (contrato documentado ahí).

### 3.2 Opciones evaluadas

| Opción | Pros | Contras | Decisión |
|--------|------|---------|----------|
| `webhook:run` con token `X-Webhook-Key` (reutiliza la api_key de webhooks) | Auth stateless; una sola clave; sin credenciales de usuario | Token compartido con la salida de webhooks | ✅ Seleccionada |
| Reutilizar `webhook:test` con sesión | Cero cambios | Requiere login/cookies; inapropiado server-to-server | ❌ Rechazada |
| Nueva api_key dedicada de sync | Aislamiento | Clave adicional que administrar | ❌ Rechazada |

### 3.3 Decisiones arquitectónicas (ADRs)

**ADR-001**: Endpoint de run con token = api_key de webhooks.
- **Contexto**: p40la-ihost debe disparar el job sin sesión.
- **Decisión**: `POST /api/accounts/{accountId}/webhook:run` valida `X-Webhook-Key` contra `settings.webhook_api_key` con `subtle.ConstantTimeCompare`.
- **Consecuencias**: Una sola clave para el flujo de webhooks (entrada y salida). Si se regenera la api_key, p40la-ihost debe actualizar su `automation_api_key`.

## 4. Diseño Técnico

### 4.1 Diagrama de arquitectura

```
[p40la-ihost] --POST /api/accounts/{id}/webhook:run, X-Webhook-Key: <key>--> [Server Go automation]
                                                                               ├── RunWebhook (token check)
                                                                               └── WebhookService.RunJob
                                                                                     ├── plugin.FetchBills
                                                                                     └── POST webhook_url
```

### 4.2 Componentes

#### 4.2.1 `internal/api/webhook_handlers.go` (extender)
- **Responsabilidad**: Handler `RunWebhook` (`POST /api/accounts/{accountId}/webhook:run`): valida `X-Webhook-Key` contra la api_key y ejecuta `RunJob`.
- **Dependencias**: `WebhookService`, `AppSettingsService`, `crypto/subtle`.

#### 4.2.2 `internal/api/routes.go` (extender)
- **Responsabilidad**: Registrar `POST /api/accounts/{accountId}/webhook:run` fuera de `AuthMiddleware`.

## 5. Criterios de Aceptación

### 5.1 Funcionales

- [ ] CA-001: Dado una cuenta con webhook configurado, cuando se llama `POST /api/accounts/{id}/webhook:run` con `X-Webhook-Key` correcto, entonces ejecuta `RunJob` y responde `200` con `{ delivered, failed }`.
- [ ] CA-002: Dado un request sin header o con key incorrecta, cuando se llama el endpoint, entonces responde `401` y no ejecuta nada.
- [ ] CA-003: Dado una cuenta inexistente o errores de dominio (webhooks deshabilitados, sin api_key, sin webhook_url, sin plugin), cuando se llama el endpoint, entonces responde `400`/`404` con el mapeo existente.

### 5.2 No funcionales

- [ ] CA-NF-001: Sin dependencias externas nuevas; `go build` y `go test ./...` pasan.

### 5.3 Testing

- **Unit tests**: auth por token (401 sin header/key incorrecta), `200` con resultado, propagación de errores de `RunJob` (httptest).

## 6. Plan de Implementación

### 6.1 Fases

| Fase | Descripción | Estimación | Dependencias |
|------|-------------|------------|--------------|
| 1 | Handler `RunWebhook` + auth por token + ruta | 0.5 día | Ninguna |
| 2 | Tests | 0.5 día | Fase 1 |

## 7. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| Regenerar la api_key de webhooks rompe la integración | Media | Medio | Documentar; p40la-ihost actualiza `automation_api_key`. |
| Token compartido entrada/salida | Baja | Bajo | Aceptable para uso personal interno. |

## 8. Notas y Referencias

- Coordinado con SPEC-092 del repo `p40la-ihost`.
- Base: `WebhookService.RunJob` y `webhook:test` en `internal/api/webhook_handlers.go`.
- `mapWebhookError` en `internal/api/webhook_handlers.go`.

## 9. Historial de Cambios

| Fecha | Autor | Descripción |
|-------|-------|-------------|
| 2026-09-24 | opencode | Creación inicial de la especificación |
| 2026-09-24 | opencode | Implementación: handler `RunWebhook` (auth por token `X-Webhook-Key` con `crypto/subtle`), ruta `POST /api/accounts/{accountId}/webhook:run` fuera de `AuthMiddleware`, tests (`webhook_run_handlers_test.go`). Validación local E2E OK (200 `{delivered,failed}`, 401 sin header/key incorrecta, 400 errores de dominio). Commit `a2c8ea0` en rama `feature/SPEC-017-webhook-run`. |
| 2026-09-24 | opencode | **Release**: merge a main, versión 0.1.10, issue #19 cerrado. Consumido por SPEC-092 del repo `p40la-ihost` (sincronización manual de servicios). |