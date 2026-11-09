variable "project_id" {
  description = "Proyecto de GCP de la E4 (el de Camila). Lo pasa deploy/gcp/e4/tf.ps1 desde deploy/gcp/e4/proyecto.ps1."
  type        = string
}

variable "region" {
  description = "Región de todo: Cloud Run, Cloud SQL, bucket, Artifact Registry y subred."
  type        = string
  default     = "us-central1"
}

variable "image_tag" {
  description = "Etiqueta de las imágenes a desplegar: el commit corto que publicó deploy/gcp/e4/construir-imagenes.ps1. Terraform la convierte en el digest de la imagen, así que se despliega exactamente la imagen probada. Solo hace falta cuando hay algún servicio habilitado."
  type        = string
  default     = "sin-imagenes"
}

variable "image_tag_por_servicio" {
  description = "Etiqueta propia para algún servicio (frontend, backend, worker, migraciones) cuando no se publicó con el mismo commit que image_tag. Ejemplo: { worker = \"a1b2c3d\" }."
  type        = map(string)
  default     = {}
}

variable "servicios_habilitados" {
  description = "Qué servicios de Cloud Run crea Terraform. Todo en false = solo la base (red, Cloud SQL, bucket, Artifact Registry, Pub/Sub, identidades). Se cambia a true cuando la imagen de ese servicio ya está en Artifact Registry."
  type = object({
    migraciones = bool
    backend     = bool
    frontend    = bool
    worker      = bool
  })
  default = {
    migraciones = true
    backend     = true
    frontend    = false
    worker      = false
  }
}

variable "equipo" {
  description = "Correos del grupo (y del equipo docente si se quiere) que pueden ver el proyecto y publicar imágenes. No modifican infraestructura: eso lo hace Terraform."
  type        = list(string)
  default     = []
}

# ---------- nombres ----------

variable "topic_trabajos" {
  description = "Topic de Pub/Sub de los trabajos multimedia."
  type        = string
  default     = "mooc-media-jobs"
}

variable "nivel_sql" {
  description = "Tipo de máquina de Cloud SQL. db-g1-small admite hasta 50 conexiones: alcanza para 3 backends x 5 + 3 workers x 3 + migraciones."
  type        = string
  default     = "db-g1-small"
}

variable "correo_admin" {
  description = "Correo de la primera cuenta de administrador. El backend la crea al arrancar si la base no tiene ningún administrador; la clave está en el secreto mooc-admin-password."
  type        = string
  default     = "admin@mooc.local"
}

variable "origenes_cors_extra" {
  description = "Orígenes adicionales permitidos para subir al bucket con URL firmadas (por ejemplo el frontend local)."
  type        = list(string)
  default     = ["http://localhost:5173"]
}
