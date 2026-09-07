# Plataforma MOOC - Backend

Backend para una plataforma web de cursos masivos abiertos en linea, construido
como monolito modular en Go. El sistema expone una API REST bajo `/api/v1`,
usa PostgreSQL como fuente transaccional, Redis para sesiones/cache/colas,
MinIO para almacenamiento de objetos, Mailpit para correos locales y ClamAV
como servicio base para escaneo antimalware.

Este repositorio corresponde a la Entrega 1 del proyecto. El alcance actual es
backend: se valida con curl/Postman, pruebas automaticas y la especificacion
OpenAPI.

## Estado actual

Implementado:

- Persona 1: cursos, autoria y catalogo.
- Persona 2: identidad, sesiones y administracion.
- Infraestructura local base con Docker Compose: PostgreSQL, Redis, MinIO,
  Mailpit y ClamAV.
- API base con Echo.
- Worker base con asynq.
- Migraciones SQL con `golang-migrate`.
- OpenAPI parcial en `api/openapi.yaml`.

Pendiente para completar el alcance total del proyecto:

- Persona 3: multimedia, URLs prefirmadas, carga multipart, ClamAV real,
  procesamiento HLS con ffmpeg, DLQ y endpoints PDF/HLS.
- Persona 4: quizzes, intentos, progreso, insignias y prueba de carga con k6.
- Dockerfiles y servicios `api`/`worker` dentro de `docker-compose.yml`.
- CI, metricas, trazas y pruebas E2E completas.

## Modulos

### Persona 1 - Cursos, Autoria y Catalogo

Documentacion: `docs/modulo-cursos-catalogo.md`

Cubre:

- Migraciones de `courses`, `course_versions`, `modules`, `units`,
  `resources` y `enrollments`.
- CRUD de la jerarquia `Curso -> Modulo -> Unidad -> Recurso`.
- Metadatos, ordenamiento y estados `draft`, `published`, `superseded`.
- Versiones publicadas inmutables.
- Preview de cursos con lista de problemas de publicacion.
- Editor Markdown para recursos `rich_text`.
- Autosave, recuperacion y publicacion de contenido dentro del borrador.
- Catalogo de cursos publicados con busqueda, filtros y cursor.
- Inscripcion, retiro y reinscripcion conservando historial.

### Persona 2 - Identidad y Administracion

Documentacion: `docs/modulo-auth-admin.md`

Cubre:

- Migracion `000002_create_auth_admin_tables` con `users`, `sessions` y
  `audit_log`.
- Registro publico de estudiantes.
- Verificacion de correo con Mailpit.
- Login con bcrypt y sesiones opacas revocables.
- Sesiones persistidas en PostgreSQL y cacheadas en Redis.
- Middleware real de autenticacion y roles.
- Recuperacion de contrasena por token.
- Bootstrap del primer administrador.
- Creacion de profesores desde administracion.
- CRUD administrativo basico de usuarios, roles y estados.
- Revocacion de sesiones.
- Auditoria basica.
- Proteccion del ultimo administrador activo.
- Pruebas de casos de uso de Auth/Admin.

## Estructura

```text
cmd/
  api/                 main del servidor HTTP
  worker/              main del worker asynq
internal/
  admin/               endpoints administrativos
  auth/                identidad, sesiones y middleware
  catalog/             catalogo e inscripciones
  courses/             cursos, versionado, autoria y editor
  platform/            utilidades compartidas
migrations/            migraciones SQL de golang-migrate
api/                   OpenAPI 3.1
docs/                  documentacion por modulo
docker-compose.yml     infraestructura local
```

Cada modulo sigue el patron:

```text
domain/    entidades, errores e interfaces
usecase/   reglas de negocio
postgres/  implementacion SQL
http/      handlers y rutas Echo
```

## Requisitos

- Go instalado. En esta maquina se uso `go1.26.6`; el proyecto no usa codigo
  especifico de esa version, pero `go mod tidy` dejo `go 1.26.0` en `go.mod`.
- Docker y Docker Compose.
- `golang-migrate` para aplicar migraciones.

Instalar `golang-migrate` si no esta disponible:

```bash
go install -tags "postgres" github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

## Configuracion

Variables de entorno soportadas por la API:

```bash
DATABASE_URL=postgres://mooc:mooc@localhost:5432/mooc?sslmode=disable
REDIS_ADDR=localhost:6379
SMTP_ADDR=localhost:1025
MAIL_FROM=no-reply@mooc.local
PORT=8080
```

Si no se definen, la API usa esos valores por defecto.

## Levantar en local

1. Levantar infraestructura:

```bash
docker compose up -d postgres redis minio mailpit clamav
```

2. Verificar contenedores:

```bash
docker compose ps
```

3. Descargar dependencias:

```bash
go mod tidy
```

4. Aplicar migraciones:

```bash
migrate -database "postgres://mooc:mooc@localhost:5432/mooc?sslmode=disable" -path migrations up
```

5. Correr API:

```bash
go run ./cmd/api
```

6. Correr worker en otra terminal:

```bash
go run ./cmd/worker
```

7. Probar healthcheck:

```bash
curl localhost:8080/healthz
```

Respuesta esperada:

```json
{"status":"ok"}
```

## Flujo rapido de Auth/Admin

Crear primer administrador:

```bash
curl -X POST localhost:8080/api/v1/auth/bootstrap-admin \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"admin@example.com\",\"password\":\"claveSegura123\",\"full_name\":\"Admin MOOC\"}"
```

Login del admin:

```bash
curl -X POST localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"admin@example.com\",\"password\":\"claveSegura123\"}"
```

Usar el campo `token` de la respuesta para endpoints protegidos:

```bash
curl localhost:8080/api/v1/admin/users \
  -H "Authorization: Bearer TOKEN_ADMIN"
```

Crear profesor desde administracion:

```bash
curl -X POST localhost:8080/api/v1/admin/users/teachers \
  -H "Authorization: Bearer TOKEN_ADMIN" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"profe@example.com\",\"password\":\"claveSegura123\",\"full_name\":\"Profe Uno\"}"
```

Registrar estudiante:

```bash
curl -X POST localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"ana@example.com\",\"password\":\"claveSegura123\",\"full_name\":\"Ana Perez\"}"
```

El token de verificacion llega a Mailpit en:

```text
http://localhost:8025
```

Para facilitar pruebas locales, el endpoint tambien devuelve
`verification_token_dev`.

Verificar estudiante:

```bash
curl -X POST localhost:8080/api/v1/auth/verify-email \
  -H "Content-Type: application/json" \
  -d "{\"token\":\"TOKEN_DE_VERIFICACION\"}"
```

Login de estudiante:

```bash
curl -X POST localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"ana@example.com\",\"password\":\"claveSegura123\"}"
```

## Flujo rapido de Cursos/Catalogo

Crear curso como profesor:

```bash
curl -X POST localhost:8080/api/v1/courses \
  -H "Authorization: Bearer TOKEN_PROFESOR" \
  -H "Content-Type: application/json" \
  -d "{\"title\":\"Introduccion a Go\",\"summary\":\"Curso basico\",\"category\":\"programacion\"}"
```

Consultar catalogo publico:

```bash
curl "localhost:8080/api/v1/catalog?q=go&category=programacion"
```

Inscribirse como estudiante:

```bash
curl -X POST localhost:8080/api/v1/courses/COURSE_ID/enrollments \
  -H "Authorization: Bearer TOKEN_ESTUDIANTE"
```

## Pruebas

Ejecutar toda la suite:

```bash
go test ./...
```

Actualmente hay pruebas automaticas para casos de uso de Auth/Admin:

- registro de estudiante y token de verificacion;
- verificacion de correo y login;
- creacion del primer admin;
- proteccion del ultimo admin activo.

## OpenAPI

La especificacion esta en:

```text
api/openapi.yaml
```

Incluye endpoints de:

- identidad;
- administracion;
- cursos/autoria;
- editor Markdown;
- catalogo e inscripciones.

## Notas de entrega

- El backend ya no depende de headers falsos para autenticar: el middleware
  real llena el contexto usado por los modulos de cursos y catalogo.
- `FakeAuthMiddleware` se conserva en `internal/platform/authctx` solo como
  compatibilidad para pruebas/manualidades antiguas.
- La verificacion de correo usa Mailpit local; en produccion se cambiaria por
  un proveedor SMTP real.
- Las sesiones se revocan en Postgres y se eliminan de Redis cuando aplica.
- La auditoria es minima pero persistente y consultable por admin.
