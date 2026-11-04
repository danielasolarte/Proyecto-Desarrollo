# Guía de despliegue de la Entrega 4 (borrador)

> Borrador de Daniela con los pasos que ya están probados. Cada responsable agrega los de su servicio, y Camila la valida siguiéndola desde cero antes del 13 de noviembre.

Toda la infraestructura nueva se crea con Terraform (`infra/terraform/`). Lo que ya existía de la E2 y la E3 (VPC, Cloud SQL, bucket de medios, Artifact Registry y los secretos) se lee como datos y no se modifica.

## Requisitos

- `gcloud` con sesión iniciada en el proyecto `proyecto1-entrega2-desarrollo`.
- Terraform 1.6 o superior.
- Docker Desktop corriendo.
- Permisos de editor en el proyecto.

Una sola vez por equipo:

```powershell
gcloud auth login
gcloud auth application-default login
gcloud config set project proyecto1-entrega2-desarrollo
gcloud auth configure-docker us-central1-docker.pkg.dev
```

## 1. Bucket del estado (una sola vez para todo el grupo)

```powershell
.\deploy\gcp\e4\00-bucket-estado.ps1
```

## 2. Inicializar Terraform

```powershell
cd infra\terraform
terraform init
cd ..\..
```

## 3. Publicar imágenes

Con todo en un commit:

```powershell
.\deploy\gcp\e4\construir-imagenes.ps1 -Servicios backend,migraciones
```

El script imprime la etiqueta (el commit corto) y el comando de Terraform.

## 4. Crear o actualizar la infraestructura

```powershell
cd infra\terraform
terraform plan -var "image_tag=<commit>" -out tfplan
terraform apply tfplan
terraform output
cd ..\..
```

Qué servicios se crean lo decide `servicios_habilitados` en `variables.tf`.

## 5. Migraciones

```powershell
.\deploy\gcp\e4\migrar.ps1 -Accion version
.\deploy\gcp\e4\migrar.ps1
```

## 6. Verificar

```powershell
$b = terraform -chdir=infra/terraform output -raw url_backend
curl.exe -s "$b/health/ready"
```

## Pendiente de cada responsable

- Mateo: imagen del worker, `servicios_habilitados.worker = true`, verificación de la suscripción push.
- Samara: imagen del frontend, `servicios_habilitados.frontend = true`, CORS del bucket con la URL del frontend.
- Camila: validar esta guía desde cero y anotar lo que falte.
