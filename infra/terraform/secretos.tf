# Los secretos ya existen en Secret Manager (los creó la E3 con
# deploy/gcp/e3/01-secretos-identidades.ps1). Terraform solo los lee como
# datos y da acceso a quien los necesita: nunca lee ni escribe sus valores,
# así que no quedan en el estado ni en el repositorio.
#
# Para cambiar un valor (por ejemplo, rotar la clave de Postgres):
#   gcloud secrets versions add mooc-database-url --data-file=<archivo temporal>
# Cloud Run toma la versión "latest" en la siguiente revisión.

data "google_secret_manager_secret" "app" {
  for_each = var.secretos

  secret_id = each.value
}

locals {
  # Quién lee cada secreto (por variable de entorno).
  lectores_secretos = {
    DATABASE_URL  = ["backend", "worker", "migraciones"]
    S3_ACCESS_KEY = ["backend", "worker"]
    S3_SECRET_KEY = ["backend", "worker"]
  }

  accesos_secretos = merge([
    for variable, cuentas in local.lectores_secretos : {
      for cuenta in cuentas : "${variable}|${cuenta}" => { variable = variable, cuenta = cuenta }
    }
  ]...)
}

resource "google_secret_manager_secret_iam_member" "lectura" {
  for_each = local.accesos_secretos

  secret_id = data.google_secret_manager_secret.app[each.value.variable].id
  role      = "roles/secretmanager.secretAccessor"
  member    = google_service_account.sa[each.value.cuenta].member
}
