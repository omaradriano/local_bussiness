terraform {
  required_version = ">= 1.10"
  required_providers {
    aws = { source = "hashicorp/aws", version = "~> 6.0" }
  }
  # Mismo bucket de estado que staging, otra llave.
  backend "s3" {
    bucket       = "local-bussiness-tfstate"
    key          = "production/storage.tfstate"
    region       = "us-west-2"
    encrypt      = true
    use_lockfile = true
  }
}

provider "aws" {
  region = "us-west-2" # junto a Supabase y Railway
  default_tags {
    tags = { Project = "local-bussiness", Environment = "production", ManagedBy = "opentofu" }
  }
}

# URL de la web de producción en Vercel (se conoce al crear el proyecto; luego, el dominio propio).
variable "web_origin" {
  type        = string
  description = "Web de producción"
  default     = "https://khronno.com"
}

module "storage" {
  source       = "../modules/storage"
  env          = "production"
  cors_origins = [var.web_origin] # sin localhost ni previews en producción
}

output "buckets" { value = module.storage.buckets }
output "catalog_cdn_url" { value = module.storage.catalog_cdn_url }
output "api_user" { value = module.storage.api_user }
