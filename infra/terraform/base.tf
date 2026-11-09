# Base de la E4, creada por Terraform en el proyecto de Camila: red,
# Cloud SQL, bucket de medios, Artifact Registry y contenedores de secretos.
#
# En la E2/E3 todo esto existía en el proyecto anterior; ese proyecto quedó
# sin facturación (se agotó el crédito educativo), así que la E4 lo crea de
# nuevo aquí y queda todo en código.

data "google_project" "actual" {
  project_id = var.project_id
}

locals {
  # Nombre fijo (no la referencia al recurso) para que backend y worker no
  # dependan del bucket: el CORS del bucket usa la URL del frontend, y el
  # frontend la del backend.
  nombre_bucket_medios = "mooc-e4-medios-${var.project_id}"

  registro = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.imagenes.repository_id}"

  # Cuenta administrada por Google que usa Pub/Sub para firmar los tokens de
  # push y para mover mensajes al tópico de fallidos.
  agente_pubsub = "service-${data.google_project.actual.number}@gcp-sa-pubsub.iam.gserviceaccount.com"
}

# ---------- red ----------

resource "google_compute_network" "vpc" {
  name                    = "mooc-e4-vpc"
  auto_create_subnetworks = false

  depends_on = [google_project_service.apis]
}

# De aquí toman IP las instancias de Cloud Run (salida directa a la VPC).
# Una /24 alcanza de sobra para 3 + 3 instancias y el job.
resource "google_compute_subnetwork" "subred" {
  name                     = "mooc-e4-subnet"
  region                   = var.region
  network                  = google_compute_network.vpc.id
  ip_cidr_range            = "10.30.0.0/24"
  private_ip_google_access = true
}

# Rango reservado para los servicios administrados (Cloud SQL con IP privada).
resource "google_compute_global_address" "servicios_privados" {
  name          = "mooc-e4-servicios-privados"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 20
  network       = google_compute_network.vpc.id
}

resource "google_service_networking_connection" "servicios_privados" {
  network                 = google_compute_network.vpc.id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.servicios_privados.name]
}

# ---------- Cloud SQL ----------

# Postgres 16 solo con IP privada: nadie llega desde internet. El usuario y
# la clave NO se crean aquí (quedarían en el estado): los crea
# deploy/gcp/e4/01-secretos.ps1 y guarda DATABASE_URL en Secret Manager.
resource "google_sql_database_instance" "postgres" {
  name             = "mooc-e4-postgres"
  region           = var.region
  database_version = "POSTGRES_16"

  # Protege los datos de un destroy por error. Para borrar la instancia al
  # final del semestre, pon false, apply y luego destroy.
  deletion_protection = true

  settings {
    tier              = var.nivel_sql
    edition           = "ENTERPRISE"
    availability_type = "ZONAL"
    disk_type         = "PD_SSD"
    disk_size         = 10
    disk_autoresize   = true

    ip_configuration {
      ipv4_enabled    = false
      private_network = google_compute_network.vpc.id
      ssl_mode        = "ALLOW_UNENCRYPTED_AND_ENCRYPTED"
    }

    # Recuperación (extensión opcional del plan): copia diaria y
    # recuperación a un punto en el tiempo.
    backup_configuration {
      enabled                        = true
      point_in_time_recovery_enabled = true
      start_time                     = "07:00"
      transaction_log_retention_days = 3
      backup_retention_settings {
        retained_backups = 7
      }
    }

    database_flags {
      name  = "max_connections"
      value = "50"
    }
  }

  # Encender y apagar la instancia (--activation-policy) se hace con gcloud
  # para ahorrar créditos; Terraform no lo vigila, así que no aparece como
  # cambio pendiente ni la enciende en el siguiente apply.
  lifecycle {
    ignore_changes = [settings[0].activation_policy]
  }

  depends_on = [google_service_networking_connection.servicios_privados]
}

resource "google_sql_database" "mooc" {
  name     = "mooc"
  instance = google_sql_database_instance.postgres.name
}

# ---------- Cloud Storage ----------

resource "google_storage_bucket" "medios" {
  name     = local.nombre_bucket_medios
  location = var.region

  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"

  # Borrado recuperable: un original borrado se puede recuperar 7 días.
  soft_delete_policy {
    retention_duration_seconds = 604800
  }

  # El navegador sube y descarga directo con URL firmadas, así que el bucket
  # debe aceptar el origen del frontend.
  cors {
    origin = concat(
      var.servicios_habilitados.frontend ? [google_cloud_run_v2_service.frontend[0].uri] : [],
      var.origenes_cors_extra,
    )
    method          = ["GET", "HEAD", "PUT", "POST"]
    response_header = ["Content-Type", "ETag", "Content-Range", "Range"]
    max_age_seconds = 3600
  }

  depends_on = [google_project_service.apis]
}

# ---------- Artifact Registry ----------

resource "google_artifact_registry_repository" "imagenes" {
  location      = var.region
  repository_id = "mooc-e4"
  format        = "DOCKER"
  description   = "Imágenes de la E4 (backend, worker, frontend, migraciones)"

  # Conserva las 10 versiones más recientes de cada imagen.
  cleanup_policy_dry_run = false
  cleanup_policies {
    id     = "conservar-recientes"
    action = "KEEP"
    most_recent_versions {
      keep_count = 10
    }
  }
  cleanup_policies {
    id     = "borrar-viejas"
    action = "DELETE"
    condition {
      older_than = "2592000s"
    }
  }

  depends_on = [google_project_service.apis]
}

# ---------- acceso del grupo ----------

locals {
  # Grupo: ver, publicar imágenes y encender o apagar Cloud SQL
  # (roles/cloudsql.editor no permite borrar la instancia, ni cambiar
  # usuarios, claves o permisos).
  roles_equipo = {
    for par in setproduct(var.equipo, ["roles/viewer", "roles/artifactregistry.writer", "roles/logging.viewer", "roles/monitoring.viewer", "roles/cloudsql.editor"]) :
    "${par[0]}|${par[1]}" => { correo = par[0], rol = par[1] }
  }

  # Equipo docente: solo lectura (proyecto, registros y métricas).
  roles_docentes = {
    for par in setproduct(var.docentes, ["roles/viewer", "roles/logging.viewer", "roles/monitoring.viewer"]) :
    "${par[0]}|${par[1]}" => { correo = par[0], rol = par[1] }
  }
}

resource "google_project_iam_member" "equipo" {
  for_each = local.roles_equipo

  project = var.project_id
  role    = each.value.rol
  member  = "user:${each.value.correo}"
}

resource "google_project_iam_member" "docentes" {
  for_each = local.roles_docentes

  project = var.project_id
  role    = each.value.rol
  member  = "user:${each.value.correo}"
}
