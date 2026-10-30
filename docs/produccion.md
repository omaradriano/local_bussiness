# Despliegue a producción

Producción es un entorno aparte de staging: su propio proyecto de Supabase, su entorno en Railway, su proyecto en Vercel y sus buckets en AWS. **Ningún dato de staging pasa a producción** y el seed (negocios demo) no se carga.

Leyenda: **[Tú]** requiere tu inicio de sesión, una cuenta o tu aprobación. **[Claude]** lo hago yo, ya con acceso y aprobación.

---

## 0. Antes de empezar

- [ ] **Correo con Resend [Tú + Claude]: necesario antes del primer piloto.** El correo por defecto de Supabase solo envía a direcciones del equipo del proyecto y con un límite muy bajo por hora; un piloto no podría confirmar su cuenta ni recuperar su contraseña.
  - Crea la cuenta en resend.com (plan gratis) y un dominio o remitente verificado. Mientras no haya dominio propio se puede usar el de prueba de Resend con límites.
  - En Supabase → Authentication → SMTP: host `smtp.resend.com`, puerto 465, usuario `resend` y la API key de Resend como contraseña.
- [ ] Decide el nombre del proyecto en Vercel; de él sale la URL. Abajo se asume `local-bussiness` → `https://local-bussiness.vercel.app`.

## 1. Supabase

1. **[Tú]** En el dashboard de Supabase, crea el proyecto `local-bussiness-production`:
   - región **West US (Oregon)**, igual que staging;
   - una contraseña de base de datos fuerte, guardada en tu gestor de contraseñas.
2. **[Tú]** Pásame el **Project ref**: no es secreto, aparece en la URL del dashboard.
3. **[Claude]** Descomento `[remotes.production]` en `supabase/config.toml` con ese ref y la URL de Vercel. Con tu aprobación, aplico la configuración de Auth (URL del sitio y redirecciones de `/reset`) con `supabase config push`.
4. **[Tú]** De Project Settings → API copia la **URL** y la **publishable key**, que van a Vercel (§3). De Database → Connection string (pooler, modo sesión) copia la cadena; va a Railway (§4) como `DATABASE_URL`.

Las migraciones **no** se corren a mano: las aplica el flujo de producción (§6).

## 2. AWS (archivos)

1. **[Tú]** `aws login --profile local-bussiness --region us-west-2`.
2. **[Claude]** Plan sin cambios:
   ```sh
   cd infra/aws/production
   AWS_PROFILE=local-bussiness tofu init
   AWS_PROFILE=local-bussiness tofu plan -var web_origin=https://local-bussiness.vercel.app
   ```
3. **[Tú aprueba] [Claude]** `tofu apply` con la misma variable. Crea:
   - los buckets `local-bussiness-production-{catalog,private-docs,rental-evidence}`;
   - el CloudFront del catálogo;
   - el usuario IAM `local-bussiness-production-api`.

   Los buckets privados quedan con la regla que borra versiones anteriores a los 30 días.
4. **[Tú aprueba] [Claude]** Llave de acceso para ese usuario IAM (`aws iam create-access-key`). **[Tú]** La guardas directo en Railway (§4); nunca pasa por el repo ni por el chat.
5. **[Claude]** Pongo la URL del CloudFront en `.railway/railway.ts` (`production.CATALOG_CDN_URL`).

> El mismo `tofu apply` en `infra/aws/staging` aplica la regla de versiones también a staging (pendiente de tu aprobación).

## 3. Vercel (web)

1. **[Tú]** Inicia sesión (`vercel login`) si hace falta.
2. **[Claude]** Creo el proyecto `local-bussiness` con raíz `apps/client`. `vercel.json` ya desactiva el despliegue por Git: solo se despliega desde GitHub Actions.
3. **[Tú aprueba] [Claude]** Variables del entorno *Production* (`vercel env add`):
   | Variable | Valor |
   | --- | --- |
   | `EXPO_PUBLIC_SUPABASE_URL` | URL del proyecto de producción |
   | `EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY` | publishable key de producción |
   | `EXPO_PUBLIC_API_URL` | URL pública de la API de producción (§4) |
   | `EXPO_PUBLIC_APP_ENV` | `production` |
   | `EXPO_PUBLIC_SENTRY_DSN` | opcional, proyecto *app* de Sentry |
4. **[Claude]** Anoto el `VERCEL_PROJECT_ID` (para GitHub, §5).

## 4. Railway (API y worker)

1. **[Tú aprueba] [Claude]** `railway environment new production` en el proyecto `local-bussiness`.
2. **[Claude]** Lleno `production` en `.railway/railway.ts`:
   - `SUPABASE_URL`;
   - `CORS_ORIGINS` (la URL de Vercel);
   - `CATALOG_CDN_URL`.
3. **[Tú aprueba] [Claude]**
   ```sh
   LB_ENV=production railway config plan --environment production
   LB_ENV=production railway config apply --environment production
   ```
4. **[Tú]** En el dashboard de Railway, entorno *production*, servicio `api`, pon los secretos (el `worker` los toma por referencia):
   - `DATABASE_URL`: la cadena del pooler de Supabase de producción;
   - `AWS_ACCESS_KEY_ID` y `AWS_SECRET_ACCESS_KEY`: la llave del §2;
   - `SENTRY_DSN`: opcional, proyecto *api* de Sentry.
5. **[Tú aprueba] [Claude]** Dominio público del servicio `api` (`railway domain`): esa es la `API_URL`.
6. **[Tú]** Crea un **project token** del entorno *production* (Project Settings → Tokens): va a GitHub como `RAILWAY_TOKEN`.

## 5. GitHub (flujo de despliegue)

**[Tú]** En el repo → Settings → Environments → **New environment `production`**:
- **Required reviewers:** tú. Así cada despliegue espera tu aprobación.
- **Deployment branches:** solo `main`.
- **Secrets:**
  - `SUPABASE_ACCESS_TOKEN`: puede ser el mismo de staging;
  - `SUPABASE_DB_PASSWORD`: la de producción;
  - `RAILWAY_TOKEN`: el token del entorno production;
  - `VERCEL_TOKEN`: puede ser el mismo;
  - `VERCEL_ORG_ID`;
  - `VERCEL_PROJECT_ID`: el del proyecto de producción.
- **Variables:** `SUPABASE_PROJECT_REF` (el ref de producción) y `API_URL` (la del §4).

Con `gh` puedo dejar listos los nombres; los valores secretos los pones tú.

## 6. Primer despliegue

1. **[Tú]** Actions → **deploy-production** → *Run workflow* (rama `main`) → **Approve**.
2. El flujo corre:
   1. migraciones en la base de producción, sin seed;
   2. API y worker en Railway, con revisión de `/healthz`;
   3. web en Vercel.
3. **[Claude]** Verifico con lectura:
   - `/healthz`;
   - que carguen `/login`, `/terminos` y `/privacidad`;
   - que no haya errores en los logs de Railway.

Los siguientes despliegues son iguales: cuando staging esté probado, *Run workflow* y aprobar.

## 7. Después del primer despliegue

1. **[Tú]** Crea tu cuenta en la web de producción (confirma el correo; requiere Resend, §0).
2. **[Tú]** Hazte superadministrador en el SQL editor de Supabase de producción:
   ```sql
   insert into public.platform_admins (user_id)
   select id from auth.users where email = 'tu-correo@ejemplo.com';
   ```
3. **[Tú]** En Superadministrador revisa los giros: la migración deja Renta y Barbería en Público, y Uñas y Florería en Solo superadmin.
4. **[Tú]** Comparte la URL de Vercel con los pilotos. A cada uno, en Superadministrador → Plan de la cuenta: **Pro sin vencimiento** con nota «Piloto».

## 8. Pendientes que no bloquean

- Datos del responsable en el aviso de privacidad y en los términos, y revisión legal.
- Dominio propio (luego: Vercel, CORS en Railway y en `infra/aws/production`, URLs de Auth en Supabase).
- Sentry (DSN en Railway y Vercel).
- Respaldos diarios (plan de pago de Supabase), cuando haya datos que lo justifiquen.
- Fase 4 (avisos y recordatorios), builds de tiendas con EAS.
