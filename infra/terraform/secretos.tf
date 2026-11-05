# Secretos de la aplicación. Terraform crea solo el CONTENEDOR de cada
# secreto y da permiso de lectura a quien lo necesita; nunca conoce los
# valores, así que no quedan en el estado ni en el repositorio.
#
# Los valores los genera y carga deploy/gcp/e4/01-secretos.ps1:
#   mooc-database-url   usuario y clave nuevos de Postgres + IP privada
#   mooc-s3-access-key  llave HMAC de sa-medios (URL firmadas del bucket)
#   mooc-s3-secret-key  secreto de esa llave HMAC
#
# Rotar un valor: volver a correr el script con -Rotar; Cloud Run toma la
# versión "latest" en la siguiente revisión.

locals {
  # Variable de entorno de la aplicación => nombre del secreto.
  secretos = {
    DATABASE_URL  = "mooc-database-url"
    S3_ACCESS_KEY = "mooc-s3-access-key"
    S3_SECRET_KEY = "mooc-s3-secret-key"
  }

  # Quién lee cada secreto.
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

resource "google_secret_manager_secret" "app" {
  for_each = local.secretos

  secret_id = each.value
  replication {
    auto {}
  }

  depends_on = [google_project_service.apis]
}

resource "google_secret_manager_secret_iam_member" "lectura" {
  for_each = local.accesos_secretos

  secret_id = google_secret_manager_secret.app[each.value.variable].id
  role      = "roles/secretmanager.secretAccessor"
  member    = google_service_account.sa[each.value.cuenta].member
}
