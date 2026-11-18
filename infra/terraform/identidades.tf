# Una cuenta de servicio por responsabilidad, con permisos mínimos.
# Invocar un servicio y leer sus datos son permisos distintos y van en
# cuentas distintas (clase 11).
#
#   sa-frontend     sirve la SPA y reenvía /api; no toca datos.
#   sa-backend      publica en el topic; lee DATABASE_URL y las llaves S3.
#   sa-worker       publica trabajos encadenados; lee DATABASE_URL y las llaves S3.
#   sa-pubsub-push  identidad del token OIDC del push; solo puede invocar el worker.
#   sa-migraciones  corre el job de migraciones; solo lee DATABASE_URL.
#   sa-funcion-validar  (E5) ejecuta la Cloud Function de validación: lee y
#                   borra originales del bucket, lee DATABASE_URL y publica
#                   el siguiente trabajo (scan_antivirus).
#   sa-disparo-validar  (E5) identidad del disparador de Eventarc; solo
#                   puede invocar la función.
#   sa-build-funcion    (E5) compila la función en Cloud Build; solo lee el
#                   código fuente y publica la imagen en Artifact Registry.
#   sa-medios       dueña de la llave HMAC con la que backend y worker firman
#                   URL y leen o escriben en el bucket; solo tiene permiso
#                   sobre ese bucket.
#
# Los permisos sobre recursos concretos están junto a cada recurso
# (secretos.tf, pubsub.tf, cloudrun.tf). Aquí van solo los de proyecto.

locals {
  cuentas = {
    frontend    = "Frontend (nginx) de la E4 en Cloud Run"
    backend     = "Backend (API) de la E4 en Cloud Run"
    worker      = "Workers de medios de la E4 en Cloud Run"
    pubsub-push = "Identidad del push de Pub/Sub hacia el worker"
    migraciones = "Job de migraciones de la E4"
    medios      = "Dueña de la llave HMAC del bucket de medios"
    # Entrega 5. Se crean aunque la función esté apagada: así los permisos
    # quedan listos y encender la función no depende de crear cuentas.
    funcion-validar = "Cloud Function de validación de cargas (E5)"
    disparo-validar = "Disparador de Eventarc de la función de validación (E5)"
    build-funcion   = "Compilación de la Cloud Function en Cloud Build (E5)"
  }
}

resource "google_service_account" "sa" {
  for_each = local.cuentas

  account_id   = "sa-${each.key}"
  display_name = each.value

  depends_on = [google_project_service.apis]
}

# Registros y métricas para los que ejecutan código. sa-pubsub-push no
# ejecuta nada, así que no los necesita.
locals {
  roles_observabilidad = {
    for par in setproduct(["frontend", "backend", "worker", "migraciones", "funcion-validar"], ["roles/logging.logWriter", "roles/monitoring.metricWriter"]) :
    "${par[0]}|${par[1]}" => { cuenta = par[0], rol = par[1] }
  }
}

resource "google_project_iam_member" "observabilidad" {
  for_each = local.roles_observabilidad

  project = var.project_id
  role    = each.value.rol
  member  = google_service_account.sa[each.value.cuenta].member
}

# sa-medios solo puede leer y escribir objetos del bucket de medios.
resource "google_storage_bucket_iam_member" "medios_objetos" {
  bucket = google_storage_bucket.medios.name
  role   = "roles/storage.objectAdmin"
  member = google_service_account.sa["medios"].member
}
