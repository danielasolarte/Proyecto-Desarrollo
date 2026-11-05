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

## 5. Imágenes, backend y migraciones

```powershell
.\deploy\gcp\e4\construir-imagenes.ps1 -Servicios backend,migraciones
```

En `variables.tf`, pon `migraciones = true` y `backend = true`, y luego:

```powershell
.\deploy\gcp\e4\tf.ps1 plan -Tag <etiqueta que imprimió el script>
.\deploy\gcp\e4\tf.ps1 apply
.\deploy\gcp\e4\migrar.ps1
```

## 6. Frontend y worker

Igual que el paso 5, con `-Servicios frontend` o `-Servicios worker` y su valor en `servicios_habilitados`. Si la etiqueta de un servicio es distinta, `tf.ps1 plan` admite la etiqueta común con `-Tag` y en `variables.tf` se puede fijar `image_tag_por_servicio`.

## 7. Verificar

```powershell
.\deploy\gcp\e4\tf.ps1 output
$b = terraform -chdir=infra/terraform output -raw url_backend
curl.exe -s "$b/health/ready"
```

Al final, `.\deploy\gcp\e4\tf.ps1 plan` debe decir `No changes`: lo desplegado es igual a lo del repositorio.

## Pendiente

- Cuenta de administrador en la base nueva (la E3 lo hacía desde la VM de servicios, que ya no existe).
- Mateo: worker y verificación de la suscripción push.
- Samara: frontend (el CORS del bucket ya toma la URL del frontend desde Terraform).
- Camila: ejecutar esta guía y corregir lo que falte.
