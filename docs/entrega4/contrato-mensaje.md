# Contrato del mensaje de trabajos multimedia (Entrega 4)

El backend publica en el topic `mooc-media-jobs` y Pub/Sub entrega cada mensaje al worker por push (`POST /pubsub/push`). Este documento es el acuerdo entre las dos partes. El código que lo implementa está en `internal/media/platform/pubsub_contract.go` (tipos y decodificación, compartidos por backend y worker) y `internal/media/platform/pubsubqueue.go` (publicación).

## Mensaje

- **data** (JSON, Pub/Sub lo transporta en base64):

  ```json
  {"job_id": "8f0c2c1e-5b8e-4f57-9d3e-1b2a3c4d5e6f"}
  ```

  Solo viaja el ID. El worker relee el `media_job` en Postgres, que es la fuente de verdad, igual que en la E3 con asynq.

- **attributes**:
  - `job_type`: `scan_antivirus`, `transcode_hls` o `convert_presentation_to_pdf`.
  - `contract_version`: `"1"`.

  Los atributos sirven para registros y filtros. El worker decide qué hacer con el `job_type` que lee de Postgres, no con el atributo.

## Sobre que llega al worker

```json
{
  "message": {
    "data": "eyJqb2JfaWQiOiI4ZjBjLi4uIn0=",
    "attributes": {"job_type": "transcode_hls", "contract_version": "1"},
    "messageId": "1234567890",
    "publishTime": "2026-11-09T15:04:05Z"
  },
  "subscription": "projects/proyecto1-entrega2-desarrollo/subscriptions/mooc-media-jobs-worker",
  "deliveryAttempt": 1
}
```

`platform.DecodePushEnvelope(body)` devuelve el `job_id` y el sobre. Si el cuerpo no se puede leer o no trae `job_id`, devuelve `platform.ErrInvalidMessage`.

## Reglas de respuesta del worker

| Situación | Respuesta | Efecto en Pub/Sub |
|---|---|---|
| Mensaje inválido (`ErrInvalidMessage`) o `job_id` que no existe | 204 y registro de error | Se confirma; reintentar no lo arregla |
| Job `done` o `dead_letter` | 204 sin procesar | Se confirma (es una reentrega) |
| Job `processing` con bloqueo reciente (otra entrega lo está haciendo) | 429 | Reintenta con espera creciente |
| Procesado y guardado con éxito | 204, solo después de guardar derivados y estado | Se confirma |
| Error con intentos disponibles | 500 | Reintenta con espera creciente |
| Error y se agotaron los intentos del job | Marcar `dead_letter` y 204 | Se confirma; no se escala un error repetible |

La suscripción además manda al tópico `mooc-media-jobs-dlq` cualquier mensaje entregado 5 veces sin confirmarse.

## Registros

Cada paso registra el mismo `job_id`, así una consulta en Cloud Logging sigue la tarea de punta a punta:

- backend: `pubsub: publicado job_id=… job_type=… message_id=…`
- worker: `job_id`, `message_id`, `deliveryAttempt` y estado final.

## Idempotencia

- `usecase.EnqueueJob` usa claves de idempotencia: repetir una carga no crea otro job.
- Si un job sigue `pending` (por ejemplo, porque publicar falló), repetir la operación lo vuelve a publicar. Un mensaje duplicado es inofensivo por las reglas de arriba.

## Cambios al contrato

- Compatibles: agregar campos opcionales a `data` o atributos nuevos.
- Incompatibles: subir `contract_version` y hacer que el worker acepte la versión vieja y la nueva mientras queden mensajes viejos en la suscripción.
