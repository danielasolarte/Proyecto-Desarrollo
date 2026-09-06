# Módulo: Cursos, Autoría y Catálogo (Persona 1 / Persona B)

Este documento describe lo que implementa este módulo dentro del monolito,
para que el resto del equipo (y quien evalúe la entrega) entienda qué
cubre y qué asume de otros módulos.

## Qué cubre

- Migraciones de `courses`, `course_versions`, `modules`, `units`,
  `resources`, `enrollments` (`migrations/000001_create_courses_catalog_tables.*`).
- CRUD completo de la jerarquía Curso → Módulo → Unidad → Recurso.
- Metadatos de curso (título, resumen, categoría) y ordenamiento por
  posición en cada nivel.
- Estados de versión: `draft`, `published`, `superseded`. Las versiones
  publicadas son inmutables: cualquier intento de mutar su estructura
  responde `423 Locked`.
- **Opcional implementado**: borradores de actualización con
  clasificación de cambios (`POST /courses/{id}/draft` con
  `change_type: minor|major`). Permite editar un curso publicado sin
  despublicarlo: crea una nueva versión draft copiando módulos, unidades
  y recursos, preservando el `stable_id` de cada uno para que el módulo
  de progreso no pierda el avance de los estudiantes cuando se publique
  la actualización.
- Previsualización (`GET /courses/{id}/preview`): devuelve el árbol
  completo de la versión en edición más la lista exhaustiva de problemas
  que faltan para poder publicar (no solo el primero).
- Editor de bloques: autosave (`PUT /resources/{id}/content/draft`),
  recuperación (`GET .../content/draft`) y "publicar dentro del
  borrador" (`POST .../content/publish`) para recursos de tipo
  `rich_text`, en Markdown extendido.
- Catálogo (`GET /catalog`): búsqueda y filtros sobre cursos publicados
  únicamente, con paginación por cursor.
- Inscripción, retiro y reinscripción (`POST`/`DELETE
  /courses/{id}/enrollments`, `GET /me/enrollments`): la reinscripción
  reactiva la misma fila en vez de crear una nueva, para no perder el
  historial.
- OpenAPI de todos estos endpoints en `api/openapi.yaml`.

## Qué NO cubre (a propósito, es de otros módulos)

- **Autenticación real**: este módulo depende de
  `internal/platform/authctx`, un shim temporal que lee el usuario de
  los headers `X-User-Id` / `X-User-Role`. Cuando el módulo de identidad
  (Persona A, `internal/auth`) esté listo, hay que reemplazar
  `authctx.FakeAuthMiddleware()` en `cmd/api/main.go` por el middleware
  real de sesiones. Los handlers de este módulo no deberían necesitar
  ningún cambio, porque solo dependen de `authctx.UserFromContext`.
- **Procesamiento multimedia**: los recursos de tipo `image`, `video`,
  `audio`, `pdf`, `presentation` y `file` se crean con
  `processing_status = pending` y quedan a la espera de que el módulo de
  multimedia (Persona C) los actualice a `ready` (o `failed`). La
  validación de publicación (`ValidatePublication` en
  `internal/courses/usecase/publish.go`) ya exige que todo recurso
  visible de tipo media esté en `ready` antes de poder publicar.
- **Progreso y aprobación**: la tabla `resources.required` es la señal
  que el módulo de progreso (Persona D) necesita para saber qué recursos
  cuentan para completar un curso; ese cálculo vive en su módulo, no en
  este.
- **Contenido de quizzes**: el recurso de tipo `quiz` solo se referencia
  aquí como una entrada más de la unidad (título, posición, visibilidad);
  las preguntas, intentos y calificación viven en `internal/quiz`.

## Cómo correrlo localmente

```
docker compose up -d postgres redis minio mailpit clamav
migrate -database "postgres://mooc:mooc@localhost:5432/mooc?sslmode=disable" -path migrations up
go mod tidy
go run ./cmd/api
```

Como todavía no existe el módulo de auth, para probar cualquier endpoint
de autoría hay que mandar estos headers a mano (simulan un profesor ya
autenticado):

```
curl -X POST localhost:8080/api/v1/courses \
  -H "Content-Type: application/json" \
  -H "X-User-Id: 11111111-1111-1111-1111-111111111111" \
  -H "X-User-Role: teacher" \
  -d '{"title":"Introduccion a Go","summary":"Curso basico","category":"programacion"}'
```

Y para probar catálogo/inscripción como estudiante:

```
curl localhost:8080/api/v1/catalog

curl -X POST localhost:8080/api/v1/courses/<courseID>/enrollments \
  -H "X-User-Id: 22222222-2222-2222-2222-222222222222" \
  -H "X-User-Role: student"
```

## Nota sobre el número de migración

Este módulo usa `000001_create_courses_catalog_tables`. Como
`golang-migrate` numera las migraciones para todo el repositorio (no por
módulo), en cuanto se junte con las migraciones de los otros 3 módulos
va a ser necesario renumerar para que no se pisen los números de
secuencia. Avisar en el grupo antes de hacer ese merge.
