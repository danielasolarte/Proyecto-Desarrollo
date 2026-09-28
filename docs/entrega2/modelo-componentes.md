# Modelo de componentes

Responsable: Daniela Solarte (con aportes de Andrés Jurado para workers y multimedia).

Este documento describe los módulos de la API, el worker, la cola de mensajería,
la base de datos y el almacenamiento de objetos, sus responsabilidades y cómo
se comunican entre sí en el despliegue de la Entrega 2.

## Vista general

```text
                         HTTPS (Web Server)
                               |
                               v
                        API en Go (Echo)
                     AuthMiddleware + CSRFMiddleware
                               |
          -----------------------------------------------
          |          |          |          |             |
       courses    catalog     auth       admin        badges
          |                     |                        |
      progress <-------------------------------------- quizzes
          |
        media
          |
     (síncrono: HTTP)      (asíncrono: Redis / asynq, cola "media")
          |                               |
     PostgreSQL <---------------- Worker Server (Go, asynq.Server)
   (Cloud SQL)                       |     |        |
                                clamscan  ffmpeg  LibreOffice
                                     |
                          Cloud Storage (objetos: originales,
                          derivados HLS, PDFs, miniaturas)
```

## API (proceso `cmd/api`)

Un único binario Go, monolito modular, expuesto por Echo bajo `/api/v1` y
publicado solo en `127.0.0.1:8080` detrás del proxy HTTPS del Web Server
(ver `docs/entrega2/modelo-despliegue-red-web.md`).

Cadena de middleware aplicada a toda petición: `Logger` → `Recover` →
`AuthMiddleware` (resuelve el usuario desde `Authorization: Bearer`,
`X-Session-Token` o la cookie `mooc_session`, en ese orden) → `CSRFMiddleware`
(solo exige `X-CSRF-Token` cuando la sesión vino de cookie; los clientes con
Bearer, incluidos los scripts de k6, no la necesitan — contrato completo en
`docs/entrega2/secretos-y-seguridad.md`).

Módulos registrados en `cmd/api/main.go`, cada uno con su propio
`domain/usecase/postgres/http` (ver README, sección "Estructura"):

| Módulo | Responsabilidad | Depende de |
|---|---|---|
| `courses` | Cursos, versiones, módulos, unidades, recursos, autoría y publicación | PostgreSQL |
| `catalog` | Catálogo público de cursos publicados, inscripciones | PostgreSQL, `courses` |
| `auth` | Registro, verificación de correo, login, sesiones, recuperación de contraseña | PostgreSQL, Redis, SMTP (Mailpit) |
| `admin` | Gestión de usuarios, roles, auditoría | `auth` |
| `quizzes` | Preguntas, intentos, calificación con `Idempotency-Key` | PostgreSQL, `progress` |
| `progress` | Progreso por recurso y por curso, heartbeats validados en servidor | PostgreSQL, `courses`, `badges` |
| `badges` | Emisión y verificación pública de insignias | PostgreSQL, `progress` |
| `media` | Carga multipart, URLs prefirmadas, estado de procesamiento | PostgreSQL, Cloud Storage, Redis/asynq |

Todas las peticiones entre estos módulos y el cliente son **síncronas**
(HTTP/JSON). La única comunicación **asíncrona** de la API es hacia el
worker: al confirmar una carga multimedia, `media` encola un trabajo en
Redis (`mediaQueue`, cola `media`) y responde de inmediato sin esperar el
procesamiento.

## Worker (proceso `cmd/worker`)

Proceso Go independiente (no expone HTTP) que corre en el Worker Server.
Usa un servidor `asynq` que escucha la cola `media` en Redis, con
concurrencia configurable por `WORKER_CONCURRENCY` (número de trabajos que
procesa en paralelo dentro de una sola instancia; escalar horizontalmente
es levantar más instancias del contenedor, no subir este número
indefinidamente).

Tipos de trabajo (`internal/media/domain.JobType`):

- `JobScanAntivirus`: escaneo con ClamAV antes de aceptar un archivo.
- `JobTranscodeHLS`: transcodificación de video a HLS con ffmpeg.
- `JobConvertPresentationToPDF`: conversión de presentaciones a PDF con
  LibreOffice.

El mensaje que viaja por Redis solo trae el `job_id`; el worker relee el
job completo desde PostgreSQL antes de procesarlo, que es la fuente de
verdad. Esto hace el procesamiento **idempotente ante reintentos y
reentregas de Redis**: si un job ya quedó en `done` o `dead_letter`, el
worker lo descarta sin reprocesar. Job encolado pero nunca reclamado, o
reclamado pero fallido sin reintento agotado, son los dos puntos donde se
verifica reintento con backoff (evidencia pedida en la sustentación).

## Base de datos: PostgreSQL (Cloud SQL)

Fuente de verdad transaccional para los ocho módulos de la tabla anterior.
Acceso vía `pgxpool`, con el tamaño de pool configurable por variables de
entorno (`DB_MAX_CONNS`, `DB_MIN_CONNS`; por defecto 5 y 0), tanto en la
API como en el worker. Detalle de la instancia administrada, migraciones,
respaldo y recuperación en `docs/operacion-recuperacion.md` y
`docs/entrega2/costos-entrega2.md`.

## Cola de mensajería: Redis (sesiones + asynq)

Redis cumple dos roles distintos en el mismo despliegue, ambos en el
Worker Server:

- **Sesiones y caché** de `auth` (`RedisSessionStore`), alcanzado por la API
  desde el Web Server por IP interna.
- **Cola de trabajos `asynq`** que conecta `media` (API) con el worker.

El cliente Redis del proyecto no soporta contraseña, así que el aislamiento
de este puerto es responsabilidad de la regla de firewall
`mooc-e2-allow-redis-from-web` (detalle en
`docs/entrega2/modelo-despliegue-red-web.md`), no de Redis mismo.

## Almacenamiento de objetos: Cloud Storage

`internal/media/platform/s3storage.go` habla el protocolo compatible con
S3 de Cloud Storage (`S3_ENDPOINT=storage.googleapis.com`). Guarda
originales, derivados HLS, PDFs convertidos y miniaturas; nunca binarios en
PostgreSQL. La API firma URLs de carga y descarga directas entre el
navegador/cliente y Cloud Storage: ni la carga ni la descarga pasan por la
API como proxy de bytes.

`S3_PUBLIC_ENDPOINT` (opcional) distingue el host que la API usa
internamente para hablar con el storage del host que se firma dentro de
una URL prefirmada para un cliente externo, igual que ya ocurría con MinIO
en la Entrega 1.

## Integraciones externas

- **SMTP (Mailpit):** verificación de correo y recuperación de contraseña;
  de laboratorio, sin cambios frente a la Entrega 1.
- **ClamAV:** escaneo antivirus síncrono dentro del job `JobScanAntivirus`,
  en el Worker Server.
- **Cloud Storage:** ver arriba.
- **Cloud SQL:** ver arriba.
