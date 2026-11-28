# Entrega 5 · Guía de despliegue

Todo el despliegue se hace con `terraform init`, `terraform plan` y `terraform apply`, sin configuración manual en la consola. Los scripts de `deploy/gcp/e4/` siguen sirviendo como atajos: pasan el proyecto y la etiqueta para no escribirlos en cada comando.

## 1. Requisitos (una vez)

- Proyecto de GCP con facturación activa.
- `gcloud`, Terraform 1.6 o superior y Docker.
- Quien corre Terraform necesita el rol Propietario del proyecto, porque crea cuentas de servicio y permisos y actúa como `sa-build-funcion` y `sa-funcion-validar` al desplegar la función.
- Autenticación:

  ```powershell
  gcloud auth login
  gcloud auth application-default login
  gcloud config set project <proyecto>
  gcloud auth configure-docker us-central1-docker.pkg.dev
  ```

- Bucket del estado, una sola vez (Terraform no puede crear el bucket que guarda su propio estado): `.\deploy\gcp\e4\00-bucket-estado.ps1`.
- Valores de los secretos, una sola vez: `.\deploy\gcp\e4\01-secretos.ps1`. Terraform solo crea los contenedores de los secretos; los valores nunca pasan por el estado ni por el repositorio.

## 2. Parámetros del entorno

Copia `infra/terraform/terraform.tfvars.example` como `infra/terraform/terraform.tfvars` (git lo ignora) y ajusta:

- `project_id` y `region`;
- `image_tag`: commit corto de las imágenes publicadas;
- `funcion_validar`: `true` (valor por defecto). Desde que el worker ya no valida, con `false` las cargas pasarían sin validar;
- `equipo` y `docentes`: correos con acceso de lectura.

Otros parámetros, con valores por defecto en `variables.tf`:

- `servicios_habilitados`
- `image_tag_por_servicio`
- `topic_trabajos` y `topic_validacion`
- `funcion_validar_config` (memoria, CPU, tiempo máximo, instancias, concurrencia, conexiones a la base)
- `nivel_sql`

## 3. Preparar imágenes y código

- **Imágenes de Cloud Run** (frontend, backend, worker, migraciones): `.\deploy\gcp\e4\construir-imagenes.ps1` las construye y publica en Artifact Registry con el commit corto como etiqueta. Terraform convierte la etiqueta en el digest, así que despliega exactamente esa imagen.
- **Código de la función:** no hay que hacer nada aparte. Terraform empaqueta `functions/validar-carga/` como zip, lo sube al bucket de fuentes con el hash del contenido en el nombre, y Cloud Build lo compila durante `apply`. Si el código cambia, el hash cambia y `apply` actualiza la función.

## 4. Desplegar

```powershell
terraform -chdir=infra/terraform init -backend-config="bucket=mooc-tfstate-<proyecto>"
terraform -chdir=infra/terraform plan -out=tfplan
terraform -chdir=infra/terraform apply tfplan
```

Lo que pasa en `apply`, en orden:

1. Se activan las APIs (`cloudfunctions`, `cloudbuild` y `eventarc` son nuevas en la E5).
2. Se crean el topic `mooc-media-validate`, las cuentas `sa-funcion-validar`, `sa-disparo-validar` y `sa-build-funcion`, sus permisos y el bucket de fuentes.
3. El job `mooc-migraciones` se ejecuta solo cuando cambia su imagen, y `apply` espera a que termine. Así se aplica la migración `000006`, que permite `validate_upload`. Cloud SQL debe estar encendida.
4. Se compila y despliega la función con su disparador de Eventarc.
5. Sale la revisión nueva del backend con `PUBSUB_TOPIC_VALIDACION`.

Para confirmar que todo coincide con el código, un segundo `plan` debe decir `No changes`.

Tráfico de Cloud Run: Terraform fija el 100 % del tráfico de frontend, backend y worker en la última revisión (bloque `traffic` en `cloudrun.tf`). Así un `gcloud run services update-traffic` hecho a mano, como el del ensayo de reversión de la E4, aparece en `plan` y `apply` lo corrige; no hace falta ningún comando aparte. Para volver a una versión anterior se despliega su etiqueta (`image_tag`) con `plan` y `apply`.

## 5. Recursos que ya existen

- La E5 reutiliza el mismo estado remoto de la E4 (prefijo `entrega4` en el bucket de estado). Todo lo de la E4 ya está en ese estado, así que `plan` solo muestra lo nuevo de la E5.
- En un proyecto nuevo, `apply` crea todo desde cero.
- Si un recurso se creó a mano antes (por ejemplo, un topic con el mismo nombre), se importa antes del `plan`. Ejemplo: `terraform import google_pubsub_topic.validacion projects/<proyecto>/topics/mooc-media-validate`.
- Si `apply` falla porque una API recién activada todavía no responde, basta con repetir `plan` y `apply`.
- La primera vez que se crea la función en un proyecto, Google crea su bucket de fuentes (`gcf-v2-sources-<número del proyecto>-<región>`). El permiso de lectura de `sa-build-funcion` sobre ese bucket (`build_lee_fuentes_gcf` en `funcion.tf`) puede fallar en ese primer `apply` porque el bucket todavía no existe. Basta repetir `plan` y `apply`: la función se vuelve a crear con el permiso ya puesto. Así pasó en el proyecto de la entrega (evidencia 11: la función se reemplaza).

## 6. Volver atrás

Para volver a una versión anterior se despliega su etiqueta (`image_tag`) con `plan` y `apply`. No se apaga solo la función (`funcion_validar = false`): desde la E5 el worker ya no valida, así que las cargas pasarían sin validar.

## 7. Despliegues hechos con Terraform

Cuatro rondas, todas con `plan`, `apply` y un `plan` final en `No changes`:

- **23 de noviembre, base con la función apagada** (evidencias 01 a 03): `19 added, 4 changed`. Terraform ejecutó la migración 000006.
- **24 de noviembre, imágenes nuevas** con el arreglo del video (evidencias 04 a 06): `4 changed`.
- **25 de noviembre, tráfico** (evidencias 07 a 09): `1 changed`; el backend pasó de `mooc-backend-00003` a la última revisión.
- **26 de noviembre, función encendida** con `funcion_validar = true` (evidencias 10 a 12): el `plan` mostró la función, el permiso de invocación del disparador, la variable `PUBSUB_TOPIC_VALIDACION` del backend y las imágenes nuevas (`3 to add, 2 to change`). El primer `apply` pidió dos permisos que se agregaron en `funcion.tf`: `serviceUsageConsumer` para `sa-disparo-validar` y lectura del bucket de fuentes de Cloud Functions para `sa-build-funcion`. Después, `apply` creó `mooc-validar-carga` con su disparador de Eventarc y el `plan` final quedó en `No changes`.

## Despliegue base de la E5 con la función desactivada

El primer despliegue de la Entrega 5 se realizó desde `main`, commit `534bc6e`, manteniendo `funcion_validar = false`. Esto permitió desplegar y validar la infraestructura base de la E5 sin activar todavía la Cloud Function de validación ni su trigger de Eventarc.

Las imágenes de backend, frontend, worker y migraciones fueron construidas y publicadas con la etiqueta `534bc6e`.

El plan previo al despliegue fue:

```text
Plan: 19 to add, 4 to change, 0 to destroy.
```

Los recursos nuevos corresponden principalmente a la infraestructura preparatoria de la E5: el topic mooc-media-validate, el bucket de fuentes, las nuevas cuentas de servicio, los permisos requeridos y las APIs necesarias para Cloud Functions, Eventarc y Cloud Build.

El despliegue terminó correctamente:
Apply complete! Resources: 19 added, 4 changed, 0 destroyed.

Con funcion_validar = false no se desplegaron todavía la Cloud Function ni el trigger de Eventarc.
Migración 000006
Como parte del despliegue, Terraform ejecutó el job mooc-migraciones. La ejecución terminó correctamente con 1/1 tareas completadas.

En los registros apareció:
6/u validate_upload_job_type
Container called exit(0).

Esto confirma que la migración 000006 se aplicó correctamente y dejó disponible el nuevo tipo de trabajo validate_upload.
Verificación posterior
Después del apply se ejecutó nuevamente:
terraform -chdir=infra/terraform plan

El resultado fue:
No changes. Your infrastructure matches the configuration.

Esto confirma que la infraestructura desplegada coincide con la configuración declarada en Terraform.

Evidencias:
- evidencias/01-apply-base.txt
- evidencias/02-migracion-000006.txt
- evidencias/03-plan-sin-cambios-base.txt
