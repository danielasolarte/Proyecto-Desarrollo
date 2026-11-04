# Job de migraciones (Daniela). Reemplaza a la VM mooc-servicios de la E3:
# corre golang-migrate una vez por despliegue, dentro de la VPC (Cloud SQL
# solo tiene IP privada) y con DATABASE_URL leída de Secret Manager.
# golang-migrate toma un bloqueo en Postgres, así que dos ejecuciones
# simultáneas no aplican la misma migración dos veces.
#
# Se ejecuta con deploy/gcp/e4/migrar.ps1 (up por defecto, o -Accion version).

resource "google_cloud_run_v2_job" "migraciones" {
  name                = "mooc-migraciones"
  location            = var.region
  deletion_protection = false

  template {
    task_count  = 1
    parallelism = 1

    template {
      service_account = google_service_account.sa["migraciones"].email
      timeout         = "600s"
      max_retries     = 0

      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = data.google_compute_network.vpc.id
          subnetwork = data.google_compute_subnetwork.subred.id
        }
      }

      containers {
        image = data.google_artifact_registry_docker_image.migraciones.self_link
        args  = ["up"]

        resources {
          limits = {
            cpu    = "1"
            memory = "512Mi"
          }
        }

        env {
          name = "DATABASE_URL"
          value_source {
            secret_key_ref {
              secret  = data.google_secret_manager_secret.app["DATABASE_URL"].secret_id
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
