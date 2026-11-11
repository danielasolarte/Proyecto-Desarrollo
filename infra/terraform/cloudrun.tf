# Servicios de Cloud Run de la E4. Base y backend: Daniela. Cada responsable
# completa el bloque de su servicio y cambia su valor en
# servicios_habilitados (variables.tf) a true cuando su imagen ya está en
# Artifact Registry:
#   frontend -> Samara   worker -> Mateo
#
# Configuración documentada (docs/entrega4/arquitectura.md):
#
#   servicio   mín  máx  CPU  memoria  concurrencia  timeout  acceso
#   frontend    0    3    1   512 MiB       80          60 s  público
#   backend     0    3    1   1 GiB         40          60 s  público
#   worker      0    3    2   4 GiB          1         600 s  solo sa-pubsub-push
#
# Backend, worker y migraciones salen a la VPC por salida directa (solo rangos privados)
# para llegar a Cloud SQL por IP privada; el resto (Pub/Sub, Storage) sale
# directo a internet de Google.

locals {
  imagen = {
    for s in ["frontend", "backend", "worker", "migraciones"] :
    s => "mooc-e4-${s}:${lookup(var.image_tag_por_servicio, s, var.image_tag)}"
  }

  # Variables no secretas comunes a backend y worker (las mismas de la E3).
  entorno_almacenamiento = {
    S3_ENDPOINT = "storage.googleapis.com"
    S3_BUCKET   = local.nombre_bucket_medios
    S3_USE_SSL  = "true"
    S3_REGION   = var.region
  }

  entorno_pubsub = {
    PUBSUB_PROJECT = var.project_id
    PUBSUB_TOPIC   = google_pubsub_topic.trabajos.name
  }
}

# El digest de cada imagen: se despliega exactamente la imagen publicada con
# esa etiqueta, y un cambio de imagen crea una revisión nueva.
data "google_artifact_registry_docker_image" "servicio" {
  for_each = {
    for s, habilitado in var.servicios_habilitados : s => local.imagen[s] if habilitado
  }

  location      = var.region
  repository_id = google_artifact_registry_repository.imagenes.repository_id
  image_name    = each.value
}


# =====================================================================
# Backend (Daniela)
# =====================================================================
resource "google_cloud_run_v2_service" "backend" {
  count = var.servicios_habilitados.backend ? 1 : 0

  name                = "mooc-backend"
  location            = var.region
  ingress             = "INGRESS_TRAFFIC_ALL"
  deletion_protection = false

  template {
    service_account                  = google_service_account.sa["backend"].email
    timeout                          = "60s"
    max_instance_request_concurrency = 40

    scaling {
      min_instance_count = 0
      max_instance_count = 3
    }

    vpc_access {
      egress = "PRIVATE_RANGES_ONLY"
      network_interfaces {
        network    = google_compute_network.vpc.id
        subnetwork = google_compute_subnetwork.subred.id
      }
    }

    containers {
      image = data.google_artifact_registry_docker_image.servicio["backend"].self_link

      ports {
        container_port = 8080
      }

      resources {
        limits = {
          cpu    = "1"
          memory = "1Gi"
        }
        cpu_idle          = true
        startup_cpu_boost = true
      }

      dynamic "env" {
        for_each = merge(local.entorno_almacenamiento, local.entorno_pubsub, {
          AUTH_COOKIE_ENABLED            = "true"
          CSRF_ENABLED                   = "true"
          SESSION_COOKIE_SECURE          = "true"
          SESSION_COOKIE_SAMESITE        = "Lax"
          SESSION_COOKIE_NAME            = "mooc_session"
          CSRF_COOKIE_NAME               = "mooc_csrf"
          CSRF_HEADER_NAME               = "X-CSRF-Token"
          SESSION_COOKIE_MAX_AGE_SECONDS = "86400"
          BOOTSTRAP_ADMIN_EMAIL          = var.correo_admin
          # 3 instancias x 5 conexiones = 15, dentro del límite de Cloud SQL.
          DB_MAX_CONNS      = "5"
          DB_MIN_CONNS      = "0"
          HEALTH_TIMEOUT_MS = "1500"
          # Cloud Run da 10 s entre SIGTERM y SIGKILL.
          SHUTDOWN_DRAIN_SECONDS   = "0"
          SHUTDOWN_TIMEOUT_SECONDS = "8"
        })
        content {
          name  = env.key
          value = env.value
        }
      }

      # Secretos: Cloud Run los inyecta como variables de entorno desde
      # Secret Manager; no pasan por el repositorio ni por el estado.
      dynamic "env" {
        for_each = { for k, v in local.secretos : k => v if contains(local.lectores_secretos[k], "backend") }
        content {
          name = env.key
          value_source {
            secret_key_ref {
              secret  = google_secret_manager_secret.app[env.key].secret_id
              version = "latest"
            }
          }
        }
      }

      startup_probe {
        http_get {
          path = "/health/ready"
        }
        period_seconds    = 5
        timeout_seconds   = 3
        failure_threshold = 12
      }
    }
  }

  depends_on = [
    google_project_service.apis,
    google_secret_manager_secret_iam_member.lectura,
  ]
}

# =====================================================================
# Frontend (Samara)
# nginx sirve la SPA y reenvía /api/* al backend (mismo origen: sin CORS y
# las cookies siguen funcionando). BACKEND_URL es la URL del backend; nginx
# debe enviar Host del backend y usar SNI (proxy_ssl_server_name on).
# =====================================================================
resource "google_cloud_run_v2_service" "frontend" {
  count = var.servicios_habilitados.frontend ? 1 : 0

  name                = "mooc-frontend"
  location            = var.region
  ingress             = "INGRESS_TRAFFIC_ALL"
  deletion_protection = false

  template {
    service_account                  = google_service_account.sa["frontend"].email
    timeout                          = "60s"
    max_instance_request_concurrency = 80

    scaling {
      min_instance_count = 0
      max_instance_count = 3
    }

    containers {
      image = data.google_artifact_registry_docker_image.servicio["frontend"].self_link

      ports {
        container_port = 8080
      }

      resources {
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }
        cpu_idle = true
      }

      env {
        name  = "BACKEND_URL"
        value = var.servicios_habilitados.backend ? google_cloud_run_v2_service.backend[0].uri : ""
      }
    }
  }

  depends_on = [google_project_service.apis]
}

# =====================================================================
# Worker (Mateo)
# Recibe POST /pubsub/push. Escucha en $PORT (Cloud Run lo define).
# Responde 204 solo después de guardar el resultado; ver reglas de
# respuesta en docs/entrega4/contrato-mensaje.md.
# =====================================================================
resource "google_cloud_run_v2_service" "worker" {
  count = var.servicios_habilitados.worker ? 1 : 0

  name                = "mooc-worker"
  location            = var.region
  ingress             = "INGRESS_TRAFFIC_ALL" # protegido por IAM: sin allUsers, cualquier llamada sin identidad recibe 403
  deletion_protection = false

  template {
    service_account                  = google_service_account.sa["worker"].email
    timeout                          = "600s"
    max_instance_request_concurrency = 1

    scaling {
      min_instance_count = 0
      max_instance_count = 3
    }

    vpc_access {
      egress = "PRIVATE_RANGES_ONLY"
      network_interfaces {
        network    = google_compute_network.vpc.id
        subnetwork = google_compute_subnetwork.subred.id
      }
    }

    containers {
      image = data.google_artifact_registry_docker_image.servicio["worker"].self_link

      ports {
        container_port = 8080
      }

      resources {
        limits = {
          cpu    = "2"
          memory = "4Gi"
        }
        cpu_idle          = true
        startup_cpu_boost = true
      }

      dynamic "env" {
        for_each = merge(local.entorno_almacenamiento, local.entorno_pubsub, {
          WORKER_WORKDIR          = "/tmp/media"
          WORKER_LOCK_TTL_SECONDS = "60"
          CLAMAV_HOST             = "127.0.0.1"
          CLAMAV_PORT             = "3310"
          DB_MAX_CONNS            = "3"
          DB_MIN_CONNS            = "0"
        })
        content {
          name  = env.key
          value = env.value
        }
      }

      dynamic "env" {
        for_each = { for k, v in local.secretos : k => v if contains(local.lectores_secretos[k], "worker") }
        content {
          name = env.key
          value_source {
            secret_key_ref {
              secret  = google_secret_manager_secret.app[env.key].secret_id
              version = "latest"
            }
          }
        }
      }
    }
  }

  depends_on = [
    google_project_service.apis,
    google_secret_manager_secret_iam_member.lectura,
  ]
}

# ---------- quién puede invocar cada servicio ----------

# Frontend y backend son públicos (la aplicación hace su propia
# autenticación de personas con la sesión).
resource "google_cloud_run_v2_service_iam_member" "publico_frontend" {
  count = var.servicios_habilitados.frontend ? 1 : 0

  name     = google_cloud_run_v2_service.frontend[0].name
  location = var.region
  role     = "roles/run.invoker"
  member   = "allUsers"
}

resource "google_cloud_run_v2_service_iam_member" "publico_backend" {
  count = var.servicios_habilitados.backend ? 1 : 0

  name     = google_cloud_run_v2_service.backend[0].name
  location = var.region
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# El worker solo lo invoca la identidad del push de Pub/Sub.
resource "google_cloud_run_v2_service_iam_member" "push_invoca_worker" {
  count = var.servicios_habilitados.worker ? 1 : 0

  name     = google_cloud_run_v2_service.worker[0].name
  location = var.region
  role     = "roles/run.invoker"
  member   = google_service_account.sa["pubsub-push"].member
}
