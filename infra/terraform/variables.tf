# Valores por defecto = proyecto del curso. Para otro proyecto, copia
# terraform.tfvars.example a terraform.tfvars (ignorado por git) y ajústalo.

variable "project_id" {
  description = "Proyecto de GCP."
  type        = string
  default     = "proyecto1-entrega2-desarrollo"
}

variable "region" {
  description = "Región de Cloud Run, Artifact Registry y la subred."
  type        = string
  default     = "us-central1"
}

variable "image_tag" {
  description = "Etiqueta de las imágenes a desplegar: el commit corto que publicó deploy/gcp/e4/construir-imagenes.ps1. Terraform la convierte en el digest de la imagen (data.google_artifact_registry_docker_image), así que se despliega exactamente la imagen probada."
  type        = string
}

variable "servicios_habilitados" {
  description = "Qué servicios de Cloud Run crea Terraform. Cada responsable cambia el suyo a true cuando su imagen ya está en Artifact Registry."
  type = object({
    frontend = bool
    backend  = bool
    worker   = bool
  })
  default = {
    frontend = false
    backend  = true
    worker   = false
  }
}

# ---------- recursos que ya existen (E2/E3); se leen, no se crean ----------

variable "red" {
  description = "VPC de la E2. Cloud Run sale por ella (salida directa a la VPC) para llegar a Cloud SQL por IP privada."
  type        = string
  default     = "mooc-e2-vpc"
}

variable "subred" {
  description = "Subred de la E2 en la región; de aquí toman IP las instancias de Cloud Run."
  type        = string
  default     = "mooc-e2-subnet"
}

variable "instancia_sql" {
  description = "Instancia de Cloud SQL (Postgres, solo IP privada)."
  type        = string
  default     = "mooc-postgres"
}

variable "bucket_medios" {
  description = "Bucket de medios de la E2/E3 (originales, HLS, PDF)."
  type        = string
  default     = "mooc-e2-media-proyecto1-entrega2"
}

variable "repositorio_imagenes" {
  description = "Repositorio Docker de Artifact Registry."
  type        = string
  default     = "mooc-e2"
}

variable "secretos" {
  description = "Secretos que ya existen en Secret Manager (los creó la E3), por variable de entorno de la aplicación. Terraform solo da permiso de lectura; nunca lee ni guarda sus valores."
  type        = map(string)
  default = {
    DATABASE_URL  = "mooc-database-url"
    S3_ACCESS_KEY = "mooc-s3-access-key"
    S3_SECRET_KEY = "mooc-s3-secret-key"
  }
}

# ---------- nombres de la E4 ----------

variable "topic_trabajos" {
  description = "Topic de Pub/Sub de los trabajos multimedia."
  type        = string
  default     = "mooc-media-jobs"
}

variable "image_tag_por_servicio" {
  description = "Etiqueta propia para algún servicio (frontend, backend, worker, migraciones) cuando no se publicó con el mismo commit que image_tag. Ejemplo: { worker = \"a1b2c3d\" }."
  type        = map(string)
  default     = {}
}
