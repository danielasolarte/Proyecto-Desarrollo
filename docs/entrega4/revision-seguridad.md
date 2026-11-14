# Revisión de seguridad · Entrega 4

Responsable: Samara. Revisión de `infra/terraform/identidades.tf`, `secretos.tf`, `pubsub.tf`, `cloudrun.tf` (servicios y permisos de invocación) y `base.tf` (red, Cloud SQL, bucket y accesos de personas), contra el criterio *Configuración y acceso* de la rúbrica: permisos mínimos, workers protegidos por IAM y secretos en Secret Manager.

La tabla de cuentas está en [`arquitectura.md`](arquitectura.md) y su verificación contra lo desplegado en [`evidencias.md`](evidencias.md) §6. El diagrama de permisos está en [`diagramas/identidades.mmd`](diagramas/identidades.mmd) ([imagen](diagramas/identidades.png)).

## 1. Qué cumple

| Control | Dónde | Resultado |
|---|---|---|
| Una cuenta de servicio por responsabilidad | `identidades.tf` | ✅ Seis cuentas: frontend, backend, worker, pubsub-push, migraciones y medios. Ningún servicio usa la cuenta por defecto de Compute. |
| Permisos de proyecto mínimos | `identidades.tf` | ✅ A nivel de proyecto solo `logging.logWriter` y `monitoring.metricWriter`, y solo para las cuentas que ejecutan código (no para `sa-pubsub-push` ni `sa-medios`). |
| Permisos sobre el recurso concreto | `secretos.tf`, `pubsub.tf`, `cloudrun.tf`, `identidades.tf` | ✅ Cada secreto se da uno por uno; publicar se da sobre el tópico; el bucket, sobre el bucket; invocar, sobre cada servicio. |
| Secretos con el menor alcance | `secretos.tf` | ✅ `mooc-database-url`: backend, worker y migraciones. Llave HMAC: backend y worker. `mooc-admin-password`: solo backend. El frontend no lee ningún secreto. |
| Worker protegido por IAM | `cloudrun.tf`, `pubsub.tf` | ✅ Sin `allUsers`. El único `run.invoker` es `sa-pubsub-push`. La suscripción push envía un token OIDC con `audience` igual a la URL del worker. Una llamada sin identidad recibe 403 ([`evidencias.md`](evidencias.md) §5). |
| Firma del token OIDC acotada | `pubsub.tf` | ✅ El agente de Pub/Sub tiene `serviceAccountTokenCreator` solo sobre `sa-pubsub-push`, no sobre el proyecto. |
| Ningún secreto en Terraform | `secretos.tf`, `backend.tf` | ✅ Terraform crea solo los contenedores; los valores los carga `deploy/gcp/e4/01-secretos.ps1`. No hay `google_sql_user` con clave ni recursos `random_password`, así que el estado no guarda claves. `terraform.tfvars` está en `.gitignore`. |
| Ningún secreto en las imágenes | `cloudrun.tf` | ✅ Los secretos llegan como variables de entorno desde Secret Manager (`value_source.secret_key_ref`) al arrancar el contenedor; no se copian en la imagen. |
| Base de datos y bucket privados | `base.tf` | ✅ Cloud SQL sin IP pública (`ipv4_enabled = false`); backend y worker llegan por la VPC con `PRIVATE_RANGES_ONLY`. Bucket con `uniform_bucket_level_access` y `public_access_prevention = "enforced"`. |
| Personas con acceso mínimo | `base.tf` | ✅ El grupo: ver, publicar imágenes y encender o apagar Cloud SQL (`cloudsql.editor` no permite borrarla). El equipo docente: solo lectura. Solo quien corre Terraform es propietario. |
| Revisión automática en el CI | `.github/workflows/ci.yml` | ✅ `terraform fmt` y `terraform validate` en el job `terraform-ci`; el paso *Secret guard (Terraform)* falla si se versiona un `.tfstate` o un `terraform.tfvars`, o si un `.tf` tiene un valor que parece secreto. |

## 2. Observaciones

Ninguna deja un permiso abierto ni un secreto expuesto. Quedan como mejoras, con su justificación:

| Id | Observación | Riesgo | Recomendación |
|---|---|---|---|
| S1 | El backend tiene `allUsers` como invocador y se puede llamar directamente por su URL `run.app`, sin pasar por el frontend. | Bajo: todas las rutas de datos exigen sesión y CSRF en la aplicación; es la misma superficie que expone el frontend por `/api`. Es necesario porque nginx reenvía `/api` por la URL pública del backend. | Si se quiere cerrar: backend con `ingress = INGRESS_TRAFFIC_INTERNAL_ONLY` y el frontend saliendo a la VPC con `ALL_TRAFFIC`, o nginx enviando un token de identidad de `sa-frontend` con `run.invoker` solo para esa cuenta. |
| S2 | Cloud SQL acepta conexiones sin cifrar (`ssl_mode = "ALLOW_UNENCRYPTED_AND_ENCRYPTED"`). | Bajo: la base solo tiene IP privada y el tráfico no sale de la VPC. | `ssl_mode = "ENCRYPTED_ONLY"` y `sslmode=require` en `DATABASE_URL`, como en la E3. |
| S3 | `sa-medios` tiene `storage.objectAdmin` sobre el bucket. | Muy bajo: con acceso uniforme no hay ACL por objeto, y el permiso es solo sobre ese bucket. | `roles/storage.objectUser` alcanza (leer, escribir y borrar objetos) sin permisos de administración. |

## 3. Pendiente: secretos en el estado de Terraform

[`evidencias.md`](evidencias.md) §7 deja pendiente revisar el estado, porque leer su bucket requiere el rol de quien corre Terraform. Lo corre **Camila** desde la raíz del repositorio:

```powershell
terraform -chdir=infra/terraform state pull > $env:TEMP\estado.json
Select-String -Path $env:TEMP\estado.json -Pattern 'postgres://[^:]+:[^@]+@','PRIVATE KEY','"secret_data"','"private_key"' | Measure-Object | Select-Object -ExpandProperty Count
Remove-Item $env:TEMP\estado.json
```

El resultado debe ser **0**. Guardar la salida en `k6/results/e4/sin-secretos-estado-<fecha>.txt` (solo el conteo, nunca el estado) y la captura como `07-sin-secretos-estado.png`.
