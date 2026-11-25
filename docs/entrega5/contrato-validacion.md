# Entrega 5 · Contrato de `validate_upload`

Acuerdo entre el backend (Daniela), la Cloud Function `mooc-validar-carga` (Mateo) y Terraform (Daniela y Camila). Extiende el contrato de la E4 ([`../entrega4/contrato-mensaje.md`](../entrega4/contrato-mensaje.md)) sin cambiar su versión.

## 1. Mensaje

- **Topic:** `mooc-media-validate` (variable `topic_validacion`).
- **Quién publica:** solo el backend, al confirmar una carga (`CompleteUpload`), y solo si tiene `PUBSUB_TOPIC_VALIDACION`.
- **data:** `{"job_id": "<uuid>"}`, igual que en la E4. Solo viaja el ID; Postgres es la fuente de verdad.
- **attributes:**
  - `job_type = validate_upload`
  - `contract_version = "1"`
- **Cómo llega a la función:** CloudEvent `google.cloud.pubsub.topic.v1.messagePublished`. El mensaje viene en `message.data` (bytes) y `message.attributes`.

## 2. Qué hace la función

1. Lee `job_id`. Si el mensaje está mal formado o no trae `job_id`, lo registra con severidad ERROR y **confirma** (devuelve `nil`): reintentarlo no lo arregla.
2. Toma el job con el mismo `UPDATE` atómico de `MediaRepository.ClaimJob` (`internal/media/postgres/media_repository.go`), con bloqueo de 60 s:
   - `ErrJobAlreadyTerminal` (el job ya está `done` o `dead_letter`): confirma sin hacer nada. Es un mensaje repetido.
   - `ErrJobLocked` (otra instancia lo tiene): devuelve error para que Eventarc reintente más tarde.
3. Marca el recurso como `processing`, como hace hoy el worker al empezar `scan_antivirus`.
4. Lee el original del bucket **en flujo**, sin guardarlo entero en memoria, y en una sola pasada calcula:
   - el SHA-256 real;
   - el MIME real (`http.DetectContentType` sobre los primeros 512 bytes);
   - en presentaciones, si empieza con la firma ZIP `PK\x03\x04`.

   Las reglas son las que tenía el worker hasta la E4 (`inspectFile`, `looksLikeZip` y `allowedMimeTypesByKind`); desde la E5 están en `functions/validar-carga/validacion.go` y se quitaron del worker.
5. **Si no pasa** (checksum distinto del declarado, MIME no permitido o presentación sin firma ZIP):
   - borra el original del bucket;
   - pone `attempts = max_attempts` y marca el job como fallido, con lo que queda en `dead_letter`;
   - pone el recurso en `failed`;
   - **confirma**. Es un fallo permanente, no se reintenta.
6. **Si pasa:**
   - guarda en `media_assets` el checksum y el MIME reales;
   - encola `scan_antivirus` en `mooc-media-jobs` con la clave `<asset_id>:scan_antivirus`, usando la misma lógica que `usecase.EnqueueJob`;
   - cierra el job como `done`.

   Primero se encola y después se cierra, como hace el worker con el paso siguiente. Si algo falla entre los dos, el reintento vuelve a validar y encolar no duplica nada (la misma clave). Al revés, un job `done` sin `scan_antivirus` dejaría la carga detenida.
7. **Error transitorio** (Postgres, Storage o Pub/Sub no responden): marca el job como fallido y devuelve error para que Eventarc reintente. Al llegar a `max_attempts` (3), el job pasa a `dead_letter`, el recurso a `failed` y la función **confirma**.

## 3. Garantías

- **Sin duplicados.** Un mensaje repetido encuentra el job `done`, o el `scan_antivirus` ya creado con la misma clave, y no hace nada.
- **Sin pérdidas.** El backend publica en el topic de validación solo cuando la función existe: Terraform pone `PUBSUB_TOPIC_VALIDACION` con `funcion_validar = true`, y la revisión del backend depende de la función.
- **Mismo resultado que la E4.** Los mismos campos quedan guardados, los mismos archivos se borran y la cadena sigue igual: `scan_antivirus`, luego `transcode_hls` o `convert_presentation_to_pdf`.
- **Tope de intentos en Postgres.** La suscripción que crea Eventarc no tiene tópico de fallidos, así que el tope lo pone `max_attempts`, igual que en el worker.

## 4. Cambios en el worker (Mateo)

- Quitar de `scan_antivirus` el checksum, el MIME y la firma ZIP. El escaneo con ClamAV y el encadenamiento no cambian.
- Si al worker le llega un `validate_upload` (por ejemplo, alguien lo publicó en `mooc-media-jobs` por error), lo rechaza: el job queda `dead_letter` con un mensaje claro y se confirma. Hoy ya cae en `tipo de job desconocido`; falta forzar que no se reintente.
- Orden: quitar la validación del worker **en el mismo despliegue** en que Camila activa `funcion_validar = true`. Si se quita antes, las cargas pasarían sin validar mientras la función está apagada.

## 5. Configuración de la función (Terraform)

Variables de entorno (sin secretos):

- `S3_BUCKET`: bucket de medios.
- `PUBSUB_PROJECT` y `PUBSUB_TOPIC`: dónde se encola `scan_antivirus`.
- `DB_MAX_CONNS`: 2 por instancia.

Secretos (Secret Manager, versión `latest`):

- `DATABASE_URL` ← `mooc-database-url`.

Identidad: `sa-funcion-validar`. Usa sus credenciales por defecto para Storage y Pub/Sub, así que no necesita las llaves HMAC.

Recursos y escalado (variable `funcion_validar_config`):

- 512 MiB y 1 vCPU;
- 300 s de tiempo máximo;
- de 0 a 3 instancias;
- 1 petición por instancia.

## 6. Cómo armar el código (Mateo decide)

La carpeta `functions/validar-carga/` es un módulo de Go aparte. Terraform la sube como zip y Cloud Build compila solo esa carpeta. Hay dos formas de reutilizar la lógica del backend:

- **Reutilizar los paquetes** `internal/media/...` con `replace github.com/equipo-mooc/plataforma-mooc => ../..` y `go mod vendor`. La carpeta `vendor/` viaja en el zip y Cloud Build compila sin salir del módulo. Así la función usa el mismo `ClaimJob` y el mismo `EnqueueJob`. Conviene sacar `inspectFile`, `looksLikeZip` y `allowedMimeTypesByKind` a un paquete compartido (por ejemplo `internal/media/validacion`) que usen el worker y la función.
- **Código propio y pequeño** con `pgx`, `cloud.google.com/go/storage` y `cloud.google.com/go/pubsub`, copiando las dos consultas (tomar el job y encolar) con pruebas que comprueben que se comportan igual.

En los dos casos, `go.sum` va en el repo y las pruebas (`*_test.go`) no viajan en el zip.

**Decisión (Mateo): código propio y pequeño**, pero sin los clientes oficiales de Storage y Pub/Sub. El backend pide Go 1.26 y las versiones actuales de `cloud.google.com/go/storage` y `pubsub` también, mientras que el runtime de la función es `go125`. Por eso:

- Storage y Pub/Sub se llaman por su API REST con `net/http` y el token del servidor de metadatos, igual que hace el backend en `internal/media/platform/pubsubqueue.go` (`servicios.go`).
- Las consultas de `ClaimJob`, `TouchJobLock`, `UpdateJob`, `UpdateAsset` y `CreateJob` están copiadas en `repositorio.go`. `TomarJob` agrega `job_type = 'validate_upload'`, así la función nunca toma un job del worker. Para crear `scan_antivirus` se usa `ON CONFLICT (idempotency_key) DO NOTHING`, que se apoya en la restricción `UNIQUE` de la 000004.
- `repositorio_test.go` prueba esas consultas contra un Postgres con las migraciones del repo, y `funcion_test.go` prueba los reintentos sin duplicados. Ver `functions/validar-carga/README.md`.
- Dependencias directas: `pgx`, `functions-framework-go` y `cloudevents`, todas compatibles con Go 1.25.
