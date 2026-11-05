# APIs que usa la E4. disable_on_destroy = false: un destroy no apaga APIs
# (apagarlas puede borrar recursos que no maneja Terraform).
locals {
  apis = toset([
    "run.googleapis.com",
    "compute.googleapis.com",
    "sqladmin.googleapis.com",
    "servicenetworking.googleapis.com",
    "storage.googleapis.com",
    "cloudresourcemanager.googleapis.com",
    "pubsub.googleapis.com",
    "secretmanager.googleapis.com",
    "iam.googleapis.com",
    "iamcredentials.googleapis.com",
    "artifactregistry.googleapis.com",
    "monitoring.googleapis.com",
    "logging.googleapis.com",
  ])
}

resource "google_project_service" "apis" {
  for_each = local.apis

  service            = each.value
  disable_on_destroy = false
}
