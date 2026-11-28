# Entrega 5: Cloud Function de validación de cargas (base: Daniela; código
# de la función: Mateo; aplicar y evidencias: Camila).
#
# Flujo (docs/entrega5/analisis-serverless.md):
#   backend --validate_upload--> topic mooc-media-validate
#     --Eventarc--> función mooc-validar-carga (checksum, MIME, firma ZIP)
#     --scan_antivirus--> topic mooc-media-jobs --push--> worker (E4)
#
# Todo lo de la función depende de var.funcion_validar. El topic, las
# cuentas de servicio y el bucket de fuentes existen siempre: son baratos y
# así encender la función es solo cambiar la variable a true.
#
# Contrato del mensaje y reglas de idempotencia:
# docs/entrega5/contrato-validacion.md

locals {
  nombre_funcion = "mooc-validar-carga"
  dir_funcion    = "${path.module}/../../functions/validar-carga"
}

# ---------- topic que dispara la función ----------

resource "google_pubsub_topic" "validacion" {
  name = var.topic_validacion

  message_retention_duration = "86400s"

  depends_on = [google_project_service.apis]
}

# Solo el backend publica validate_upload.
resource "google_pubsub_topic_iam_member" "backend_publica_validacion" {
  topic  = google_pubsub_topic.validacion.id
  role   = "roles/pubsub.publisher"
  member = google_service_account.sa["backend"].member
}

# La función publica el siguiente paso (scan_antivirus) en el topic del
# worker, con la misma clave de idempotencia que usa el backend.
resource "google_pubsub_topic_iam_member" "funcion_publica_trabajos" {
  topic  = google_pubsub_topic.trabajos.id
  role   = "roles/pubsub.publisher"
  member = google_service_account.sa["funcion-validar"].member
}

# ---------- permisos de la función ----------

# Lee el original y lo borra si no pasa la validación. objectUser no
# permite cambiar permisos del bucket ni de los objetos.
resource "google_storage_bucket_iam_member" "funcion_objetos" {
  bucket = google_storage_bucket.medios.name
  role   = "roles/storage.objectUser"
  member = google_service_account.sa["funcion-validar"].member
}

# El disparador invoca la función con un token OIDC de sa-disparo-validar;
# el agente de Pub/Sub firma ese token.
resource "google_service_account_iam_member" "agente_firma_disparo" {
  service_account_id = google_service_account.sa["disparo-validar"].name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:${local.agente_pubsub}"
}

resource "google_project_iam_member" "disparo_recibe_eventos" {
  project = var.project_id
  role    = "roles/eventarc.eventReceiver"
  member  = google_service_account.sa["disparo-validar"].member
}

resource "google_project_iam_member" "disparo_service_usage" {
  project = var.project_id
  role    = "roles/serviceusage.serviceUsageConsumer"
  member  = google_service_account.sa["disparo-validar"].member
}

resource "google_storage_bucket_iam_member" "build_lee_fuentes_gcf" {
  bucket = "gcf-v2-sources-828976161294-us-central1"
  role   = "roles/storage.objectViewer"
  member = google_service_account.sa["build-funcion"].member
}

# ---------- compilación (Cloud Build) ----------

# Cloud Build compila con sa-build-funcion, no con la cuenta por defecto de
# Compute Engine: solo escribe registros, lee el zip de fuentes y publica la
# imagen en el mismo Artifact Registry de la E4.
resource "google_project_iam_member" "build_registros" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = google_service_account.sa["build-funcion"].member
}

resource "google_artifact_registry_repository_iam_member" "build_publica_imagen" {
  location   = google_artifact_registry_repository.imagenes.location
  repository = google_artifact_registry_repository.imagenes.name
  role       = "roles/artifactregistry.writer"
  member     = google_service_account.sa["build-funcion"].member
}

resource "google_storage_bucket_iam_member" "build_lee_fuentes" {
  bucket = google_storage_bucket.fuentes.name
  role   = "roles/storage.objectViewer"
  member = google_service_account.sa["build-funcion"].member
}

# Quien hace apply le pide a Cloud Functions que compile como
# sa-build-funcion, así que necesita actuar como esa cuenta (y como la de
# ejecución). Lo tiene quien es dueño del proyecto (Camila).

# ---------- código de la función ----------

# Bucket privado para el zip del código. Los zips viejos se borran a los 30
# días; el nombre del objeto lleva el hash del contenido, así que cambiar el
# código crea un objeto nuevo y apply actualiza la función.
resource "google_storage_bucket" "fuentes" {
  name     = "mooc-e5-fuentes-${var.project_id}"
  location = var.region

  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = true

  lifecycle_rule {
    condition {
      age = 30
    }
    action {
      type = "Delete"
    }
  }

  depends_on = [google_project_service.apis]
}

# El zip se arma con lo que hay en functions/validar-carga (sin pruebas ni
# archivos locales). No lleva secretos: DATABASE_URL llega desde Secret
# Manager en tiempo de ejecución.
data "archive_file" "funcion_validar" {
  count = var.funcion_validar ? 1 : 0

  type        = "zip"
  source_dir  = local.dir_funcion
  output_path = "${path.module}/.build/validar-carga.zip"
  excludes    = ["*_test.go", ".build", "README.md", ".env", "*.zip"]
}

resource "google_storage_bucket_object" "funcion_validar" {
  count = var.funcion_validar ? 1 : 0

  name   = "validar-carga-${data.archive_file.funcion_validar[0].output_md5}.zip"
  bucket = google_storage_bucket.fuentes.name
  source = data.archive_file.funcion_validar[0].output_path
}

# ---------- la función ----------

resource "google_cloudfunctions2_function" "validar" {
  count = var.funcion_validar ? 1 : 0

  name        = local.nombre_funcion
  location    = var.region
  description = "Valida cada carga (checksum, MIME real, firma ZIP) y encola el antivirus. Entrega 5."

  build_config {
    runtime         = "go125"
    entry_point     = "ValidarCarga"
    service_account = google_service_account.sa["build-funcion"].id

    # Misma carpeta de imágenes que la E4 (y su política de limpieza).
    docker_repository = google_artifact_registry_repository.imagenes.id

    source {
      storage_source {
        bucket = google_storage_bucket.fuentes.name
        object = google_storage_bucket_object.funcion_validar[0].name
      }
    }
  }

  service_config {
    service_account_email = google_service_account.sa["funcion-validar"].email

    available_memory                 = var.funcion_validar_config.memoria
    available_cpu                    = var.funcion_validar_config.cpu
    timeout_seconds                  = var.funcion_validar_config.timeout_s
    max_instance_count               = var.funcion_validar_config.max_instancias
    min_instance_count               = var.funcion_validar_config.min_instancias
    max_instance_request_concurrency = var.funcion_validar_config.concurrencia

    # Solo la invoca Eventarc; no tiene URL pública.
    ingress_settings               = "ALLOW_INTERNAL_ONLY"
    all_traffic_on_latest_revision = true

    # Salida directa a la VPC, como backend y worker: Cloud SQL solo tiene
    # IP privada. Storage y Pub/Sub siguen por internet de Google.
    direct_vpc_egress = "VPC_EGRESS_PRIVATE_RANGES_ONLY"
    direct_vpc_network_interface {
      network    = google_compute_network.vpc.id
      subnetwork = google_compute_subnetwork.subred.id
    }

    environment_variables = {
      S3_BUCKET      = local.nombre_bucket_medios
      PUBSUB_PROJECT = var.project_id
      PUBSUB_TOPIC   = google_pubsub_topic.trabajos.name
      # 3 instancias x 2 = 6 conexiones más, dentro de las 50 de Cloud SQL.
      DB_MAX_CONNS = tostring(var.funcion_validar_config.conexiones_bd)
    }

    secret_environment_variables {
      key        = "DATABASE_URL"
      project_id = var.project_id
      secret     = google_secret_manager_secret.app["DATABASE_URL"].secret_id
      version    = "latest"
    }
  }

  event_trigger {
    trigger_region        = var.region
    event_type            = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic          = google_pubsub_topic.validacion.id
    service_account_email = google_service_account.sa["disparo-validar"].email

    # Si la función devuelve error, Eventarc reintenta con espera creciente.
    # No hay duplicados: la función toma el job con el mismo UPDATE atómico
    # del worker y corta a los 3 intentos (max_attempts en Postgres).
    retry_policy = "RETRY_POLICY_RETRY"
  }

  depends_on = [
    google_project_service.apis,
    google_secret_manager_secret_iam_member.lectura,
    google_project_iam_member.build_registros,
    google_artifact_registry_repository_iam_member.build_publica_imagen,
    google_storage_bucket_iam_member.build_lee_fuentes,
    google_service_account_iam_member.agente_firma_disparo,
  ]
}

# El disparador solo puede invocar esta función (su servicio de Cloud Run).
resource "google_cloud_run_v2_service_iam_member" "disparo_invoca_funcion" {
  count = var.funcion_validar ? 1 : 0

  name     = google_cloudfunctions2_function.validar[0].service_config[0].service
  location = var.region
  role     = "roles/run.invoker"
  member   = google_service_account.sa["disparo-validar"].member
}
