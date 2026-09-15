---
title: "Plugins en módulo privado con dominios ofuscados"
id: "SPEC-015"
status: "released"
author: "paulomcnally"
created: "2026-09-15"
updated: "2026-09-15"
github_issue: 16
---

# Plugins en módulo privado con dominios ofuscados

**ID**: SPEC-015  
**Estado**: released  
**Autor**: paulomcnally  
**Creado**: 2026-09-15  
**Actualizado**: 2026-09-15

---

## 1. Resumen Ejecutivo

El repositorio `p40la-ihost-automation` es público y hoy contiene, en código,
tests y scripts, los dominios reales de los proveedores integrados (Tigo, Claro,
ENACAL, DISNORTE-DISSUR y ASSA) junto con el know-how de cada integración
(endpoints, headers, flujos de auth, bypass de captcha). Además, la imagen Docker
publicada en Docker Hub es pública, por lo que los literales de dominio también
pueden recuperarse con `strings` desde el binario.

Esta spec mueve el contrato de plugins (`Plugin`, `Bill`, `CredentialField`,
errores y `Registry`) y las 5 implementaciones a un **repositorio privado**
(`github.com/paulomcnally/p40la-ihost-automation-plugins`) consumido como módulo
Go con tag semver. Dentro del módulo privado los dominios se guardan
**cifrados** (AES-256-GCM, llave embebida solo en ese repo) y se descifran en
runtime, de modo que el binario y la imagen pública no contienen dominios
legibles.

Resultado esperado: el repo público deja de exponer los dominios en el código y
la imagen Docker solo contiene ciphertext, manteniendo el build local y el de CI
funcionando con un token de GitHub.

---

## 2. Requerimientos

### 2.1 Requerimientos Funcionales (P0 - Obligatorios)

1. **REQ-001**: Crear el repo privado `github.com/paulomcnally/p40la-ihost-automation-plugins` con módulo Go propio y estructura:
   - `plugins/` — contrato movido desde `internal/plugins/types.go` + `registry.go`.
   - `secret/` — helper AES-256-GCM + llave embebida (solo en este repo).
   - `claro/nicaragua/`, `tigo/nicaragua/`, `disnorte/dissur/nicaragua/`, `enacal/nicaragua/`, `assa/nicaragua/` — implementaciones + tests movidos.
   - `all/` — `RegisterAll(r *plugins.Registry)`.
   - `cmd/gensecret/` — utilidad de desarrollo para cifrar literales.
   - `scripts/tigo-auth.sh` — movido desde el repo público.
2. **REQ-002**: Ofuscar todos los literales de dominio/URL de proveedor en el módulo privado: cada base se guarda como blob Base64 cifrado con AES-256-GCM y se descifra con `secret.Must(...)` al construir el plugin. Las rutas se concatenan a la base descifrada. El helper debe exponer `Encrypt`/`Decrypt` y una llave partida en dos constantes XOR-eadas al runtime (ofuscación, no criptografía fuerte; ver Riesgos).
3. **REQ-003**: El repo público consume el módulo privado:
   - `go.mod` con `require github.com/paulomcnally/p40la-ihost-automation-plugins vX.Y.Z`.
   - `internal/plugins` se reduce a aliases (`type Plugin = pplugins.Plugin`, `var ErrAuthFailed = pplugins.ErrAuthFailed`, etc.) para no tocar `services`/`api`.
   - `cmd/server/main.go` registra con `all.RegisterAll(registry)` y elimina los imports de implementaciones.
   - Eliminar `internal/plugins/{claro,tigo,disnorte,enacal,assa}` del repo público.
4. **REQ-004**: Build autenticado:
   - `Dockerfile` (stage `backend-builder`) con `GOPRIVATE=github.com/paulomcnally/p40la-ihost-automation-plugins` y `RUN --mount=type=secret,id=gh_token` para `go mod download` (nunca `ARG`/`ENV` con el token).
   - `.github/workflows/docker-publish.yml` pasa `secrets: gh_token=${{ secrets.PLUGINS_TOKEN }}` al `docker/build-push-action`.
   - No usar `go mod vendor` en el repo público (el vendor volvería a exponer el código privado).
5. **REQ-005**: Documentar el setup local: `go env -w GOPRIVATE=github.com/paulomcnally/p40la-ihost-automation-plugins` y `gh auth setup-git` (o PAT con scope `repo`), en `AGENTS.md` y README.
6. **REQ-006**: Redactar dominios en el repo público:
   - `scripts/seed-apps.sh`: eliminar `PLUGIN_SOURCE` por defecto con dominio y los ejemplos con dominios; `PLUGIN_SOURCE` pasa a ser obligatorio (o placeholder).
   - `scripts/tigo-auth.sh`: se mueve al repo privado (contiene dominios y client id de Tigo).
7. **REQ-007**: Versionado: tag semver `v1.0.0` en el repo privado; el repo público fija la versión en `go.mod`/`go.sum` (nunca pseudo-versiones de rama).

### 2.2 Requerimientos Funcionales (P1 - Importantes)

1. **REQ-008**: CI propio en el repo privado (GitHub Actions): `go build ./...`, `go vet ./...` y `go test ./...` en push/PR.
2. **REQ-009**: `scripts/build.sh` y `scripts/release.sh` fallan con mensaje claro si el build no puede resolver el módulo privado (sin token o sin `GOPRIVATE`), en lugar de un error críptico de Go.

### 2.3 Requerimientos Funcionales (P2 - Deseables)

1. **REQ-010**: Evaluar (en spec aparte) dejar de persistir/exponer `plugins.source` y `bills.source` (dominio en claro en SQLite y en `GET /api/plugins`), ya que hoy el runtime autenticado revela los dominios.

### 2.4 Requerimientos No Funcionales

- **Seguridad**: el token de GitHub no debe quedar en capas de imagen, `go.sum` ni logs; usar BuildKit secrets. Los blobs cifrados no deben contener el dominio en claro.
- **Rendimiento**: el descifrado ocurre una vez por plugin al construir el registry (init); sin impacto medible en iHost.
- **Dependencias**: solo stdlib (`crypto/aes`, `crypto/cipher`, `encoding/base64`); cero deps nuevas.
- **Almacenamiento**: sin cambios en SQLite ni migraciones.
- **Disponibilidad**: el build requiere acceso a GitHub; el runtime no (todo compilado).

---

## 3. Investigación y Decisiones Técnicas

### 3.1 Contexto investigado

- Los plugins viven en `internal/plugins/{claro,tigo,disnorte,enacal,assa}` y el contrato en `internal/plugins/{types,registry}.go` (`Plugin` con `Name/Version/Source/Description/CredentialSchema/FetchBills`).
- `internal/` de un módulo Go **no puede importarse desde otro módulo**, por lo que el contrato debe vivir en el módulo privado y el repo público re-exportarlo con aliases (evita también un ciclo de módulos).
- El build de la imagen corre en GitHub Actions (`docker-publish.yml`, tags `v*`, multi-arch amd64/arm64/armv7) y ejecuta `go mod download` dentro del Dockerfile.
- `gh` está autenticado con scopes `repo` y `workflow`, suficiente para crear el repo privado y sus secrets.

### 3.2 Opciones evaluadas

| Opción | Pros | Contras | Decisión |
|--------|------|---------|----------|
| A. Inyección en build (`-ldflags`, env) | Rápida | Dominios en CI/secrets y en el binario; no oculta el know-how | ❌ Rechazada |
| B. Blob cifrado en repo público | Oculta de búsquedas casuales | La llave está en el repo público → no es secreto; el know-how sigue público | ❌ Rechazada |
| C. Módulo privado (esta spec) | Oculta código, endpoints y flujos; única opción con secreto real | Requiere token en build/CI y onboarding | ✅ Seleccionada |
| D. Config remota cifrada | Rota sin recompilar | Infra extra y bootstrap también visible | ❌ Rechazada (por ahora) |
| C+B. Módulo privado + ofuscación de dominios | Cubre además la imagen pública de Docker Hub | Complejidad menor | ✅ Seleccionada |

### 3.3 Decisiones arquitectónicas (ADRs)

**ADR-001**: El contrato de plugins vive en el módulo privado; el repo público solo re-exporta aliases.
- **Contexto**: `internal/` no es importable entre módulos y el contrato no es información sensible.
- **Decisión**: mover `types.go`/`registry.go` al paquete `plugins` del módulo privado; en el público `internal/plugins` define aliases y variables equivalentes.
- **Consecuencias**: una sola fuente de verdad; `services`/`api` no cambian; el repo público no compila sin el módulo privado (esperado).

**ADR-002**: Ofuscación de dominios dentro del módulo privado (AES-256-GCM, llave partida).
- **Contexto**: la imagen Docker es pública y `strings` recupera los literales del binario.
- **Decisión**: cifrar cada base URL; descifrar en runtime con `secret.Must`.
- **Consecuencias**: `strings` sobre la imagen no muestra dominios; un atacante con el binario podría extraer llave+ciphertext con ingeniería inversa (aceptado: eleva el costo, no es criptografía fuerte). El binario sigue conteniendo endpoints/paths de las rutas relativas.

**ADR-003**: Sin `go mod vendor` en el repo público.
- **Contexto**: `vendor/` haría que el código privado se copie al repo público.
- **Decisión**: prohibir vendor; el build usa la module cache.
- **Consecuencias**: el build necesita red+token; se documenta y se cachea `GOMODCACHE` en CI si hace falta.

**ADR-004**: Las specs `SPEC-003/007/009/012/014` quedan como están (públicas, con dominios y endpoints).
- **Contexto**: el usuario decidió no moverlas ni redactarlas (2026-09-15).
- **Decisión**: no tocarlas en esta spec.
- **Consecuencias**: los dominios siguen públicos en `docs/specs/`; el objetivo de ocultamiento queda parcial. Ver Riesgos y REQ-010/futura spec.

---

## 4. Diseño Técnico

### 4.1 Diagrama de arquitectura

```
[repo público p40la-ihost-automation]                [repo privado p40la-ihost-automation-plugins]
  internal/plugins/ (aliases)  ----require---->        plugins/   (contrato: Plugin, Bill, errors, Registry)
  cmd/server/main.go (RegisterAll)                     secret/    (AES-256-GCM + llave)
  Dockerfile (--mount=secret gh_token)                 claro/ tigo/ disnorte/ enacal/ assa/  (+ tests)
  workflow (secrets.PLUGINS_TOKEN)                     all/       (RegisterAll)
                                                       cmd/gensecret/
                                                       scripts/tigo-auth.sh
        |                                                        |
        +------------------ go build (módulo privado) ------------+
        v
  [binario server] -- dominios en ciphertext; se descifran en runtime
```

### 4.2 Componentes

#### 4.2.1 Módulo privado `p40la-ihost-automation-plugins`
- **Responsabilidad**: contrato + implementaciones + ofuscación de dominios.
- **Interfaz**: paquete `plugins` (contrato), paquete `all` (`RegisterAll(*plugins.Registry)`).
- **Dependencias**: stdlib; el contrato no depende de nadie.
- **Ubicación**: repo privado (nuevo).

#### 4.2.2 `secret` (módulo privado)
- **Responsabilidad**: cifrar/descifrar literales.
- **Interfaz**: `func Must(cipherB64 string) string`, `func Decrypt(cipherB64 string) (string, error)`, `func Encrypt(plain string) (string, error)`.
- **Diseño**: AES-256-GCM; nonce aleatorio prefijado al ciphertext; llave de 32 bytes partida en dos constantes y XOR-eada en runtime.
- **Ubicación**: `p40la-ihost-automation-plugins/secret/secret.go`.

#### 4.2.3 `cmd/gensecret` (módulo privado)
- **Responsabilidad**: cifrar un literal y emitir el blob Base64 para pegar en el plugin (solo desarrollo; no se compila en el server).
- **Interfaz**: `go run ./cmd/gensecret 'https://dominio...'`.

#### 4.2.4 `internal/plugins` (repo público, aliases)
- **Responsabilidad**: mantener los imports de `services`/`api` sin cambios.
- **Contenido**:
  ```go
  package plugins

  import pplugins "github.com/paulomcnally/p40la-ihost-automation-plugins/plugins"

  type Plugin = pplugins.Plugin
  type Bill = pplugins.Bill
  type CredentialField = pplugins.CredentialField
  type Registry = pplugins.Registry

  var (
      NewRegistry         = pplugins.NewRegistry
      ErrAuthFailed       = pplugins.ErrAuthFailed
      ErrAuthExpired      = pplugins.ErrAuthExpired
      ErrServiceNotFound  = pplugins.ErrServiceNotFound
      ErrAPISchemaChanged = pplugins.ErrAPISchemaChanged
      ErrUpstream         = pplugins.ErrUpstream
  )
  ```

#### 4.2.5 `cmd/server/main.go` (repo público)
- **Cambio**: reemplazar los 5 imports de implementaciones por `all "github.com/paulomcnally/p40la-ihost-automation-plugins/all"` y llamar `all.RegisterAll(registry)`.

#### 4.2.6 Build (Dockerfile + workflow)
- **Dockerfile** (stage backend):
  ```dockerfile
  ENV GOPRIVATE=github.com/paulomcnally/p40la-ihost-automation-plugins
  COPY go.mod go.sum ./
  RUN --mount=type=secret,id=gh_token \
      GIT_CONFIG_COUNT=1 \
      GIT_CONFIG_KEY_0=url.https://x-access-token:$(cat /run/secrets/gh_token)@github.com/.insteadOf \
      GIT_CONFIG_VALUE_0=https://github.com/ \
      go mod download
  ```
- **Workflow**: en el step de build agregar `secrets: gh_token=${{ secrets.PLUGINS_TOKEN }}`. El PAT requiere scope `repo` (lectura del repo privado).

### 4.3 Modelo de datos

Sin cambios. `plugins.source` y `bills.source` siguen persistiendo el dominio descifrado en runtime (ver REQ-010/P2).

### 4.4 APIs / Contratos

Sin cambios en la API HTTP. El contrato Go (`plugins.Plugin`) se mantiene idéntico (mismos métodos y firmas), solo cambia de módulo.

### 4.5 Dependencias

- **Internas**: `internal/plugins` (aliases), `cmd/server/main.go`, `Dockerfile`, `.github/workflows/docker-publish.yml`, `scripts/seed-apps.sh`, `scripts/tigo-auth.sh` (se va), `AGENTS.md`.
- **Externas**: repo privado `github.com/paulomcnally/p40la-ihost-automation-plugins`; secret `PLUGINS_TOKEN` en GitHub Actions; stdlib crypto.

---

## 5. Criterios de Aceptación

### 5.1 Funcionales

- [ ] CA-001: `go build ./...` y `go test ./...` pasan en el repo público con `GOPRIVATE` + credenciales de GitHub configuradas.
- [ ] CA-002: El repo privado compila y sus tests pasan (`go build ./...`, `go vet ./...`, `go test ./...`).
- [ ] CA-003: En el repo público, `rg -i '<dominio_proveedor>' --hidden -g '!.git' -g '!docs/specs/**'` no devuelve coincidencias (docs/specs queda excluido por ADR-004).
- [ ] CA-004: `strings server | rg -i '<dominio_proveedor>'` no devuelve coincidencias en el binario compilado.
- [ ] CA-005: El server arranca, `GET /api/plugins` devuelve `source` correcto (descifrado) y `bills:fetch` funciona end-to-end contra al menos un proveedor.
- [ ] CA-006: `docker buildx build` (amd64) con el secret `gh_token` construye la imagen; sin el secret falla con error claro.
- [ ] CA-007: `scripts/seed-apps.sh` no contiene dominios y exige `PLUGIN_SOURCE` explícito.
- [ ] CA-008: `scripts/tigo-auth.sh` ya no existe en el repo público.
- [ ] CA-009: `go.sum` referencia una versión semver del módulo privado (no pseudo-versión de rama).

### 5.2 No funcionales

- [ ] CA-NF-001: El token de GitHub no aparece en ninguna capa de la imagen ni en logs de CI (`docker history` limpio).
- [ ] CA-NF-002: Cero dependencias nuevas de runtime en el repo público; el binario mantiene tamaño similar (±5%).
- [ ] CA-NF-003: El descifrado de dominios no agrega latencia perceptible al arranque (< 10 ms).

### 5.3 Testing

- **Unit tests** (repo privado): `secret.Encrypt/Decrypt` (round-trip, nonce distinto, error con blob inválido), `Source()` devuelve el dominio correcto en cada plugin, tests existentes de plugins siguen verdes.
- **Integration tests** (repo público): `internal/services` y `internal/api` sin cambios de comportamiento (aliases).
- **E2E tests**: build Docker con secret + arranque + `bills:fetch` real (local).
- **Carga/Performance**: arranque del server en iHost sin regresión.

---

## 6. Plan de Implementación

### 6.1 Fases

| Fase | Descripción | Estimación | Dependencias |
|------|-------------|------------|--------------|
| 1 | Crear repo privado + mover contrato, implementaciones, tests y `tigo-auth.sh`; tag `v1.0.0` | 0.5 día | Acceso `gh` |
| 2 | Implementar `secret` + `cmd/gensecret` + ofuscar dominios + tests | 0.5 día | Fase 1 |
| 3 | Adaptar repo público: aliases, `main.go`, `go.mod`/`go.sum`, Dockerfile, workflow, scripts, docs | 0.5 día | Fase 2 |
| 4 | Verificación: build/test local, `strings` del binario, build Docker amd64 con secret, E2E `bills:fetch` | 0.5 día | Fase 3 |
| 5 | Release: tag público + imagen multi-arch en CI + documentación | 0.25 día | Fase 4 |
| 6 | (Opcional, destructivo) Purga de historial git del repo público (BFG/filter-repo) | 0.25 día | Aprobación explícita del usuario |

### 6.2 Milestones

1. **MVP**: módulo privado funcionando + repo público compilando local con token.
2. **V1.0**: imagen Docker publicada sin dominios legibles + E2E verificado.

---

## 7. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| Token de GitHub filtrado en capas de imagen o logs | Baja | Alto | BuildKit secret mount; nunca `ARG`/`ENV`; revisar `docker history` en verificación |
| Build roto para devs sin token/`GOPRIVATE` | Alta | Medio | Documentar en `AGENTS.md`/README; REQ-009 con mensaje claro |
| `go.sum` con pseudo-versión o cambios sin tag | Media | Medio | REQ-007: tags semver; renovar `go get` al subir versión |
| Historial git público conserva el código con dominios | Alta | Medio | Fase 6 opcional (BFG/filter-repo + force-push) con aprobación explícita; alternativa: aceptar |
| `docs/specs/SPEC-003/007/009/012/014` siguen públicas con dominios (ADR-004) | Alta | Medio | Documentado; futura spec de redacción si el usuario lo decide |
| Runtime expone `source` en `/api/plugins` y SQLite | Media | Bajo | REQ-010 (P2, spec aparte) |
| Ofuscación reversible (llave+ciphertext en el binario) | Media | Medio | Aceptado: eleva el costo; no usar como único control. Endpoints/paths relativos siguen visibles |
| Pérdida del repo privado | Baja | Alto | Mantener remoto/backup; el repo público no es autosuficiente a propósito |
| `go mod vendor` accidental reintroduce código privado | Baja | Alto | ADR-003 + CA-003; `.gitignore`/revisión en CI |

---

## 8. Notas y Referencias

- Go private modules: `GOPRIVATE`, `GONOPROXY`/`GONOSUMDB` (https://go.dev/ref/mod#private-modules).
- BuildKit secrets en Dockerfile (`RUN --mount=type=secret`).
- `docker/build-push-action` y `secrets:` (https://github.com/docker/build-push-action).
- SPEC-003 (claro), SPEC-007 (tigo), SPEC-009 (disnorte), SPEC-012 (enacal), SPEC-014 (assa).
- Archivos afectados en repo público: `internal/plugins/**`, `cmd/server/main.go`, `go.mod`, `go.sum`, `Dockerfile`, `.github/workflows/docker-publish.yml`, `scripts/seed-apps.sh`, `scripts/tigo-auth.sh`, `AGENTS.md`.

---

## 9. Historial de Cambios

| Fecha | Autor | Descripción |
|-------|-------|-------------|
| 2026-09-15 | paulomcnally | Creación inicial de la especificación. Opción C (módulo privado) + ofuscación de dominios (B) dentro del módulo privado. Repo privado `p40la-ihost-automation-plugins`. Specs de plugins quedan públicas por decisión del usuario. |
| 2026-09-15 | paulomcnally | Aprobada por el usuario; estado → in_progress. Comienza implementación. |
| 2026-09-15 | paulomcnally | Implementación completa y verificada: repo privado `p40la-ihost-automation-plugins` v1.0.0 (contrato + 5 plugins + `secret` AES-256-GCM + `cmd/gensecret` + CI), aliases en `internal/plugins`, `main.go` con `pluginall.RegisterAll`, Dockerfile con BuildKit secret + `git` en builder, workflow con `secrets.PLUGINS_TOKEN`, compose con `.gh_token`, `seed-apps.sh` sin dominios, `tigo-auth.sh` movido al privado, docs (AGENTS/README) con setup local. Verificado: `go build`/`vet`/`test` OK, `strings` del binario sin dominios, imagen Docker construida con secret. Secret `PLUGINS_TOKEN` creado por el usuario. Estado → released. |
