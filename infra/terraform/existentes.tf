# Recursos que ya existen desde la E2/E3. Terraform solo los LEE (data): no
# los crea, no los modifica y un "terraform destroy" no los borra. Así la
# base de datos y los archivos de los cursos quedan fuera del ciclo de vida
# de la E4.

data "google_project" "actual" {
  project_id = var.project_id
}

data "google_compute_network" "vpc" {
  name = var.red
}

data "google_compute_subnetwork" "subred" {
  name   = var.subred
  region = var.region
}

data "google_sql_database_instance" "postgres" {
  name = var.instancia_sql
}

data "google_storage_bucket" "medios" {
  name = var.bucket_medios
}

data "google_artifact_registry_repository" "imagenes" {
  location      = var.region
  repository_id = var.repositorio_imagenes
}

locals {
  registro = "${var.region}-docker.pkg.dev/${var.project_id}/${var.repositorio_imagenes}"

  # Cuenta de servicio administrada por Google que usa Pub/Sub para firmar
  # los tokens de push y para mover mensajes al tópico de fallidos.
  agente_pubsub = "service-${data.google_project.actual.number}@gcp-sa-pubsub.iam.gserviceaccount.com"
}
