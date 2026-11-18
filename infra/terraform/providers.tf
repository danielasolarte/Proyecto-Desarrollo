terraform {
  required_version = ">= 1.6.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 7.42"
    }
    # E5: arma el zip del código de la Cloud Function.
    archive = {
      source  = "hashicorp/archive"
      version = "~> 2.7"
    }
  }
}

# La autenticación es la de gcloud de quien ejecuta Terraform:
#   gcloud auth application-default login
provider "google" {
  project = var.project_id
  region  = var.region

  default_labels = {
    app      = "mooc"
    entrega  = "e4"
    gestiona = "terraform"
  }
}
