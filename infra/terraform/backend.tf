# Estado remoto de Terraform (Entrega 4).
#
# El estado vive en un bucket de Cloud Storage con versionado, no en el
# repositorio: así todo el grupo trabaja sobre el mismo estado y GCS lo
# bloquea mientras alguien hace apply. El bucket se crea una sola vez con
# deploy/gcp/e4/00-bucket-estado.ps1 (Terraform no puede crear el bucket que
# guarda su propio estado).
#
# El nombre del bucket depende del proyecto, así que se pasa al inicializar
# (deploy/gcp/e4/tf.ps1 init lo hace solo):
#   terraform init -backend-config="bucket=mooc-tfstate-<proyecto>"
#
# El estado no guarda ningún secreto: la clave de Postgres y las llaves HMAC
# se generan y se cargan en Secret Manager por fuera de Terraform
# (deploy/gcp/e4/01-secretos.ps1).
terraform {
  backend "gcs" {
    prefix = "entrega4"
  }
}
