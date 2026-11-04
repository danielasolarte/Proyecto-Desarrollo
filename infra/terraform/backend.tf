# Estado remoto de Terraform (Entrega 4).
#
# El estado vive en un bucket de Cloud Storage con versionado, no en el
# repositorio: así todo el grupo trabaja sobre el mismo estado y GCS bloquea
# el estado mientras alguien hace apply. El bucket se crea una sola vez con
# deploy/gcp/e4/00-bucket-estado.ps1 (Terraform no puede crear el bucket que
# guarda su propio estado).
#
# El estado no guarda ningún secreto: los valores de los secretos se
# cargan en Secret Manager por fuera de Terraform (ver secretos.tf).
terraform {
  backend "gcs" {
    bucket = "mooc-tfstate-proyecto1-entrega2-desarrollo"
    prefix = "entrega4"
  }
}
