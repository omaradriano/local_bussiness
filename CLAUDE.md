# CLAUDE.md

Plataforma multi-negocio de reservas, pedidos y rentas (uñas, florería, renta de herramienta). El plan completo está en `docs/plan.md`; es la fuente de verdad del alcance, el modelo de datos y las fases.

## Stack

- `apps/client`: Expo + Expo Router (iOS, Android, web: catálogo público `/<slug>` y PWA `/admin`).
- `packages/shared` (zod, enums, terminología por giro, fechas), `packages/api-client` (generado de `openapi.yaml`), `packages/ui`.
- `services/api`: Go (chi, pgx, sqlc); binarios `cmd/api` y `cmd/worker`.
- `supabase/`: única fuente de verdad del esquema (migraciones, RLS, seed). Sin Supabase local ni Docker: el desarrollo usa el proyecto en la nube `local-bussiness-staging` (ref `upsolqbrrpvhiusnlsfh`), vinculado con `supabase link`.
- Hosting: Railway (api, worker), Vercel (web), Supabase, AWS S3 + CloudFront.
  - Railway: proyecto `local-bussiness`, entorno `staging`, servicios `api` y `worker` (misma imagen de `services/api`, us-west2). Su configuración vive en `.railway/railway.ts` (Infrastructure as Code; `railway.json` está obsoleto desde 2026-12-01): `railway config plan` para ver cambios, `railway config apply` para aplicarlos. El código se sube con `railway up services/api --path-as-root --service <api|worker>`.
  - API staging: https://api-staging-4e97e.up.railway.app
  - AWS (cuenta personal, perfil `local-bussiness`, sesión con `aws login --profile local-bussiness --region us-west-2`): buckets `local-bussiness-staging-{catalog,private-docs,rental-evidence}` y CloudFront https://d1tklylecl3ocy.cloudfront.net (catálogo), usuario IAM `local-bussiness-staging-api`. Código en `infra/aws/` (OpenTofu, estado en el bucket `local-bussiness-tfstate`). La cuenta tiene recursos de otros proyectos (`loukys-*`, `fastmarketcuu-*`): nunca tocarlos.
  - Vercel: proyecto `local-bussiness-staging` ligado a `apps/client` (`vercel.json` ahí). Despliegue solo desde GitHub Actions (deploy por Git desactivado).

## Comandos

- `pnpm dev`: API en `localhost:8090`, worker y cliente Expo en `localhost:8082` (puertos fijos para no chocar con otros proyectos en 8080/8081). API y worker se recompilan solos al guardar (air, `go tool air`)
- `pnpm test`, `pnpm typecheck`, `pnpm lint` (incluyen Go vía `services/api/package.json` → `make`)
- `pnpm gen`: tipos de Supabase + cliente OpenAPI
- `make -C services/api test`

## Instrucciones

1. Trabaja una fase a la vez, en orden, y dentro de cada fase por partes pequeñas. Al terminar cada parte, detente, entrega cómo verla y sus tests, y espera confirmación antes de seguir. Al cerrar la fase, entrega la tabla criterio → test → resultado.
2. Antes de cada fase, propón el plan de archivos y espera aprobación si cambia algo de `docs/plan.md`.
3. El esquema solo cambia con nuevas migraciones en `supabase/migrations`; nunca edites una migración ya aplicada.
4. Después de cambiar el esquema o `openapi.yaml`, corre `pnpm gen` y confirma que typecheck pasa.
5. Toda regla de negocio (disponibilidad, estados, pagos) vive en Go y se escribe empezando por sus tests unitarios; el cliente solo valida formato, y también lleva tests donde haya lógica.
6. Montos en centavos (`bigint`, MXN), fechas en UTC (`timestamptz`) con la zona del negocio; nada de `float` para dinero ni fechas locales sin zona.
7. Textos visibles en español de México, centralizados en `packages/shared` para poder adaptarlos por giro.
8. Nada de diseño visual antes de la fase 6: componentes base funcionales y accesibles. En la fase 6 se instalan y usan las skills de `emilkowalski/skills`.
9. Nunca subas secretos; usa `.env.example`, variables de Railway y Vercel, y secretos de GitHub. Nunca imprimas, copies ni edites `.env.local`.
10. Si algo del plan es ambiguo o contradictorio, pregunta antes de construir.
11. Commits y PRs sin ninguna atribución a Claude (nada de `Co-Authored-By: Claude` ni "Generated with Claude Code"). Mensajes en Conventional Commits (`feat:`, `fix:`, `test:`, `chore:`).
12. Cada vez que un servicio requiera autenticación (Vercel, Railway, Supabase, AWS, Expo…), detente y avisa qué comando correr; después verifica con un comando de solo lectura (`whoami`, `status`, `ls`) y reporta.

## CLIs y aprobaciones

| CLI | Libre | Requiere aprobación |
| --- | --- | --- |
| `gh` | ramas, PRs, leer checks y logs | tags de release, merge a `main` |
| `railway` | `status`, `logs`, leer variables, `up` a staging | cambiar variables, cualquier acción en production |
| `vercel` | `ls`, `inspect`, `logs`, deploy de preview | `env add/rm`, `--prod` |
| `supabase` | `migration new`, `migration list`, `gen types --linked` | `db push` y `db reset` (staging), cualquier acción en production |
| `aws` | `s3 ls`, revisar políticas y CORS en staging, `plan` de `infra/aws/` | crear o cambiar buckets o IAM, borrar objetos, production |
| `eas` | `build` de prueba, `update` a preview | `submit`, `update` a production |

`.claude/settings.json` hace cumplir lo bloqueado (`deny`) y lo que pide confirmación (`ask`).

## Receta: tipo de paso nuevo (workflow steps)

1. **Go:** implementar la interfaz `StepType` (`Key`, `ValidateConfig`, `ValidateData`, `OnComplete`) en `internal/steps/` y registrarla en `registry.go`.
2. **Esquemas:** definir en `packages/shared/steps/<tipo>.ts` el esquema de `config` y de `data`; de ahí se exporta JSON Schema que también usa Go.
3. **Cliente:** registrar en `stepRenderers` el componente de captura y el de configuración (si no, se genera un formulario desde el JSON Schema).
4. **Tests** del tipo (config válida e inválida, datos completos e incompletos) y activarlo solo para el negocio que lo pidió con `settings.features`.

Reglas: la máquina de estados vive solo en `internal/bookings/machine.go`; ningún flujo revisa un tipo de paso por nombre, todo pasa por el registro; las plantillas por giro viven en `packages/shared/templates/`.
