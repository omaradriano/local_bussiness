terraform {
  required_version = ">= 1.10"
  required_providers {
    aws = { source = "hashicorp/aws", version = "~> 6.0" }
  }
  # Estado remoto cifrado y versionado; el bucket se creó una vez a mano (ver infra/aws/README.md).
  backend "s3" {
    bucket       = "local-bussiness-tfstate"
    key          = "staging/storage.tfstate"
    region       = "us-west-2"
    encrypt      = true
    use_lockfile = true
  }
}

provider "aws" {
  region = "us-west-2" # junto a Supabase y Railway
  default_tags {
    tags = { Project = "local-bussiness", Environment = "staging", ManagedBy = "opentofu" }
  }
}

module "storage" {
  source = "../modules/storage"
  env    = "staging"
  cors_origins = [
    "https://local-bussiness-staging.vercel.app",
    "https://local-bussiness-staging-*-omaradrianos-projects.vercel.app",
    "http://localhost:8082",
  ]
}

output "buckets" { value = module.storage.buckets }
output "catalog_cdn_url" { value = module.storage.catalog_cdn_url }
output "api_user" { value = module.storage.api_user }
