---
title: "Título de la especificación"
id: "SPEC-XXX"
status: "draft"
author: ""
created: "YYYY-MM-DD"
updated: "YYYY-MM-DD"
github_issue: null
---

# {{title}}

**ID**: {{id}}  
**Estado**: {{status}}  
**Autor**: {{author}}  
**Creado**: {{created}}  
**Actualizado**: {{updated}}

---

## 1. Resumen Ejecutivo

Describir en 2-3 párrafos:
- Qué problema se resuelve
- Por qué es importante ahora
- Qué resultado se espera

Consideraciones específicas de iHost: impacto en memoria, almacenamiento, SQLite, etc.

Consideraciones de UI obligatorias para specs que toquen formularios:
- Todo `input`/`select`/`textarea` DEBE usar los tokens de color del tema (`bg-card`, `text-text`, `text-text-secondary`). Jamás colores hardcodeados (ej: `bg-white`, `dark:bg-[#2c2c2e]`).
- Todo formulario nuevo debe verificarse en darkmode antes de considerarse completo: texto y placeholders legibles.

Consideraciones de UI obligatorias para specs que creen páginas de detalle o formularios con lista padre:
- La acción "volver a la lista" va en el header (flecha atrás que reemplaza la hamburguesa en móvil) vía el mapa `BACK_ROUTES` de `frontend/src/components/DashboardLayout.tsx`. Nunca crear links "← Título" dentro del contenido. Ver SPEC-063.

## 2. Requerimientos

### 2.1 Requerimientos Funcionales (P0 - Obligatorios)

1. **REQ-001**: Descripción del requerimiento obligatorio
2. **REQ-002**: Descripción del requerimiento obligatorio

### 2.2 Requerimientos Funcionales (P1 - Importantes)

1. **REQ-003**: Descripción del requerimiento importante

### 2.3 Requerimientos Funcionales (P2 - Deseables)

1. **REQ-004**: Descripción del requerimiento deseable

### 2.4 Requerimientos No Funcionales

- **Rendimiento**: Tiempo de respuesta esperado, uso máximo de memoria
- **Seguridad**: Autenticación, autorización, datos sensibles
- **Almacenamiento**: Tamaño máximo en disco, rotación de logs
- **Disponibilidad**: Uptime esperado, health checks
- **iHost**: Consumo de RAM, CPU, dependencias minimizadas

## 3. Investigación y Decisiones Técnicas

### 3.1 Contexto investigado

Documentar qué se investigó:
- Fuentes consultadas (links, documentación)
- Tecnologías evaluadas
- Patrones considerados

### 3.2 Opciones evaluadas

| Opción | Pros | Contras | Decisión |
|--------|------|---------|----------|
| Opción A | ... | ... | ✅ Seleccionada |
| Opción B | ... | ... | ❌ Rechazada |

### 3.3 Decisiones arquitectónicas (ADRs)

**ADR-001**: [Título de la decisión]
- **Contexto**: Por qué se tomó esta decisión
- **Decisión**: Qué se decidió
- **Consecuencias**: Impactos positivos y negativos

## 4. Diseño Técnico

### 4.1 Diagrama de arquitectura

```
[Componente A] --(API REST)--> [Componente B]
     |                              |
     v                              v
[SQLite]                    [Frontend estático]
```

### 4.2 Componentes

#### 4.2.1 [Nombre del componente]
- **Responsabilidad**: Qué hace
- **Interfaz**: API expuesta, contratos
- **Dependencias**: Qué necesita
- **Ubicación**: Ruta en el repo

### 4.3 Modelo de datos

```
Entidad: [Nombre]
- campo_1: tipo (descripción)
- campo_2: tipo (descripción)
- Relaciones: [Entidad] (1:N)
```

### 4.4 APIs / Contratos

#### Endpoint: `METHOD /ruta`

**Request**:
```json
{
  "campo": "tipo"
}
```

**Response 200**:
```json
{
  "campo": "tipo"
}
```

**Response Error**:
```json
{
  "error": "código",
  "message": "descripción"
}
```

### 4.5 Dependencias

- **Internas**: Servicios/componentes del sistema que se modifican
- **Externas**: Librerías, APIs de terceros, servicios externos

## 5. Criterios de Aceptación

### 5.1 Funcionales

- [ ] CA-001: Dado [contexto], cuando [acción], entonces [resultado esperado]
- [ ] CA-002: Dado [contexto], cuando [acción], entonces [resultado esperado]
- [ ] CA-DARK: Si la spec toca formularios/modales, los inputs usan tokens del tema (`bg-card`, `text-text`) y se verificó legibilidad en darkmode (texto y placeholders).
- [ ] CA-BACK: Si la spec crea una página de detalle o formulario con lista padre, el retorno se hace con la flecha atrás del header (registrada en `BACK_ROUTES`) y NO existen links "← Título" dentro del contenido.

### 5.2 No funcionales

- [ ] CA-NF-001: [Criterio de rendimiento/seguridad/etc.]

### 5.3 Testing

- **Unit tests**: Qué lógica debe cubrirse
- **Integration tests**: Qué flujos integrados probar
- **E2E tests**: Qué escenarios de usuario probar
- **Carga/Performance**: Qué métricas validar en iHost

## 6. Plan de Implementación

### 6.1 Fases

| Fase | Descripción | Estimación | Dependencias |
|------|-------------|------------|--------------|
| 1 | [Tarea] | X días | Ninguna |
| 2 | [Tarea] | Y días | Fase 1 |

### 6.2 Milestones

1. **MVP**: [Qué incluye el mínimo viable]
2. **V1.0**: [Versión completa]

## 7. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| [Descripción] | Alta/Media/Baja | Alto/Medio/Bajo | [Estrategia] |

## 8. Notas y Referencias

- Links útiles
- Documentación relacionada
- Tickets/issues relacionados

## 9. Historial de Cambios

| Fecha | Autor | Descripción |
|-------|-------|-------------|
| YYYY-MM-DD | [Nombre] | Creación inicial de la especificación |
