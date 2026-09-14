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
- Persona 4: quizzes, intentos, progreso de recursos, progreso de cursos e insignias    verificables.
- Infraestructura local base con Docker Compose: PostgreSQL, Redis, MinIO,
  Mailpit y ClamAV.
- API base con Echo.
- Worker base con asynq.
- Migraciones SQL con golang-migrate.
- OpenAPI parcial en api/openapi.yaml.

Pendiente para completar el alcance total del proyecto:

- Persona 3: multimedia, URLs prefirmadas, carga multipart, ClamAV real,
  procesamiento HLS con ffmpeg, DLQ y endpoints PDF/HLS.
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
- Creacion de administradores desde administracion.
- CRUD administrativo basico de usuarios, roles y estados.
- Revocacion individual y masiva de sesiones.
- Auditoria basica append-only.
- Proteccion del ultimo administrador activo.
- Pruebas de casos de uso de Auth/Admin.

### Persona 4 - Quizzes, Progreso e Insignias

Cubre:

- Migraciones para quizzes, questions, question_options, attempts,
  attempt_answers, resource_progress, progress_events,
  course_progress, badges y badge_issuances.

#### Quizzes

- Creacion de quizzes asociados a recursos de tipo quiz.
- Creacion de preguntas de seleccion multiple y sus opciones.
- Configuracion de puntaje minimo de aprobacion, numero maximo de intentos,
  limite de tiempo y modo de feedback.
- Inicio de intentos por estudiantes inscritos.
- Snapshot del quiz al iniciar el intento para conservar su contenido durante
  la resolucion.
- El snapshot entregado al estudiante no expone las respuestas correctas.
- Guardado parcial de respuestas y recuperacion de intentos en progreso.
- Validacion de que la pregunta y opcion seleccionada pertenezcan al quiz.
- Calificacion realizada exclusivamente en el servidor.
- Calculo de puntaje, porcentaje y estado de aprobacion.
- Manejo de expiracion de intentos.
- Submit final protegido mediante Idempotency-Key.
- Reintentos del mismo submit devuelven el resultado existente.

#### Progreso

- Seguimiento de progreso por recurso mediante identificadores estables.
- Registro de apertura de recursos.
- Heartbeats para contabilizar tiempo de actividad validado por el servidor.
- Registro de posicion de reproduccion o lectura.
- Rechazo de valores de progreso calculados directamente por el cliente.
- Tiempo minimo de actividad antes de permitir completar recursos normales.
- Registro de eventos de progreso.
- Los quizzes se marcan como completados al realizar el submit final.
- Registro independiente de eventos quiz_submitted y quiz_passed.
- Calculo del porcentaje del curso a partir de recursos visibles y obligatorios.
- Estado completed cuando todos los recursos obligatorios fueron completados.
- Estado approved cuando, ademas, todos los quizzes obligatorios fueron
  aprobados.
- Conservacion del progreso mediante stable_id.

#### Insignias

- Definicion de una insignia por curso.
- Emision automatica cuando el progreso del curso alcanza approved.
- Una sola emision por estudiante, curso e insignia.
- Codigo UUID unico de verificacion.
- Endpoint publico de verificacion sin autenticacion.
- La verificacion publica no expone el correo del estudiante.
- Revocacion administrativa de insignias.
- Una insignia revocada permanece consultable pero se reporta como invalida.

## Estructura

cmd/
  api/                  main del servidor HTTP
  worker/               main del worker asynq

internal/
  admin/                endpoints administrativos
  auth/                 identidad, sesiones y middleware
  badges/               insignias, emisiones y verificacion publica
  catalog/              catalogo e inscripciones
  courses/              cursos, versionado, autoria y editor
  progress/             progreso por recurso y curso
  quizzes/              quizzes, preguntas, intentos y calificacion
  platform/             utilidades compartidas

migrations/             migraciones SQL de golang-migrate
api/                    OpenAPI 3.1
docs/                   documentacion por modulo
docker-compose.yml      infraestructura local

Cada modulo sigue el patron:


domain/    entidades, errores e interfaces
usecase/   reglas de negocio
postgres/  implementacion SQL
http/      handlers y rutas Echo


## Requisitos

- Go instalado. En esta maquina se uso `go1.26.6`; el proyecto no usa codigo
  especifico de esa version, pero `go mod tidy` dejo `go 1.26.0` en `go.mod`.
- Docker y Docker Compose.
- `golang-migrate` para aplicar migraciones.

Instalar golang-migrate si no esta disponible:

go install -tags "postgres" github.com/golang-migrate/migrate/v4/cmd/migrate@latest


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
curl localhost:8080/health
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

Crear otro administrador desde administracion:

```bash
curl -X POST localhost:8080/api/v1/admin/users/admins \
  -H "Authorization: Bearer TOKEN_ADMIN" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"admin2@example.com\",\"password\":\"claveSegura123\",\"full_name\":\"Admin Dos\"}"
```

Consultar un usuario por ID:

```bash
curl localhost:8080/api/v1/admin/users/USER_ID \
  -H "Authorization: Bearer TOKEN_ADMIN"
```

Actualizar rol, estado o nombre de un usuario:

```bash
curl -X PATCH localhost:8080/api/v1/admin/users/USER_ID \
  -H "Authorization: Bearer TOKEN_ADMIN" \
  -H "Content-Type: application/json" \
  -d "{\"status\":\"suspended\"}"
```

Cuando un usuario activo queda `suspended` o `deleted`, sus sesiones se revocan
en PostgreSQL y se eliminan de Redis.

Listar sesiones de un usuario:

```bash
curl localhost:8080/api/v1/admin/users/USER_ID/sessions \
  -H "Authorization: Bearer TOKEN_ADMIN"
```

Revocar todas las sesiones de un usuario:

```bash
curl -X DELETE localhost:8080/api/v1/admin/users/USER_ID/sessions \
  -H "Authorization: Bearer TOKEN_ADMIN"
```

Revocar una sesion individual:

```bash
curl -X DELETE localhost:8080/api/v1/admin/sessions/SESSION_ID \
  -H "Authorization: Bearer TOKEN_ADMIN"
```

Consultar auditoria reciente:

```bash
curl localhost:8080/api/v1/admin/audit-log \
  -H "Authorization: Bearer TOKEN_ADMIN"
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

## Flujo rapido de Quizzes

Crear un quiz para un recurso como profesor:

```bash
curl -X POST localhost:8080/api/v1/resources/RESOURCE_ID/quiz \
  -H "Authorization: Bearer TOKEN_PROFESOR" \
  -H "Content-Type: application/json" \
  -d "{\"passing_score\":60,\"max_attempts\":3,\"time_limit_seconds\":900,\"feedback_mode\":\"after_submit\"}"
```

Crear una pregunta:

```bash
curl -X POST localhost:8080/api/v1/quizzes/QUIZ_ID/questions \
  -H "Authorization: Bearer TOKEN_PROFESOR" \
  -H "Content-Type: application/json" \
  -d "{\"text\":\"Pregunta de prueba\",\"position\":1,\"points\":1}"
```

Crear una opción:

```Bash
curl -X POST localhost:8080/api/v1/questions/QUESTION_ID/options \
  -H "Authorization: Bearer TOKEN_PROFESOR" \
  -H "Content-Type: application/json" \
  -d "{\"text\":\"Respuesta A\",\"position\":1,\"is_correct\":true}"
```

Iniciar intento como estudiante:

```Bash
curl -X POST localhost:8080/api/v1/quizzes/QUIZ_ID/attempts \
  -H "Authorization: Bearer TOKEN_ESTUDIANTE"
```
Guardar una respuesta:

```Bash
curl -X PUT localhost:8080/api/v1/attempts/ATTEMPT_ID/answers/QUESTION_ID \
  -H "Authorization: Bearer TOKEN_ESTUDIANTE" \
  -H "Content-Type: application/json" \
  -d "{\"selected_option_id\":\"OPTION_ID\"}"
```

Enviar intento final:

```Bash
curl -X POST localhost:8080/api/v1/attempts/ATTEMPT_ID/submit \
  -H "Authorization: Bearer TOKEN_ESTUDIANTE" \
  -H "Idempotency-Key: submit-intento-1"
```
La calificacion es calculada en el servidor. El cliente nunca recibe la clave
de respuestas correctas dentro del snapshot utilizado durante el intento.


## Flujo rapido de Progeeso

Abrir un recurso:

```Bash
curl -X POST localhost:8080/api/v1/resources/RESOURCE_ID/progress/open \
  -H "Authorization: Bearer TOKEN_ESTUDIANTE"
```
Registrat actividad:

```Bash
curl -X POST localhost:8080/api/v1/resources/RESOURCE_ID/progress/heartbeat \
  -H "Authorization: Bearer TOKEN_ESTUDIANTE" \
  -H "Content-Type: application/json" \
  -d "{\"position_seconds\":30}"
```

Completar un recurso:

```Bash
curl -X POST localhost:8080/api/v1/resources/RESOURCE_ID/progress/complete \
  -H "Authorization: Bearer TOKEN_ESTUDIANTE"
```

Consultar progreso curso:

```Bash
curl localhost:8080/api/v1/courses/COURSE_ID/progress \
  -H "Authorization: Bearer TOKEN_ESTUDIANTE"
```

El porcentaje de progreso se calcula exclusivamente en el servidor a partir
de los recursos visibles y obligatorios de la version publicada.

## Flujo rapido de Insignias

Crear la insignia de un curso como profesor propietario o administrador:

```Bash
curl -X POST localhost:8080/api/v1/courses/COURSE_ID/badge \
  -H "Authorization: Bearer TOKEN_PROFESOR" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"Curso completado\",\"description\":\"Insignia por aprobar el curso\",\"image_url\":\"https://example.com/badge.png\"}"
```

Cuando un estudiante alcanza el estado approved, la insignia se emite
automaticamente.
Cada emision contiene un verification_code UUID unico.

Verificar insignia publicamente:

```Bash
curl localhost:8080/api/v1/badges/verify/VERIFICATION_CODE
```

Este endpoint es publico y no requiere autenticacion.

Revocar emision como admin:

```Bash
curl -X POST localhost:8080/api/v1/badge-issuances/ISSUANCE_ID/revoke \
  -H "Authorization: Bearer TOKEN_ADMIN"
```

Una insignia revocada continua siendo verificable, pero su campo valid
se reporta como false.


## Pruebas

Ejecutar toda la suite:

```bash
go test ./...
```

En Windows, si Go no puede escribir en `AppData\Local\go-build`, usar una cache
local dentro del repo:

```powershell
$env:GOCACHE = (Resolve-Path .).Path + '\.gocache'
go test ./...
```

Actualmente hay pruebas automaticas para Auth/Admin:

- registro de estudiante y token de verificacion;
- verificacion de correo y login;
- creacion del primer admin;
- proteccion del ultimo admin activo.
- endpoint HTTP de registro;
- middleware HTTP con ruta sin token, token de estudiante y token de admin.

Adicionalmente, se cuenta con una coleccion reproducible de Postman para validar de punta a punta los modulos de quizzes, progreso e insignias.

La coleccion cubre:

- creacion de quizzes, preguntas y opciones;
- inicio de intentos;
- snapshot del quiz sin exposicion de respuestas correctas;
- guardado parcial y recuperacion de intentos;
- calificacion realizada en el servidor;
- submit final con Idempotency-Key;
- repeticion idempotente del submit;
- calculo del progreso del curso;
- transicion a estado approved;
- emision automatica de insignias;
- verificacion publica de insignias;
- revocacion de insignias;
- control de acceso por rol;
- rechazo de solicitudes sin autenticacion;
- rechazo de heartbeats multimedia con posiciones invalidas.

## OpenAPI

La especificacion esta en:

api/openapi.yaml


Incluye endpoints de:

- identidad y autenticacion;
- administracion de usuarios y sesiones;
- cursos, autoria y versionado;
- editor Markdown;
- catalogo e inscripciones;
- multimedia y cargas multipart;
- procesamiento y reproduccion de recursos multimedia;
- quizzes, preguntas y opciones;
- intentos, guardado parcial y calificacion;
- progreso de recursos y cursos;
- heartbeats y validacion de progreso multimedia;
- insignias;
- emision automatica de insignias;
- verificacion publica de insignias;
- revocacion de insignias.

La especificacion usa OpenAPI 3.1 y define los esquemas de request/response, parametros de ruta, autenticacion, respuestas de error y endpoints publicos y protegidos de la plataforma.

## Notas de entrega

- El backend ya no depende de headers falsos para autenticar: el middleware
  real llena el contexto usado por los modulos de cursos y catalogo.
- `FakeAuthMiddleware` se conserva en `internal/platform/authctx` solo como
  compatibilidad para pruebas/manualidades antiguas.
- La verificacion de correo usa Mailpit local; en produccion se cambiaria por
  un proveedor SMTP real.
- Las sesiones se revocan en Postgres y se eliminan de Redis cuando aplica.
- La auditoria es minima, persistente, consultable por admin y append-only a
  nivel de base de datos mediante triggers.
