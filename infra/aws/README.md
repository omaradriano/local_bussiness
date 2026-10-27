# infra/aws

Archivos del proyecto en AWS con OpenTofu. Todo lleva el prefijo `local-bussiness-<entorno>-` y la etiqueta `Project=local-bussiness`; no se administra nada más de la cuenta.

| Recurso | Para qué |
| --- | --- |
| `local-bussiness-<env>-catalog` | Fotos del catálogo. Privado; se lee solo por CloudFront (OAC) |
| `local-bussiness-<env>-private-docs` | INE. Privado y versionado; lectura con URL prefirmada |
| `local-bussiness-<env>-rental-evidence` | Fotos de salida y devolución. Privado y versionado |
| CloudFront | CDN del catálogo (`*.cloudfront.net` hasta tener dominio) |
| `local-bussiness-<env>-api` (IAM) | Usuario de la API: solo `Put/Get/DeleteObject` en esos buckets |

## Uso

```sh
aws login --profile local-bussiness --region us-west-2   # sesión de consola
cd infra/aws/staging
AWS_PROFILE=local-bussiness tofu init
AWS_PROFILE=local-bussiness tofu plan
AWS_PROFILE=local-bussiness tofu apply   # requiere aprobación del dueño
```

## Una sola vez: bucket del estado

```sh
aws s3api create-bucket --bucket local-bussiness-tfstate --region us-west-2 \
  --create-bucket-configuration LocationConstraint=us-west-2
aws s3api put-bucket-versioning --bucket local-bussiness-tfstate --versioning-configuration Status=Enabled
aws s3api put-public-access-block --bucket local-bussiness-tfstate \
  --public-access-block-configuration BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true
```

## Llave de la API

No se crea con OpenTofu para que el secreto no quede en el estado:

```sh
aws iam create-access-key --user-name local-bussiness-staging-api --profile local-bussiness
```

Las dos partes van solo a variables de Railway (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`).
