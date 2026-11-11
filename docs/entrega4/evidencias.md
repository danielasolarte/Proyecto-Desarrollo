# Evidencias de la Entrega 4

Comandos y consultas para obtener cada evidencia que pide la rúbrica. Están pensados para copiar y pegar en PowerShell desde la raíz del repositorio, con `gcloud` autenticado en el proyecto de la E4. Cada sección dice qué captura guardar y dónde se usa.

Las capturas van en `docs/entrega4/evidencias/` con el nombre que indica cada sección.

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

```powershell
git grep -n -I -E "postgres://[^:]+:[^@]+@|BEGIN (RSA )?PRIVATE KEY|GOOG[0-9A-Z]{20,}" -- . ":!*.md"
terraform -chdir=infra/terraform state pull | Select-String -Pattern "postgres://|PRIVATE KEY|secret_data"
```

Los dos deben salir vacíos. El primero busca cadenas de conexión con clave, llaves privadas e identificadores de llaves HMAC; el segundo revisa que el estado de Terraform no tenga valores de secretos. Captura: `07-sin-secretos.png`.

## 8. Reentrega sin efectos duplicados

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

## 9. Despliegue reproducible y reversión

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
