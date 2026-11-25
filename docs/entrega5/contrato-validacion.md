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

   Las reglas son las mismas de `inspectFile`, `looksLikeZip` y `allowedMimeTypesByKind` en `internal/media/worker/processor.go`.
5. **Si no pasa** (checksum distinto del declarado, MIME no permitido o presentación sin firma ZIP):
   - borra el original del bucket;
   - pone `attempts = max_attempts` y marca el job como fallido, con lo que queda en `dead_letter`;
   - pone el recurso en `failed`;
   - **confirma**. Es un fallo permanente, no se reintenta.
6. **Si pasa:**
   - guarda en `media_assets` el checksum y el MIME reales;
   - cierra el job como `done`;
   - encola `scan_antivirus` en `mooc-media-jobs` con la clave `<asset_id>:scan_antivirus`, usando la misma lógica que `usecase.EnqueueJob`.
7. **Error transitorio** (Postgres, Storage o Pub/Sub no responden): marca el job como fallido y devuelve error para que Eventarc reintente. Al llegar a `max_attempts` (3), el job pasa a `dead_letter`, el recurso a `failed` y la función **confirma**.

## 3. Garantías

- **Sin duplicados.** Un mensaje repetido encuentra el job `done`, o el `scan_antivirus` ya creado con la misma clave, y no hace nada.
- **Sin pérdidas.** El backend publica en el topic de validación solo cuando la función existe: Terraform pone `PUBSUB_TOPIC_VALIDACION` con `funcion_validar = true`, y la revisión del backend depende de la función.
- **Mismo resultado que la E4.** Los mismos campos quedan guardados, los mismos archivos se borran y la cadena sigue igual: `scan_antivirus`, luego `transcode_hls` o `convert_presentation_to_pdf`.
- **Tope de intentos en Postgres.** La suscripción que crea Eventarc no tiene tópico de fallidos, así que el tope lo pone `max_attempts`, igual que en el worker.

## 4. Cambios en el worker

- `scan_antivirus` ya no verifica el checksum, el MIME ni la firma ZIP: solo escanea con ClamAV y encadena HLS o PDF.
- Si al worker le llega un `validate_upload` (por ejemplo, publicado en `mooc-media-jobs` por error), lo deja en `dead_letter` al primer intento con un mensaje claro y confirma el mensaje (`internal/media/worker/processor_test.go`).
- Por eso `funcion_validar` quedó en `true` por defecto en Terraform: worker y función se despliegan juntos. Con `false` las cargas pasarían sin validar.

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

## 6. Cómo está hecho el código

`functions/validar-carga/` es un módulo de Go aparte (Terraform sube solo esa carpeta). No importa los paquetes del backend, para no arrastrar a la función MinIO, asynq y Redis; repite las consultas que necesita, que son las mismas del backend:

- tomar el trabajo: el `UPDATE` condicional de `ClaimJob`;
- fallar o cerrar: las reglas de `MarkJobFailed` y `MarkJobDone`;
- encadenar: `INSERT ... ON CONFLICT (idempotency_key) DO NOTHING`, y se publica solo si el trabajo quedó `pending`, como en `EnqueueJob`.

Storage y Pub/Sub se usan por su API REST con el token de `sa-funcion-validar` que da el servidor de metadatos.

Dependencias: `functions-framework-go`, `cloudevents/sdk-go` y `pgx`. `go.sum` va en el repo y la CI lo comprueba con `go mod verify`.

Pruebas (`go test ./...` en la carpeta):

- archivo válido: guarda y encola el antivirus;
- mensaje repetido: no duplica;
- checksum falso y presentación sin ZIP: se borra el original y el trabajo queda en `dead_letter`;
- error transitorio: se reintenta y al tercer intento queda en `dead_letter`;
- falla de Pub/Sub: el reintento no duplica;
- trabajo tomado por otra instancia, inexistente o de otro tipo;
- latido del bloqueo;
- clientes REST de Storage, Pub/Sub y metadatos;
- lectura del CloudEvent.
