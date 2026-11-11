# Guía de despliegue de la Entrega 4 (borrador)

> Borrador de Daniela. Camila la ejecuta en su proyecto y la corrige con lo que encuentre; cada responsable agrega los pasos de su servicio.

La E4 corre en el proyecto de Camila. El proyecto de la E2/E3 quedó sin facturación porque se agotó el crédito educativo, así que **toda** la infraestructura se crea desde cero con Terraform (`infra/terraform/`): red, Cloud SQL, bucket de medios, Artifact Registry, Pub/Sub, cuentas de servicio y los tres servicios de Cloud Run. Nada se crea a mano en la consola.

## Requisitos

- Proyecto de GCP con facturación activa.
- `gcloud`, Terraform 1.6 o superior y Docker Desktop.
- Para quien corre Terraform: rol Propietario del proyecto (crea cuentas de servicio y permisos).

Una sola vez por persona:

```powershell
gcloud auth login
gcloud auth application-default login
gcloud config set project <proyecto>
gcloud auth configure-docker us-central1-docker.pkg.dev
```

## 1. Configurar el proyecto (una vez, Camila)

En `deploy/gcp/e4/proyecto.ps1`, reemplaza el valor por el ID del proyecto y haz commit.

Para dar acceso de lectura y de publicación de imágenes al resto del grupo, crea `infra/terraform/terraform.tfvars` (git lo ignora):

```hcl
equipo = ["correo1@...", "correo2@...", "correo3@..."]
```

## 2. Bucket del estado e inicialización

```powershell
.\deploy\gcp\e4\00-bucket-estado.ps1
.\deploy\gcp\e4\tf.ps1 init
```

## 3. Crear la base

Con todos los valores de `servicios_habilitados` en `false` (`infra/terraform/variables.tf`):

```powershell
.\deploy\gcp\e4\tf.ps1 plan
.\deploy\gcp\e4\tf.ps1 apply
```

Crea la red, Cloud SQL (tarda 10 a 15 minutos), el bucket, Artifact Registry, Pub/Sub, las cuentas de servicio y los contenedores de los secretos.

## 4. Valores de los secretos

```powershell
.\deploy\gcp\e4\01-secretos.ps1
```

Crea el usuario de Postgres con una clave aleatoria y una llave HMAC para el bucket, y los guarda en Secret Manager sin mostrarlos.

## 5. Imágenes, migraciones y backend

```powershell
.\deploy\gcp\e4\construir-imagenes.ps1 -Servicios backend,migraciones
```

Primero las migraciones, para que el backend arranque con las tablas creadas. En `variables.tf`, pon `migraciones = true` y luego:

```powershell
.\deploy\gcp\e4\tf.ps1 plan -Tag <etiqueta que imprimió el script>
.\deploy\gcp\e4\tf.ps1 apply
.\deploy\gcp\e4\migrar.ps1
```

Después pon `backend = true` y repite `tf.ps1 plan -Tag <etiqueta>` y `tf.ps1 apply`.

Al arrancar, el backend crea la cuenta de administrador con el correo `correo_admin` de `variables.tf` si la base no tiene ninguno. En los registros del backend debe aparecer `administrador inicial: administrador creado`. La clave se consulta una sola vez, cuando haga falta:

```powershell
gcloud secrets versions access latest --secret=mooc-admin-password
```

Con esa cuenta se crean los profesores y estudiantes de prueba desde la aplicación o con `npm run cuentas` del frontend.

## 6. Frontend y worker

Lo más simple es construir todas las imágenes con el mismo commit, así una sola etiqueta sirve para todo:

```powershell
git checkout main
git pull
.\deploy\gcp\e4\construir-imagenes.ps1 -Servicios backend,migraciones,frontend,worker
```

En `variables.tf`, `frontend = true` y `worker = true`, y luego:

```powershell
.\deploy\gcp\e4\tf.ps1 plan -Tag <etiqueta que imprimió el script>
.\deploy\gcp\e4\tf.ps1 apply
```

Con el worker se crea también la suscripción push `mooc-media-jobs-worker` (OIDC con `sa-pubsub-push`) y el permiso para que solo esa cuenta invoque el worker. El worker tarda entre 20 y 40 s en arrancar porque `clamd` carga las firmas antes de abrir el puerto.

Si algún servicio necesita una etiqueta distinta, en `variables.tf` se fija `image_tag_por_servicio` (por ejemplo `{ worker = "a1b2c3d" }`).

Comprobación rápida del worker: sube un PDF o un video desde el frontend como profesor y sigue su `job_id` en Cloud Logging (`docs/entrega4/evidencias.md`, sección 2). Una llamada sin identidad al worker debe responder 403 (sección 5).

## 7. Verificar

```powershell
.\deploy\gcp\e4\tf.ps1 output
$b = terraform -chdir=infra/terraform output -raw url_backend
curl.exe -s "$b/health/ready"
```

Al final, `.\deploy\gcp\e4\tf.ps1 plan` debe decir `No changes`: lo desplegado es igual a lo del repositorio.

## 8. Prueba de escalamiento del worker

`k6/run-rafaga-worker-e4.ps1` sube una ráfaga de archivos con una cuenta de profesor y mide cuánto tarda cada uno en quedar listo. Con Cloud SQL encendida y Docker Desktop abierto:

```powershell
$env:TEACHER_PASSWORD = "<clave del profesor de prueba>"
.\k6\run-rafaga-worker-e4.ps1 -Tipo presentacion -Rafaga 1    # capacidad de un worker (c)
.\k6\run-rafaga-worker-e4.ps1 -Tipo presentacion -Rafaga 9    # ráfaga: la cola crece y se vacía
.\k6\run-rafaga-worker-e4.ps1 -Tipo video -Rafaga 6
```

Los resultados quedan en `k6/results/e4/`. Mientras corre, se capturan las gráficas de la sección 4 de `docs/entrega4/evidencias.md`.

## Pendiente

- Camila: ejecutar esta guía de punta a punta y corregir lo que falte.
