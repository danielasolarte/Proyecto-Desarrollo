# Job de migraciones (Daniela). Reemplaza a la VM mooc-servicios de la E3:
# corre golang-migrate una vez por despliegue, dentro de la VPC (Cloud SQL
# solo tiene IP privada) y con DATABASE_URL leída de Secret Manager.
# golang-migrate toma un bloqueo en Postgres, así que dos ejecuciones
# simultáneas no aplican la misma migración dos veces.
#
# Entrega 5: Terraform lo ejecuta solo. run_execution_token cambia cuando
# cambia la imagen del job (que trae las migraciones de ese commit) y
# entonces apply crea una ejecución y espera a que termine antes de
# actualizar el backend. Si la imagen no cambia, no se ejecuta nada.
# deploy/gcp/e4/migrar.ps1 sigue sirviendo para correrlo a mano
# (up o -Accion version).

resource "google_cloud_run_v2_job" "migraciones" {
  count = var.servicios_habilitados.migraciones ? 1 : 0

  name                = "mooc-migraciones"
  location            = var.region
  deletion_protection = false

  # 12 caracteres del digest de la imagen: "mooc-migraciones-" + token
  # queda por debajo del límite de 63.
  run_execution_token = substr(sha256(data.google_artifact_registry_docker_image.servicio["migraciones"].self_link), 0, 12)

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
          network    = google_compute_network.vpc.id
          subnetwork = google_compute_subnetwork.subred.id
        }
      }

      containers {
        image = data.google_artifact_registry_docker_image.servicio["migraciones"].self_link
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
              secret  = google_secret_manager_secret.app["DATABASE_URL"].secret_id
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
