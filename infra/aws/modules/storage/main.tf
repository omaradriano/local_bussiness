# Archivos de un entorno: 3 buckets privados, CloudFront frente al de catálogo (OAC)
# y un usuario IAM de mínimo privilegio para la API. Nada aquí es público salvo vía CloudFront.

terraform {
  required_providers {
    aws = { source = "hashicorp/aws", version = "~> 6.0" }
  }
}

variable "env" { type = string }                # staging | production
variable "cors_origins" { type = list(string) } # orígenes web que suben directo a S3

locals {
  prefix = "local-bussiness-${var.env}"
  buckets = {
    catalog         = { versioned = false } # llaves inmutables (uuid por versión): no hace falta versionar
    private-docs    = { versioned = true }  # INE
    rental-evidence = { versioned = true }  # fotos de salida y devolución
  }
}

resource "aws_s3_bucket" "this" {
  for_each = local.buckets
  bucket   = "${local.prefix}-${each.key}"
}

resource "aws_s3_bucket_public_access_block" "this" {
  for_each                = aws_s3_bucket.this
  bucket                  = each.value.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_ownership_controls" "this" {
  for_each = aws_s3_bucket.this
  bucket   = each.value.id
  rule { object_ownership = "BucketOwnerEnforced" }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "this" {
  for_each = aws_s3_bucket.this
  bucket   = each.value.id
  rule {
    apply_server_side_encryption_by_default { sse_algorithm = "AES256" }
  }
}

resource "aws_s3_bucket_versioning" "this" {
  for_each = { for k, v in local.buckets : k => v if v.versioned }
  bucket   = aws_s3_bucket.this[each.key].id
  versioning_configuration { status = "Enabled" }
}

# El cliente sube directo con URL prefirmada (PUT) y lee archivos privados con URL prefirmada (GET).
resource "aws_s3_bucket_cors_configuration" "this" {
  for_each = aws_s3_bucket.this
  bucket   = each.value.id
  cors_rule {
    allowed_methods = ["PUT", "GET"]
    allowed_origins = var.cors_origins
    allowed_headers = ["*"]
    expose_headers  = ["ETag"]
    max_age_seconds = 3600
  }
}

# ---------------------------------------------------------------- CloudFront (solo catálogo)

resource "aws_cloudfront_origin_access_control" "catalog" {
  name                              = "${local.prefix}-catalog"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

data "aws_cloudfront_cache_policy" "optimized" {
  name = "Managed-CachingOptimized"
}

resource "aws_cloudfront_distribution" "catalog" {
  comment         = "${local.prefix} catálogo"
  enabled         = true
  is_ipv6_enabled = true
  price_class     = "PriceClass_100" # Norteamérica y Europa: suficiente para México y más barato

  origin {
    origin_id                = "catalog"
    domain_name              = aws_s3_bucket.this["catalog"].bucket_regional_domain_name
    origin_access_control_id = aws_cloudfront_origin_access_control.catalog.id
  }

  default_cache_behavior {
    target_origin_id       = "catalog"
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD"]
    cached_methods         = ["GET", "HEAD"]
    cache_policy_id        = data.aws_cloudfront_cache_policy.optimized.id
    compress               = true
  }

  restrictions {
    geo_restriction { restriction_type = "none" }
  }

  # Sin dominio propio todavía: certificado de *.cloudfront.net.
  viewer_certificate { cloudfront_default_certificate = true }
}

# Solo esta distribución puede leer el bucket de catálogo.
resource "aws_s3_bucket_policy" "catalog" {
  bucket = aws_s3_bucket.this["catalog"].id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid       = "CloudFrontReadOnly"
      Effect    = "Allow"
      Principal = { Service = "cloudfront.amazonaws.com" }
      Action    = "s3:GetObject"
      Resource  = "${aws_s3_bucket.this["catalog"].arn}/*"
      Condition = { StringEquals = { "AWS:SourceArn" = aws_cloudfront_distribution.catalog.arn } }
    }]
  })
  depends_on = [aws_s3_bucket_public_access_block.this]
}

# ---------------------------------------------------------------- IAM de la API

# La llave de acceso se crea fuera de OpenTofu (para que el secreto no quede en el estado)
# y se guarda solo en Railway.
resource "aws_iam_user" "api" {
  name = "${local.prefix}-api"
}

resource "aws_iam_user_policy" "api" {
  name = "${local.prefix}-api-objects"
  user = aws_iam_user.api.name
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["s3:PutObject", "s3:GetObject", "s3:DeleteObject"]
      Resource = [for b in aws_s3_bucket.this : "${b.arn}/*"]
    }]
  })
}

output "buckets" { value = { for k, b in aws_s3_bucket.this : k => b.id } }
output "catalog_cdn_url" { value = "https://${aws_cloudfront_distribution.catalog.domain_name}" }
output "api_user" { value = aws_iam_user.api.name }
