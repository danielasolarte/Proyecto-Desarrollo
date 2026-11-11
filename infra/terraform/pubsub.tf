# Pub/Sub de la E4. Responsable: Mateo (Daniela dejó la base).
#
# El backend publica en mooc-media-jobs y responde sin esperar. Una sola
# suscripción push (un solo propósito: procesar medios) entrega cada mensaje
# al worker, autenticada con un token OIDC de sa-pubsub-push. Hasta 3
# instancias del worker se reparten los mensajes. Si mañana otro consumidor
# necesita los mismos eventos (por ejemplo notificaciones), tendrá su propia
# suscripción (clase 11).
#
# Contrato del mensaje: docs/entrega4/contrato-mensaje.md

resource "google_pubsub_topic" "trabajos" {
  name = var.topic_trabajos

  # Mensajes no confirmados se guardan hasta 1 día.
  message_retention_duration = "86400s"

  depends_on = [google_project_service.apis]
}

# Tópico de fallidos: un mensaje que se entregó 10 veces sin confirmarse pasa
# aquí, para no escalar un error repetible (clase 11). El worker además marca
# el job como dead_letter en Postgres al agotar sus intentos.
resource "google_pubsub_topic" "fallidos" {
  name = "${var.topic_trabajos}-dlq"

  message_retention_duration = "604800s"

  depends_on = [google_project_service.apis]
}

# Suscripción de pull sobre fallidos: guarda los mensajes para revisarlos con
#   gcloud pubsub subscriptions pull mooc-media-jobs-dlq-revision --limit=10
resource "google_pubsub_subscription" "fallidos_revision" {
  name  = "${var.topic_trabajos}-dlq-revision"
  topic = google_pubsub_topic.fallidos.id

  message_retention_duration = "604800s"
  ack_deadline_seconds       = 60
  expiration_policy {
    ttl = ""
  }
}

# Suscripción push al worker. Solo existe cuando el servicio del worker ya
# está creado (servicios_habilitados.worker = true).
resource "google_pubsub_subscription" "worker" {
  count = var.servicios_habilitados.worker ? 1 : 0

  name  = "${var.topic_trabajos}-worker"
  topic = google_pubsub_topic.trabajos.id

  # Igual al tiempo máximo de solicitud del worker (600 s): Pub/Sub no
  # reenvía un trabajo que sigue en curso.
  ack_deadline_seconds       = 600
  message_retention_duration = "86400s"

  push_config {
    push_endpoint = "${google_cloud_run_v2_service.worker[0].uri}/pubsub/push"

    oidc_token {
      service_account_email = google_service_account.sa["pubsub-push"].email
      audience              = google_cloud_run_v2_service.worker[0].uri
    }
  }

  # Reintento con espera creciente para fallos transitorios.
  retry_policy {
    minimum_backoff = "10s"
    maximum_backoff = "600s"
  }

  # 10 y no 5: Pub/Sub cuenta como entrega cada 429 que devuelve el worker
  # mientras otra instancia tiene el job (bloqueo de 60 s). Si una instancia
  # muere a mitad de un trabajo, las entregas siguientes reciben 429 hasta
  # que el bloqueo vence; con 5 el mensaje podía llegar al tópico de
  # fallidos antes de que otra instancia lo retomara. Los errores reales ya
  # los corta el worker a los 3 intentos (max_attempts del job en Postgres).
  dead_letter_policy {
    dead_letter_topic     = google_pubsub_topic.fallidos.id
    max_delivery_attempts = 10
  }

  expiration_policy {
    ttl = ""
  }

  depends_on = [
    google_pubsub_topic_iam_member.agente_publica_fallidos,
    google_cloud_run_v2_service_iam_member.push_invoca_worker,
  ]
}

# ---------- permisos ----------

# Publicar trabajos: el backend (cargas) y el worker (trabajos encadenados:
# escaneo -> HLS o PDF).
resource "google_pubsub_topic_iam_member" "publica_trabajos" {
  for_each = toset(["backend", "worker"])

  topic  = google_pubsub_topic.trabajos.id
  role   = "roles/pubsub.publisher"
  member = google_service_account.sa[each.key].member
}

# El agente de Pub/Sub mueve los mensajes fallidos al tópico de fallidos y
# debe poder confirmarlos en la suscripción de origen.
resource "google_pubsub_topic_iam_member" "agente_publica_fallidos" {
  topic  = google_pubsub_topic.fallidos.id
  role   = "roles/pubsub.publisher"
  member = "serviceAccount:${local.agente_pubsub}"
}

resource "google_pubsub_subscription_iam_member" "agente_confirma_worker" {
  count = var.servicios_habilitados.worker ? 1 : 0

  subscription = google_pubsub_subscription.worker[0].id
  role         = "roles/pubsub.subscriber"
  member       = "serviceAccount:${local.agente_pubsub}"
}

# El agente de Pub/Sub firma el token OIDC en nombre de sa-pubsub-push.
resource "google_service_account_iam_member" "agente_firma_push" {
  service_account_id = google_service_account.sa["pubsub-push"].name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:${local.agente_pubsub}"
}
