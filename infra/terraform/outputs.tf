output "url_frontend" {
  description = "URL pública de la aplicación (la que se entrega)."
  value       = var.servicios_habilitados.frontend ? google_cloud_run_v2_service.frontend[0].uri : null
}

output "url_backend" {
  description = "URL del backend (también responde detrás del frontend en /api)."
  value       = var.servicios_habilitados.backend ? google_cloud_run_v2_service.backend[0].uri : null
}

output "url_worker" {
  description = "URL del worker. Sin identidad responde 403."
  value       = var.servicios_habilitados.worker ? google_cloud_run_v2_service.worker[0].uri : null
}

output "topic_trabajos" {
  value = google_pubsub_topic.trabajos.id
}

output "suscripcion_worker" {
  value = var.servicios_habilitados.worker ? google_pubsub_subscription.worker[0].id : null
}

output "job_migraciones" {
  value = google_cloud_run_v2_job.migraciones.name
}

output "cuentas_de_servicio" {
  description = "Cuenta de servicio de cada responsabilidad (tabla de permisos de la entrega)."
  value       = { for k, sa in google_service_account.sa : k => sa.email }
}

output "imagenes_desplegadas" {
  description = "Imagen exacta (con digest) de cada servicio."
  value = merge(
    { for s, img in data.google_artifact_registry_docker_image.servicio : s => img.self_link },
    { migraciones = data.google_artifact_registry_docker_image.migraciones.self_link },
  )
}
