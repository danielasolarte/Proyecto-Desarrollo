# Entrega 5 · Guía de despliegue (borrador)

> Borrador de Daniela. Camila lo completa con los requisitos que confirme al aplicar, y Samara lo revisa siguiéndolo paso a paso.

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
- `funcion_validar`: `true` para la E5, `false` para volver al flujo de la E4;
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

## 5. Recursos que ya existen

- La E5 reutiliza el mismo estado remoto de la E4 (prefijo `entrega4` en el bucket de estado). Todo lo de la E4 ya está en ese estado, así que `plan` solo muestra lo nuevo de la E5.
- En un proyecto nuevo, `apply` crea todo desde cero.
- Si un recurso se creó a mano antes (por ejemplo, un topic con el mismo nombre), se importa antes del `plan`. Ejemplo: `terraform import google_pubsub_topic.validacion projects/<proyecto>/topics/mooc-media-validate`.
- Si `apply` falla porque una API recién activada todavía no responde, basta con repetir `plan` y `apply`.

## 6. Volver atrás

Con `funcion_validar = false`, `plan` y `apply` borran la función y su disparador, y quitan la variable del backend. Las cargas vuelven a empezar por `scan_antivirus` en el worker. Ojo: esto solo da el mismo resultado si el worker todavía valida; ver la sección 4 de [`contrato-validacion.md`](contrato-validacion.md).

## 7. Pendiente (Camila y Samara)

- Confirmar los permisos que pida Cloud Build o Eventarc en el primer `apply`.
- Evidencias:
  - `plan` y `apply` iniciales;
  - un cambio aplicado (por ejemplo, `max_instancias`);
  - `plan` sin cambios;
  - una carga real que pase por la función.
