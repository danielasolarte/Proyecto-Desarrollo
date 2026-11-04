# APIs que usa la E4. disable_on_destroy = false: un destroy de la E4 no debe
# apagar APIs que también usan la E2/E3 (Compute, Cloud SQL, Storage).
locals {
  apis = toset([
    "run.googleapis.com",
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
