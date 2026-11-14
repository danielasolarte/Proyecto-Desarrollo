# Evidencias de la Entrega 4

Este documento reúne la evidencia de cada criterio de la rúbrica: qué se verificó, qué resultado dio y dónde está el archivo. Debajo de cada resultado quedan los comandos para repetir la verificación, pensados para PowerShell desde la raíz del repositorio con `gcloud` autenticado en el proyecto de la E4.

Las capturas están en `docs/entrega4/evidencias/` y las salidas de comandos y de k6, en `k6/results/e4/`.

## Resumen

| Sección | Qué se verifica | Estado |
|---|---|---|
| 1 | Servicios desplegados, máximo 3 instancias y configuración | Listo |
| 2 | Una tarea de punta a punta en los registros | Listo |
| 3 | Suscripción push autenticada y tópico de fallidos | Listo |
| 4 | Métricas de instancias y mensajes pendientes | Listo; el tiempo de procesamiento del worker se mide de nuevo |
| 5 | El worker solo acepta a Pub/Sub | Listo |
| 6 | Cuentas de servicio y permisos mínimos | Listo |
| 7 | Ningún secreto en el repositorio ni en el estado | Repositorio listo; falta el estado de Terraform |
| 8 | Reentrega sin efectos duplicados | Reentrega lista; falta el reemplazo de instancia |
| 9 | Despliegue reproducible y reversión | Listo |

URL desplegadas:

- Frontend: https://mooc-frontend-yufxihdm4q-uc.a.run.app
- Backend: https://mooc-backend-yufxihdm4q-uc.a.run.app
- Worker (privado, responde 403 sin identidad): https://mooc-worker-yufxihdm4q-uc.a.run.app

## 0. Preparar la terminal

```powershell
. .\deploy\gcp\e4\proyecto.ps1
$P = $PROYECTO_E4
$R = "us-central1"
$fe = terraform -chdir=infra/terraform output -raw url_frontend
$be = terraform -chdir=infra/terraform output -raw url_backend
$wk = terraform -chdir=infra/terraform output -raw url_worker
$bucket = terraform -chdir=infra/terraform output -raw bucket_medios
```

## 1. Servicios desplegados y su configuración

**Resultado.** Los tres servicios corren en `us-central1` con `minScale: '0'` y `maxScale: '3'`. Backend: concurrencia 40, timeout de 60 s, 1 vCPU y 1 GiB. Frontend: concurrencia 80, 60 s, 1 vCPU y 512 MiB. Worker: concurrencia 1, 600 s, 2 vCPU y 4 GiB, con salida a la VPC solo para rangos privados. La configuración completa de los tres está en [`arquitectura.md`](arquitectura.md).

**Evidencia**

- `k6/results/e4/backend-cloudrun-config.txt`
- `k6/results/e4/frontend-cloudrun-config.txt`
- `k6/results/e4/worker-cloudrun-config.txt`

**Cómo repetirlo**

Rúbrica: despliegue e integración; documentar región, instancias, CPU, memoria, concurrencia y timeout.

```powershell
gcloud run services list --region=$R --project=$P
foreach ($s in "mooc-frontend", "mooc-backend", "mooc-worker") {
    Write-Host "==== $s ===="
    gcloud run services describe $s --region=$R --project=$P --format="yaml(spec.template.metadata.annotations,spec.template.spec.containerConcurrency,spec.template.spec.timeoutSeconds,spec.template.spec.containers[0].resources,spec.template.spec.containers[0].image)"
}
```

Qué debe verse en cada servicio:

- `autoscaling.knative.dev/maxScale: '3'` (máximo 3 instancias).
- La concurrencia, el timeout, la CPU y la memoria de [`arquitectura.md`](arquitectura.md).
- La imagen con `@sha256:` (desplegada por digest) en Artifact Registry.

Capturas: `01-servicios.png` (la lista) y `01-<servicio>.png` (cada descripción). En la consola, lo mismo está en **Cloud Run** → el servicio → pestaña **Revisiones**.

## 2. Una tarea de punta a punta en los registros

**Resultado.** El 11 de noviembre se siguió el trabajo `29cd1378-e4f0-4ca0-af3e-443903943861`: a las 23:11:18 el backend lo publicó (`message_id` 20354242745055623), Pub/Sub lo entregó al worker en el primer intento (`delivery_attempt` 1) y a las 23:11:19 el worker lo terminó y guardó (`estado_final` done). La aplicación muestra el recurso procesado.

**Evidencia**

- `k6/results/e4/worker-punta-a-punta-20261111.txt`
- `docs/entrega4/evidencias/02-tarea-29cd1378-e4f0-4ca0-af3e-443903943861.png`
- `docs/entrega4/evidencias/02-tarea-app.png`

**Cómo repetirlo**

Rúbrica: Pub/Sub y workers; registros que siguen una tarea.

1. Sube un video o un PDF desde el frontend como profesor.
2. Busca el `job_id` que publicó el backend:

   ```powershell
   gcloud logging read 'resource.type="cloud_run_revision" AND resource.labels.service_name="mooc-backend" AND textPayload:"pubsub: publicado"' --project=$P --limit=5 --freshness=1h --format="value(timestamp,textPayload)"
   ```

3. Copia el `job_id` de la línea más reciente y sigue la tarea en todos los servicios:

   ```powershell
   $job = "<job_id>"
   gcloud logging read "resource.type=`"cloud_run_revision`" AND `"$job`"" --project=$P --freshness=1h --order=asc --format="table(timestamp,resource.labels.service_name,textPayload,jsonPayload.message)"
   ```

Debe verse, en orden: la publicación en `mooc-backend` (con `message_id`), la entrega en `mooc-worker` (`message_id`, número de entrega), el resultado guardado y, si es un video o una presentación, el trabajo encadenado (HLS o PDF) con su propio `job_id`.

En la consola, **Logging** → **Explorador de registros**, pega esta consulta:

```
resource.type="cloud_run_revision"
"<job_id>"
```

Captura: `02-tarea-<job_id>.png`. Después, el estado visible en la aplicación (recurso procesado): `02-tarea-app.png`.

## 3. Suscripción push y tópico de fallidos

**Resultado.** La suscripción `mooc-media-jobs-worker` envía a `https://mooc-worker-yufxihdm4q-uc.a.run.app/pubsub/push` con token OIDC de `sa-pubsub-push`, plazo de confirmación de 600 s, reintentos de 10 s a 600 s y tópico de fallidos `mooc-media-jobs-dlq` tras 10 entregas. La suscripción de revisión `mooc-media-jobs-dlq-revision` guarda los fallidos 7 días.

**Evidencia**

- `k6/results/e4/pubsub-worker-config-20261111.txt`
- `k6/results/e4/pubsub-dlq-config-20261111.txt`

**Cómo repetirlo**

```powershell
gcloud pubsub subscriptions describe mooc-media-jobs-worker --project=$P --format="yaml(pushConfig,ackDeadlineSeconds,retryPolicy,deadLetterPolicy)"
```

Debe verse:

- `pushEndpoint` terminado en `/pubsub/push`.
- `oidcToken.serviceAccountEmail` igual a `sa-pubsub-push@...` (push autenticado).
- `ackDeadlineSeconds: 600`.
- Reintentos de 10 s a 600 s.
- Tópico de fallidos `mooc-media-jobs-dlq` con 10 entregas como máximo.

Captura: `03-suscripcion.png`.

## 4. Métricas: instancias y mensajes pendientes

**Resultado.** El backend llegó a 3 instancias y el frontend a 2 bajo carga, sin errores. En la ráfaga de cargas, el worker llegó a 3 instancias, la suscripción acumuló unos 5 mensajes pendientes, el más antiguo esperó unos 20 s y luego la cola volvió a 0, sin mensajes en el tópico de fallidos. El detalle está en [`escalamiento.md`](escalamiento.md). El tiempo hasta que cada recurso queda listo se está midiendo de nuevo.

**Evidencia**

- `docs/entrega4/evidencias/backend-instance-count-20261109.png`
- `docs/entrega4/evidencias/frontend-instance-count-20261109.png`
- `docs/entrega4/evidencias/04-instancias.png`
- `docs/entrega4/evidencias/04-pendientes.png`
- `docs/entrega4/evidencias/04-edad.png`
- `docs/entrega4/evidencias/04-respuestas.png`

**Cómo repetirlo**

Rúbrica: escalamiento. En **Monitoring** → **Explorador de métricas**, cambia el editor a **PromQL** y pega cada consulta. Rango: el de la prueba de carga.

Instancias por servicio (nunca más de 3):

```
sum by (service_name) (run_googleapis_com:container_instance_count{service_name=~"mooc-.*", state="active"})
```

Mensajes pendientes en la suscripción del worker:

```
pubsub_googleapis_com:subscription_num_undelivered_messages{subscription_id="mooc-media-jobs-worker"}
```

Edad del mensaje más antiguo sin confirmar (segundos):

```
pubsub_googleapis_com:subscription_oldest_unacked_message_age{subscription_id="mooc-media-jobs-worker"}
```

Respuestas del worker al push, por código (204, 429, 500):

```
sum by (response_code) (rate(pubsub_googleapis_com:subscription_push_request_count{subscription_id="mooc-media-jobs-worker"}[1m]))
```

Mensajes enviados al tópico de fallidos (debe quedar en 0 en una prueba sana):

```
pubsub_googleapis_com:subscription_dead_letter_message_count{subscription_id="mooc-media-jobs-worker"}
```

Capturas: `04-instancias.png`, `04-pendientes.png`, `04-edad.png`, `04-respuestas.png`. Estas gráficas van también en `escalamiento.md`, junto al modelo de capacidad.

## 5. Acceso: el worker solo acepta a Pub/Sub

**Resultado.** Una llamada sin identidad a `/pubsub/push` recibe `403 Forbidden`. En el worker, el único con `roles/run.invoker` es `sa-pubsub-push`; frontend y backend tienen `allUsers`.

**Evidencia**

- `k6/results/e4/worker-403-20261111.txt`
- `k6/results/e4/worker-iam-policy-20261111.txt`
- `k6/results/e4/frontend-iam.txt` y `k6/results/e4/backend-iam.txt`

**Cómo repetirlo**

Rúbrica: configuración y acceso.

Sin identidad, el worker responde 403:

```powershell
curl.exe -s -o NUL -w "sin identidad: %{http_code}`n" -X POST "$wk/pubsub/push"
```

Con la identidad de una persona del grupo (que no tiene `run.invoker` sobre el worker), también 403:

```powershell
$t = gcloud auth print-identity-token
curl.exe -s -o NUL -w "con usuario: %{http_code}`n" -X POST -H "Authorization: Bearer $t" "$wk/pubsub/push"
```

Quién puede invocar cada servicio (en el worker solo `sa-pubsub-push`; en frontend y backend, `allUsers`):

```powershell
foreach ($s in "mooc-frontend", "mooc-backend", "mooc-worker") {
    Write-Host "==== $s ===="
    gcloud run services get-iam-policy $s --region=$R --project=$P --format="table(bindings.role,bindings.members)"
}
```

Capturas: `05-403.png` y `05-invocadores.png`.

## 6. Cuentas de servicio y permisos mínimos

**Resultado.** Coincide con [`arquitectura.md`](arquitectura.md). A nivel de proyecto, `sa-frontend`, `sa-backend`, `sa-worker` y `sa-migraciones` solo tienen `roles/logging.logWriter` y `roles/monitoring.metricWriter`. Los secretos se dan uno por uno con `roles/secretmanager.secretAccessor`: `mooc-database-url` a backend, worker y migraciones; las dos partes de la llave del bucket a backend y worker; `mooc-admin-password` solo al backend. La política del tópico no se pudo leer con permisos de solo lectura; según `infra/terraform/pubsub.tf`, publican `sa-backend` y `sa-worker`.

**Evidencia**

- `k6/results/e4/permisos-20261114.txt`

**Cómo repetirlo**

Roles de proyecto de cada cuenta:

```powershell
gcloud projects get-iam-policy $P --flatten="bindings[].members" --filter="bindings.members:serviceAccount:sa-" --format="table(bindings.members,bindings.role)"
```

Quién lee cada secreto:

```powershell
foreach ($s in "mooc-database-url", "mooc-s3-access-key", "mooc-s3-secret-key", "mooc-admin-password") {
    Write-Host "==== $s ===="
    gcloud secrets get-iam-policy $s --project=$P --format="table(bindings.role,bindings.members)"
}
```

Quién publica en el topic y quién toca el bucket:

```powershell
gcloud pubsub topics get-iam-policy mooc-media-jobs --project=$P --format="table(bindings.role,bindings.members)"
gcloud storage buckets get-iam-policy "gs://$bucket" --format="table(bindings.role,bindings.members)"
```

Debe coincidir con la lista de [`arquitectura.md`](arquitectura.md). Captura: `06-permisos.png`.

## 7. Ningún secreto en el repositorio ni en el estado

**Resultado.** La búsqueda en el repositorio no encontró secretos reales: las coincidencias son la base local de desarrollo (`mooc:mooc` en localhost), plantillas con `CAMBIAR_CLAVE`, variables de entorno y la propia búsqueda de la integración continua; el archivo de evidencia explica cada una. Falta revisar el estado de Terraform, que requiere permiso de lectura sobre su bucket. Los resúmenes de k6 ya no guardan tokens de sesión: `k6/run-rafaga-worker-e4.ps1` quita `setup_data` al terminar, y los tokens de las entregas anteriores en `k6/results/` se reemplazaron por `<redactado>`.

**Evidencia**

- `k6/results/e4/sin-secretos-repo-20261114.txt`
- Pendiente: la revisión del estado de Terraform

**Cómo repetirlo**

```powershell
git grep -n -I -E "postgres://[^:]+:[^@]+@|BEGIN (RSA )?PRIVATE KEY|GOOG[0-9A-Z]{20,}" -- . ":!*.md"
terraform -chdir=infra/terraform state pull | Select-String -Pattern "postgres://|PRIVATE KEY|secret_data"
```

Los dos deben salir vacíos. El primero busca cadenas de conexión con clave, llaves privadas e identificadores de llaves HMAC; el segundo revisa que el estado de Terraform no tenga valores de secretos. Captura: `07-sin-secretos.png`.

## 8. Reentrega sin efectos duplicados

**Resultado.** El 11 de noviembre se publicó dos veces el mensaje de un trabajo HLS ya terminado. Las dos entregas quedaron registradas como `reentrega de un job ya terminado, no se procesa` (`estado_final` ya_terminado): el worker respondió 204 sin repetir la conversión. La prueba de reemplazo de instancia está pendiente.

**Evidencia**

- `k6/results/e4/worker-idempotencia-hls-20261111.txt`
- Pendiente: `08-reemplazo-antes.png`, `08-reemplazo-registros.png` y `08-reemplazo-despues.png`

**Cómo repetirlo**

Rúbrica: el worker tolera reentregas. Usa un `job_id` de una tarea que ya terminó (sección 2) y su tipo.

Cuenta los derivados antes:

```powershell
gcloud storage ls -r "gs://$bucket/**" | Select-String "<id del recurso>" | Measure-Object
```

Publica el mismo mensaje dos veces, como si Pub/Sub lo reentregara:

```powershell
1..2 | ForEach-Object {
    gcloud pubsub topics publish mooc-media-jobs --project=$P --message="{`"job_id`":`"$job`"}" --attribute="job_type=transcode_hls,contract_version=1"
}
```

Debe verse en los registros del worker que el job ya estaba terminado y que respondió 204 sin procesar (sección 2 con el mismo `$job`). Repite el conteo de derivados: el número no cambia. Captura: `08-reentrega.png`.

### Reemplazo de instancia

Rúbrica: el worker tolera que una instancia muera a mitad de un trabajo. Cómo funciona: [contrato-mensaje.md](contrato-mensaje.md), "Bloqueo y reemplazo de instancias".

Cloud Run no permite matar una instancia a mano, así que la prueba deja el job exactamente como lo dejaría una instancia que murió: en `processing`, con un bloqueo que nadie renueva. Usa el trabajo HLS de la sección 2 (el encadenado), que ya terminó: `$job = "<job_id del HLS>"`.

1. En la consola, **SQL** → `mooc-e4-postgres` → **Cloud SQL Studio**. Base `mooc`, usuario `mooc`. La clave va dentro de la URL del secreto `mooc-database-url` (`gcloud secrets versions access latest --secret=mooc-database-url --project=$P`). No la pegues en capturas ni en el chat.
2. Simula la instancia muerta:

   ```sql
   UPDATE media_jobs SET status = 'processing', locked_at = now() WHERE id = '<job_id>';
   SELECT id, status, attempts, locked_at FROM media_jobs WHERE id = '<job_id>';
   ```

   Captura el `SELECT`: `08-reemplazo-antes.png`.
3. Enseguida, publica el mensaje como haría Pub/Sub al reintentar:

   ```powershell
   gcloud pubsub topics publish mooc-media-jobs --project=$P --message="{`"job_id`":`"$job`"}" --attribute="job_type=transcode_hls,contract_version=1"
   ```

4. Espera unos 2 minutos y mira la secuencia en el worker:

   ```powershell
   gcloud logging read "resource.type=`"cloud_run_revision`" AND resource.labels.service_name=`"mooc-worker`" AND jsonPayload.job_id=`"$job`"" --project=$P --freshness=1h --order=asc --format="table(timestamp,jsonPayload.message,jsonPayload.estado_final,jsonPayload.delivery_attempt,jsonPayload.intento)"
   ```

   Debe verse:
   - Mientras el bloqueo tiene menos de 60 s, una o varias entregas con `estado_final=ocupado`: el worker respondió 429 y no tocó el job.
   - Cuando el bloqueo vence, una entrega con `job tomado, procesando` y `intento` uno más que antes.
   - Después, `job terminado y guardado` con `estado_final=done`.

   Captura: `08-reemplazo-registros.png`.
5. Repite el `SELECT`: `status = done` y `attempts` subió en 1. Repite el conteo de derivados de arriba: el número no cambia, porque el HLS se reescribe en las mismas rutas. Captura: `08-reemplazo-despues.png`.

## 9. Despliegue reproducible y reversión

**Resultado.** Con la etiqueta desplegada (`255f3ec`), `tf.ps1 plan -Tag 255f3ec` respondió `No changes. Your infrastructure matches the configuration.` En la reversión, el tráfico del backend pasó de `mooc-backend-00003-wmm` a `mooc-backend-00002-84s`, `GET /health` respondió 200 y se restauró la revisión actual.

**Evidencia**

- `k6/results/e4/terraform-plan-sin-cambios-20261112.txt`
- `k6/results/e4/backend-rollback-20261112.txt`

**Cómo repetirlo**

Lo desplegado es igual a lo del repositorio:

```powershell
.\deploy\gcp\e4\tf.ps1 plan
```

Debe decir `No changes. Your infrastructure matches the configuration.` Captura: `09-sin-cambios.png`.

Revisiones de un servicio y reversión a la anterior (ensayo para el video):

```powershell
gcloud run revisions list --service=mooc-backend --region=$R --project=$P --format="table(metadata.name,metadata.creationTimestamp,status.conditions[0].status)"
gcloud run services update-traffic mooc-backend --region=$R --project=$P --to-revisions=<revisión anterior>=100
```

Al terminar el ensayo, vuelve a dejar todo como dice Terraform con `tf.ps1 plan` y `tf.ps1 apply`. Captura: `09-reversion.png`.
