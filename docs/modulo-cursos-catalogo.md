# Modulo: Cursos, Autoria y Catalogo (Persona 1)

Este documento describe el modulo de cursos dentro del monolito. Cubre la
autoria de cursos, la publicacion versionada, el editor Markdown y el catalogo
publico con inscripciones.

## Que cubre

- Migraciones de `courses`, `course_versions`, `modules`, `units`,
  `resources` y `enrollments`.
- CRUD completo de la jerarquia `Curso -> Modulo -> Unidad -> Recurso`.
- Metadatos de curso: titulo, resumen, categoria y slug.
- Ordenamiento por posicion en modulos, unidades y recursos.
- Estados de version: `draft`, `published` y `superseded`.
- Versiones publicadas inmutables: cualquier intento de mutar su estructura
  responde como recurso bloqueado.
- Borradores de actualizacion con `change_type: minor|major`, preservando
  `stable_id` para continuidad del progreso.
- Preview de cursos con arbol completo y lista exhaustiva de problemas de
  publicacion.
- Editor Markdown para recursos `rich_text`.
- Autosave, recuperacion y publicacion del contenido Markdown dentro del
  borrador.
- Catalogo publico de cursos publicados con busqueda, filtros y paginacion por
  cursor.
- Inscripcion, retiro y reinscripcion conservando la misma fila de historial.
- OpenAPI de endpoints en `api/openapi.yaml`.

## Integracion con Auth

Este modulo ya se integra con el middleware real de sesiones de
`internal/auth`. Los handlers de cursos/catalogo dependen de
`authctx.UserFromContext`, por lo que aceptan el usuario autenticado que llega
desde:

- `Authorization: Bearer <token>`
- `X-Session-Token: <token>`

Para crear o editar cursos se requiere rol `teacher` o `admin`. Para
inscripciones se requiere rol `student` o `admin`. El catalogo publico no exige
autenticacion.

## Que no cubre

- Procesamiento multimedia: los recursos `image`, `video`, `audio`, `pdf`,
  `presentation` y `file` quedan con `processing_status = pending` y deben ser
  actualizados por el modulo multimedia.
- Progreso y aprobacion: este modulo marca recursos `required`, pero el calculo
  de avance pertenece al modulo de progreso.
- Contenido de quizzes: el recurso `quiz` existe como entrada de unidad, pero
  preguntas, intentos y calificacion pertenecen al modulo de quizzes.

## Como correrlo localmente

```bash
docker compose up -d postgres redis minio mailpit clamav
migrate -database "postgres://mooc:mooc@localhost:5432/mooc?sslmode=disable" -path migrations up
go mod tidy
go run ./cmd/api
```

Crear un curso como profesor:

```bash
curl -X POST localhost:8080/api/v1/courses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN_PROFESOR" \
  -d '{"title":"Introduccion a Go","summary":"Curso basico","category":"programacion"}'
```

Consultar catalogo:

```bash
curl localhost:8080/api/v1/catalog
```

Inscribirse como estudiante:

```bash
curl -X POST localhost:8080/api/v1/courses/COURSE_ID/enrollments \
  -H "Authorization: Bearer TOKEN_ESTUDIANTE"
```
