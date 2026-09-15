---
name: spec-manager
description: "Use ONLY when the user asks to create, manage, update, or track technical specifications (specs). Triggers on keywords: spec, especificacion, especificación, requirement, requerimiento, tracking, contador, estado de spec, pending release. Manages the full spec lifecycle from creation to release for the p40la-ihost-automation project."
---

# Spec Manager

Sistema de gestión de especificaciones técnicas para el repositorio `p40la-ihost-automation`.

## Ubicación

- **Specs activas**: `docs/specs/`
- **Template**: `docs/specs/templates/spec-template.md`
- **Tracking**: `docs/specs/README.md`
- **GitHub Issues**: Espejo sincronizado con labels de estado

## Estados de una Spec

```
draft → pending_execution → in_progress → pending_release → released
```

| Estado | Descripción | Color | Label GitHub | Acción Issue |
|--------|-------------|-------|-------------|--------------|
| `draft` | En proceso de redacción/investigación | 🟡 | `spec/draft` | Abierto |
| `pending_execution` | Lista para desarrollo, no iniciada | 🔵 | `spec/pending-execution` | Abierto |
| `in_progress` | Actualmente en desarrollo | 🟣 | `spec/in-progress` | Abierto |
| `pending_release` | Desarrollo completo, lista para release | 🟠 | `spec/pending-release` | Abierto |
| `released` | Subida a iHost o producción | 🟢 | `spec/released` | **Cerrado** |
| `cancelled` | Cancelada o descartada | ⚫ | `spec/cancelled` | **Cerrado** |

## Regla de Oro: Flujo Forzado

**El agente DEBE seguir estas reglas sin excepción:**

1. **NUNCA escribir código significativo sin una spec existente.** Si el usuario pide una funcionalidad, primero crear la spec.
2. **Si el usuario pide cambios iterativos durante desarrollo que no están en la spec**, el agente debe:
   - Actualizar la spec con los nuevos requerimientos/cambios
   - Agregar un comentario al issue de GitHub documentando el cambio
   - Solo entonces implementar el cambio
3. **Si el usuario pide cambios pequeños y el agente los implementa sin actualizar la spec**, debe al menos agregar un comentario al issue: `"[AUTO] Cambio iterativo solicitado por usuario: <descripción>"`.
4. **Antes de cualquier operación de cambio de estado**, intentar actualizar el issue de GitHub. Si `gh` CLI falla, continuar con warning visible.

## Integración con GitHub Issues

Cada spec tiene un issue de GitHub asociado almacenado en el frontmatter como `github_issue: <número>`.

### Labels de GitHub

| Label | Color | Descripción |
|-------|-------|-------------|
| `spec/draft` | `#F4D03F` | Spec en redacción |
| `spec/pending-execution` | `#3498DB` | Lista para desarrollo |
| `spec/in-progress` | `#9B59B6` | En desarrollo activo |
| `spec/pending-release` | `#E67E22` | Desarrollo completo, pendiente release |
| `spec/released` | `#2ECC71` | En producción/iHost |
| `spec/cancelled` | `#95A5A6` | Cancelada o descartada |

### Operaciones de GitHub

**Crear labels** (una sola vez, al inicializar el repo):
```bash
for label in draft pending-execution in-progress pending-release released cancelled; do
  gh label create "spec/$label" --color "$(color)" --force 2>/dev/null || true
done
```

**Crear issue** (al crear spec):
```bash
gh issue create --title "SPEC-XXX: Título" --label "spec/<estado>" --body "<cuerpo>"
```

**Actualizar labels** (al cambiar estado):
```bash
gh issue edit <número> --remove-label "spec/<anterior>" --add-label "spec/<nuevo>"
```

**Cerrar issue** (al release/cancel):
```bash
gh issue close <número>
```

**Reabrir issue** (si se reabre spec):
```bash
gh issue reopen <número>
```

**Listar issues**:
```bash
gh issue list --label "spec/<estado>" --state open
```

**Comentar en issue** (cambios iterativos):
```bash
gh issue comment <número> --body "<comentario>"
```

## Flujo de trabajo

### 1. Crear una nueva spec

Cuando el usuario pide crear una spec:

1. **Investigar** el requerimiento usando websearch/webfetch si es necesario
2. **Generar ID**: Buscar el último número en `docs/specs/README.md` y asignar `SPEC-XXX` (3 dígitos, zero-padded)
3. **Crear archivo**: `docs/specs/SPEC-XXX-<titulo-breve>.md` con campo `github_issue: null`
4. **Documentar completamente** usando el template como guía
5. **Crear issue de GitHub**:
   ```bash
   OUTPUT=$(gh issue create --title "SPEC-XXX: Título" --label "spec/draft" --body "<cuerpo resumido>" 2>&1)
   ISSUE_NUM=$(echo "$OUTPUT" | grep -oP '\d+$')
   ```
6. **Actualizar frontmatter**: Cambiar `github_issue: null` a `github_issue: <número>`
7. **Confirmar**: Informar al usuario el ID asignado y URL del issue

### 2. Contador de specs

El contador está en `docs/specs/README.md` en la sección "## Resumen".

- Total de specs: conteo de todas las specs en el directorio
- Por estado: conteo agrupado por estado
- Último ID usado: último número secuencial

### 3. Actualizar estado de una spec

Cuando el usuario menciona que una spec cambió de estado:

1. **Validar transición**: Asegurar que el estado anterior y nuevo son válidos según el diagrama de estados
2. **Leer spec**: Obtener `github_issue` del frontmatter
3. **Actualizar GitHub** (si `github_issue` existe y `gh` está disponible):
   - Determinar label nuevo según mapeo
   - `gh issue edit <número> --remove-label "spec/<anterior>" --add-label "spec/<nuevo>"`
   - Si nuevo estado es `released` o `cancelled`: `gh issue close <número>`
   - Si falla GitHub: continuar con warning `"⚠️ No se pudo actualizar el issue de GitHub"`
4. **Actualizar archivo local**: Cambiar `status:` en el frontmatter
5. **Actualizar historial**: Agregar entrada con fecha y descripción del cambio
6. **Confirmar**: Informar al usuario

### 4. Reglas de documentación

Cada archivo de spec DEBE incluir:

- **Frontmatter** con: title, id, status, author, created, updated, github_issue
- **Resumen ejecutivo**: Qué problema resuelve (2-3 párrafos)
- **Requerimientos**: Lista numerada y priorizada (P0, P1, P2)
- **Investigación**: Referencias, decisiones técnicas evaluadas, porqué de cada decisión
- **Diseño técnico**: Arquitectura, componentes, APIs, dependencias
- **Criterios de aceptación**: Lista verificable de condiciones que definen "terminado"
- **Plan de implementación**: Fases, estimación, dependencias entre tareas
- **Riesgos y mitigaciones**: Qué puede fallar y cómo manejarlo
- **Historial de cambios**: Fecha, autor, descripción de cada modificación

### 5. Release

Cuando una spec pasa a `released`:

1. Verificar que exista commit/versión que la suba a iHost
2. Documentar en la spec: commit/versión, fecha de deploy
3. Cerrar el issue de GitHub con label `spec/released`
4. El estado `released` significa "en iHost o producción"

**Regla CRÍTICA: al pasar a `released`, el agente DEBE hacer commit.**

- El commit de release es obligatorio y se hace SIEMPRE que una spec pasa a `released` (no esperar a que el usuario lo pida).
- El commit de release incluye: código de implementación + spec + tracker README. Mensaje sugerido: `SPEC-XXX: release (código + docs + tracker: estado released)`.
- Verificar antes: `git status` para stagear solo los archivos de la spec (nunca `data/` ni `public/`), y `git log --oneline -3` para seguir el estilo del repo.
- Si el usuario pide release sin implementación (cierre por decisión), el commit incluye docs + tracker con el estado released.

**Regla CRÍTICA: implementación y release son inseparables.**

- Una spec NO se da por terminada hasta completar el release completo: frontmatter + cuerpo + README tracker + label GitHub + cierre del issue + documentación del commit.
- Si el código de una feature está commiteado en `main`, la spec DEBE estar `released`. Toda spec en `pending_release` con código en `main` es un error.
- Antes de declarar terminada una spec, ejecutar `/spec check-release` para confirmar que no quedan colgadas.
- Los commits de implementación DEBEN ir acompañados (en la misma sesión) del commit de release que documenta el cambio de estado. No dejar la spec en `pending_release` tras mergear a main.

## Comandos de la skill

### `/spec create "<título>"`
Crear nueva spec con título dado. Investigación automática. Crea archivo local + issue de GitHub.

### `/spec list [estado]`
Listar specs locales. Filtrar por estado si se proporciona.

### `/spec status <id> <nuevo_estado>`
Cambiar estado de una spec. Validar transición permitida. Actualiza archivo local + issue de GitHub.

### `/spec show <id>`
Mostrar resumen de una spec específica.

### `/spec list-issues [estado]`
Listar issues de GitHub de specs. Filtrar por estado/label si se proporciona. Muestra ID, título, estado, URL.
```bash
gh issue list --label "spec/<estado>" --json number,title,state,url
```

### `/spec sync`
Sincronizar estado de issues de GitHub con archivos locales.
1. Listar todos los issues con label que empiece con `spec/`
2. Para cada issue, buscar la spec local con `github_issue` matching
3. Comparar estado del issue (labels + open/closed) con estado de la spec
4. Si hay diferencia, actualizar archivo local + README.md
5. Reporte final: specs sincronizadas, sin cambios, warnings

### `/spec process <ID>`
Tomar una spec en `pending_execution` y empezar desarrollo.
1. Validar que spec esté en `pending_execution`
2. Cambiar estado a `in_progress` (actualizar archivo, label, README)
3. Leer criterios de aceptación de la spec
4. Generar lista de tareas (todo) para el agente
5. Confirmar y mostrar plan

### `/spec pending`
Mostrar rápidamente specs en estados no-terminal con sus issues.
```bash
gh issue list --label "spec/draft,spec/pending-execution,spec/in-progress,spec/pending-release" --state open
```

### `/spec check-release`
Verificar que no haya specs "colgadas" (código commiteado en main pero estado != `released`). Debe ejecutarse SIEMPRE antes de declarar terminada una spec y tras cada release.

1. Listar todas las specs locales y su estado (`frontmatter status`)
2. Para cada spec en estado != `released`, verificar si su feature tiene código en `main`:
   - Buscar commits con el nombre de la spec: `git log --all --oneline --grep="SPEC-XXX" -i`
   - Si hay commits de implementación pero el estado no es `released`, la spec está colgada
3. Cruzar con issues de GitHub: `gh issue list --label "spec/<estado>" --state open`
4. Reporte final:
   - Specs colgadas (requieren release o documentación de bloqueo)
   - Issues abiertos sin spec local o viceversa
   - Consistencia local vs GitHub (frontmatter vs labels + open/closed)
5. Si hay colgadas: NO declarar la tarea terminada. Ejecutar el flujo de release de la sección 5 o documentar el bloqueo.

## Validaciones

- Un ID no se puede reutilizar
- `draft` puede ir a `pending_execution` o `cancelled`
- `pending_execution` puede ir a `in_progress` o `cancelled`
- `in_progress` puede ir a `pending_release` o volver a `pending_execution`
- `pending_release` puede ir a `released` o volver a `in_progress`
- `released` es estado final (o puede ir a `in_progress` para hotfixes)
- `cancelled` es estado terminal

## Formato de IDs

- Siempre `SPEC-XXX` donde XXX es 001-999
- Los números son secuenciales y no se reutilizan
- Extraer número del nombre de archivo: `SPEC-(\d{3})`

## Cuerpo del issue de GitHub

Al crear un issue, usar este formato:

```markdown
## SPEC-XXX: Título Completo

**Estado**: draft | pending_execution | in_progress | pending_release | released
**Archivo**: [docs/specs/SPEC-XXX-titulo.md](https://github.com/paulomcnally/p40la-ihost-automation/blob/main/docs/specs/SPEC-XXX-titulo.md)

---

### Resumen
[2-3 párrafos del resumen ejecutivo]

### Requerimientos P0
1. REQ-001: ...
2. REQ-002: ...

### Criterios de Aceptación
- [ ] CA-001: ...
- [ ] CA-002: ...

### Plan de Implementación
| Fase | Descripción | Estimación |
|------|-------------|------------|
| 1 | ... | ... |

---
*Este issue es gestionado automáticamente por el sistema de specs. No cambiar labels manualmente.*
```