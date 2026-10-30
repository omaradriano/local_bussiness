# Manual del superadministrador

Guía para operar **Khronno** como dueño de la plataforma: qué controla cada pantalla de Superadministrador, cómo se manejan los planes y los giros, cómo dar de alta un negocio piloto y qué falta antes de producción.

> El nombre es **Khronno** (dominio khronno.com, constantes `APP_NAME`/`APP_DOMAIN` en `packages/shared/src/strings.ts`); el ícono de tienda sigue siendo provisional. Se cambian en `apps/client/app.json`, `apps/client/public/manifest.json`, `apps/client/public/index.html` y `apps/client/assets/icon.png`.

---

## 1. Entornos

| Entorno | Web | API | Base de datos |
| --- | --- | --- | --- |
| Staging (pruebas) | https://local-bussiness-staging.vercel.app | https://api-staging-4e97e.up.railway.app | Supabase `local-bussiness-staging` |
| Producción | **Por crear**: guía paso a paso en `docs/produccion.md` | — | — |

- Tu `pnpm dev` local usa **la misma base de staging**: lo que crees en local aparece en staging.
- Cada merge a `main` despliega solo a staging (migraciones → Railway → Vercel).
- El pooler de Supabase admite **15 conexiones** entre todos (API y worker de Railway, tu `pnpm dev` y las migraciones). Si un despliegue falla con `EMAXCONNSESSION`, espera un minuto y vuelve a correrlo (`gh run rerun <id> --failed`).

## 2. Cómo ser superadministrador

El superadministrador es una fila en `public.platform_admins`. Se agrega con SQL en el editor de Supabase:

```sql
insert into public.platform_admins (user_id)
select id from auth.users where email = 'tu-correo@ejemplo.com';
```

Con eso, en **Tus negocios** aparece la entrada **Superadministrador**.

## 3. Pantalla Superadministrador

### 3.1 Giros

Cada giro (Barbería, Florería, Uñas y belleza, Renta de herramienta) tiene un estado:

| Estado | Quién puede registrar un negocio nuevo de ese giro |
| --- | --- |
| **Público** | Cualquiera, al registrarse |
| **Solo superadmin** | Solo tú. En el alta aparece como «En prueba». Sirve para probar un giro, incluso en producción. |
| **Apagado** | Nadie |

- Los negocios que **ya existen** siguen funcionando en cualquier estado; el estado solo controla los nuevos.
- Hoy: Barbería y Renta en **Público**; Florería y Uñas en **Solo superadmin**, porque todavía no tienen su flujo propio.
- Un giro nuevo que se agregue al código arranca en «Solo superadmin».

### 3.2 Negocios

Lista de todos los negocios con su giro, sus funciones activas y el plan de su dueño. Al entrar a uno:

**Plan de la cuenta** (ver §4). Aplica a **todos** los negocios de ese dueño.

**Funciones del negocio.** Cada función tiene tres opciones:
- **Como el giro**: lo que trae su giro (y su plan).
- **Dar**: se la das aunque el giro o el plan no la traigan. Una cortesía tuya gana sobre el plan.
- **Quitar**: se la quitas aunque el giro la traiga.

Después, el dueño enciende o apaga las disponibles en **Ajustes → Funciones**.

Funciones existentes: equipo, inventario, aval del cliente, dirección obligatoria, documentos del cliente, clientes de confianza, clientes casuales, servicio a domicilio, iniciar servicio, documentos adicionales (pagaré), venta de productos.

### 3.3 Borrar un negocio

Al final de la ficha del negocio, en **Zona de peligro → Eliminar negocio**. Hay que escribir el nombre del negocio para confirmar. El dueño puede hacer lo mismo desde **Ajustes**, aunque su cuenta esté vencida.

- **Base de datos:** se borra **todo** el negocio de una vez (citas, rentas, pedidos, clientes y sus documentos, catálogo, equipo, productos, finanzas, ajustes).
- **Archivos en S3:** un trabajo del worker borra la carpeta del negocio (`{id}/`) en los tres buckets (fotos del catálogo, INE y documentos, evidencias de renta). En la bitácora de Railway del worker queda «purged business files».
- **El plan de la cuenta del dueño se conserva:** si abre otro negocio, no vuelve a tener prueba.
- **No se puede deshacer.**

**Pendiente:** los buckets de INE (`private-docs`) y de evidencias (`rental-evidence`) tienen versionado. Al borrar queda una marca y la versión anterior se conserva. Falta una regla de ciclo de vida que elimine las versiones anteriores, p. ej. a los 30 días (cambio en `infra/aws/`, requiere aprobación). Ver §10.

## 4. Planes

### 4.1 Qué incluye cada plan

Definido en `services/api/internal/plans/plans.json`. La API lo hace cumplir y la app oculta lo que no aplica. Para cambiar un número basta editar ese archivo y desplegar.

| | Lite | Pro | Business |
| --- | --- | --- | --- |
| Negocios por cuenta | 1 | 1 | 5 |
| Usuarios con acceso | 1 | 5 | Sin límite |
| Personas que atienden | 1 | 10 | Sin límite |
| Artículos en catálogo | 30 | 200 | Sin límite |
| Unidades en renta | 15 | 100 | Sin límite |
| Fotos por artículo | 3 | 8 | 8 |
| Equipo, venta de productos, documentos adicionales | — | ✓ | ✓ |
| Sueldos y comisiones | — | ✓ | ✓ |
| Gastos recurrentes y compras a plazos | — | ✓ | ✓ |
| Resumen de finanzas | Por mes | Cualquier periodo | Cualquier periodo |

### 4.2 Ciclo de una cuenta

1. **Prueba:** al abrir su primer negocio, el dueño recibe **14 días de Pro**. Un segundo negocio no reinicia la prueba.
2. **Vencida:** al terminar la prueba (o la fecha de un plan), la cuenta queda **de solo lectura**. Puede ver todo, pero no registrar nada nuevo ni abrir otro negocio. No se borra nada.
3. **Plan asignado:** tú le asignas Lite, Pro o Business desde **Superadministrador → negocio → Plan de la cuenta**.

No hay plan gratis permanente (decisión tomada).

### 4.3 Asignar un plan

En **Plan de la cuenta**:
- **Plan:** Prueba, Lite, Pro o Business.
- **Último día** (AAAA-MM-DD): hasta cuándo dura. Vacío = **sin vencimiento**, para pilotos y cortesías.
- **Prueba sin fecha:** reinicia 14 días desde hoy.
- **Nota:** para ti (p. ej. «Piloto – precio fundador»).

Recomendaciones:
- **Pilotos:** Pro sin vencimiento durante el piloto (2–3 meses); después, precio de fundador.
- **Cobro manual mientras no hay pagos en línea:** asigna el plan con último día = fin del periodo pagado y anótalo en la nota.
- Los dueños que ya existían antes de los planes quedaron en **Pro sin vencimiento**.

### 4.4 Al bajar de plan

- Lo que excede el nuevo límite **se conserva** (artículos, personas, unidades); solo se bloquea agregar más.
- Desactivar artículos o personas, o retirar unidades, libera lugar.
- Las funciones que el plan no incluye se apagan, salvo las que tú hayas dado a mano.
- Al llegar a un límite, el dueño ve un mensaje como: «Tu plan Lite permite hasta 30 artículos en el catálogo.»

### 4.5 Qué ve el dueño

- **Etiqueta de tiempo:** en la barra lateral, en el encabezado del teléfono y en «Tus negocios»: «Prueba · 12 días», «Vence en 5 días» (planes con fecha, en la última semana) o «Solo lectura».
- **Inicio del negocio:** un aviso con los días de prueba o con el estado de solo lectura.

### 4.6 Personas del equipo

En **Equipo → persona** el dueño puede:
- **Desactivar:** deja de recibir citas, pero sigue en la lista.
- **Eliminar persona:** sale de la lista, de la agenda y de Sueldos; sus citas y comisiones pasadas conservan su nombre.

La persona ligada al dueño («Yo») no se puede eliminar. Eliminar personas libera lugar en el límite del plan.

## 5. Alta de un negocio piloto (paso a paso)

1. El dueño crea su cuenta en la web (correo y contraseña) y acepta el aviso de privacidad.
2. Registra su negocio: nombre, dirección del catálogo y giro. Arranca con datos de ejemplo (catálogo, horario).
3. Tú, en Superadministrador → su negocio → **Plan de la cuenta**: **Pro**, sin último día, nota «Piloto».
4. Revisa sus **funciones** (§3.2) según lo que pidió.
5. Con él:
   - **Ajustes:** anticipo, margen entre citas, rentas, **Privacidad** (nombre o razón social, domicilio y correo para su aviso).
   - **Catálogo, horario y equipo:** los suyos.
   - **Instalar la app:** menú → «Instalar app». En iPhone: Safari → Compartir → «Agregar a inicio».
6. Explícale lo de la ventana de bienvenida: es un proyecto en desarrollo y sus comentarios cuentan (oadriandev@gmail.com).

## 6. Privacidad

- **Aviso de la plataforma:** `/privacidad` (dueños y equipo). Los datos del responsable están en `packages/shared/src/privacy.ts` → `PLATFORM_RESPONSIBLE`. **Siguen siendo marcadores** (ver §10).
- **Aviso de cada negocio:** `/<slug>/privacidad`, armado con Ajustes → Privacidad. Sin datos, usa el nombre y el WhatsApp del negocio.
- **Aceptación:**
  - Dueños: casilla obligatoria al crear cuenta (fecha y versión en sus metadatos de Supabase).
  - Clientes: casilla «Aceptó el aviso de privacidad» en su ficha.
- **Si cambias el texto de un aviso**, sube la versión en `services/api/internal/privacy/privacy.json`. Así se pide de nuevo la aceptación.
- Basado en la ley de 2025 (la autoridad ahora es la Secretaría Anticorrupción y Buen Gobierno). **Debe revisarlo un abogado** antes de cobrar.

## 7. Lo que el dueño tiene hoy

- **Día a día:** Hoy, Calendario, Citas/Rentas/Pedidos, Clientes (documentos, aval), botón **+ Nuevo** (cita o venta de producto).
- **Rentas:** salida y devolución con pasos, documentos adicionales, extender, inventario por unidad, rentas vencidas.
- **Barbería:** equipo, cliente casual, servicio a domicilio, margen entre citas, «elige quién atiende».
- **Productos:** venta en mostrador, inventario con costo, estadísticas.
- **Finanzas** (solo el dueño):
  - Resumen con ganancia, de dónde entró el dinero, en qué se gastó, rentas vencidas y reservas por estado.
  - Gastos de una vez, **recurrentes** (cada cobro queda «por confirmar» con el monto real o «No se pagó») y **a plazos**.
  - Sueldos y comisiones.
- **Ajustes:** políticas por giro, funciones, documentos, privacidad.
- **PWA:** instalable y abre sin conexión (los datos sí necesitan red).

### 7.1 Exportar e importar (Ajustes, solo el dueño)

- **Exportar a CSV:**
  - Qué: clientes, reservas (Citas, Rentas o Pedidos), gastos, ventas de productos, catálogo y productos.
  - Periodo: reservas, gastos y ventas usan el elegido.
  - El archivo se abre bien en Excel (acentos y columnas) y funciona también con la cuenta vencida.
- **Importar desde CSV:**
  - Qué: clientes (se reconocen por teléfono), catálogo y productos (por nombre).
  - Primero se ve una vista previa (nuevos, se actualizan, filas con error y por qué) y después se confirma.
  - Se aplican las mismas reglas que en las pantallas: p. ej. en renta, el cliente necesita dirección y aval.
  - El catálogo respeta el límite del plan.
  - La existencia de un producto que ya existe no se sobreescribe.
  - Hay **plantillas** descargables, y un archivo exportado se puede volver a importar.
- **Hoy solo en la versión web (PWA).**

### 7.2 Cuenta del usuario

- **¿Olvidaste tu contraseña?** en la pantalla de entrar: llega un enlace a `/reset` para poner una nueva. El correo sale con la plantilla y el remitente por defecto de Supabase, en inglés y con límite bajo de envíos, hasta configurar Resend.
- **Términos y condiciones** en `/terminos` (borrador, `packages/shared/src/terms.ts`): al crear cuenta se aceptan junto con el aviso de privacidad y se guarda la versión de los dos.

## 8. Operación técnica

- **Flujo:** rama → PR → CI verde (lint, typecheck, tests, pruebas contra base, build de Docker, preview de Vercel) → merge → despliegue a staging.
- **Esquema:** solo con migraciones nuevas en `supabase/migrations`; nunca se edita una aplicada.
- **Logs:** `railway logs --service api` / `--service worker`; Vercel con `vercel logs`.
- **Si Railway falla antes de construir** (sin logs): el workflow reintenta una vez; si sigue, `gh run rerun <id> --failed`.
- **Procesos automáticos (worker):** variantes de fotos, apartados sin anticipo que expiran, rentas que pasan a vencidas y borrado de archivos de negocios eliminados.
- **Monitoreo de errores (Sentry):** ya está en el código de la API, el worker y la app, pero **apagado** hasta configurar los DSN. Para encenderlo:
  1. Crea en sentry.io dos proyectos: Go para la API y React Native para la app.
  2. En Railway, en `api` y `worker`: `SENTRY_DSN` y `APP_ENV`.
  3. En Vercel: `EXPO_PUBLIC_SENTRY_DSN` y `EXPO_PUBLIC_APP_ENV`.
  No se envían datos personales.

## 9. Datos de contacto y textos

- Contacto en la ventana de bienvenida: `PLATFORM_CONTACT` en `packages/shared/src/privacy.ts` (oadriandev@gmail.com).
- Todos los textos visibles están en `packages/shared/src/strings.ts` y en los mapas por giro (`packages/shared/src/verticals/`).

## 10. Pendientes

### 10.1 Para ir a producción con pilotos (solo el panel del dueño)

Bloqueantes:
- [ ] **Entorno de producción:**
  - Supabase (proyecto nuevo; se recomienda el plan de pago por respaldos diarios);
  - Railway (entorno `production` para `api` y `worker`);
  - Vercel (proyecto de producción);
  - AWS (buckets y CloudFront de producción con OpenTofu en `infra/aws/`);
  - flujo `deploy-production.yml` con aprobación manual.
  - *Requiere tus inicios de sesión y aprobaciones.*
- [ ] **Correo de Supabase con SMTP propio (Resend)**: el correo por defecto tiene un límite muy bajo de envíos por hora y puede caer en spam, lo que afecta confirmaciones y recuperación de contraseña.
- [ ] **Datos del responsable en el aviso de la plataforma** (nombre o razón social y domicilio).
- [ ] **Revisión legal** de los términos y del aviso de privacidad, con los datos del responsable.

Recomendados antes del primer piloto:
- [ ] **Encender el monitoreo** con los DSN de Sentry (el código ya está).
- [ ] **Dominio propio** (por ahora se comparte el link de Vercel).
- [ ] **Respaldos diarios** (por ahora no; decisión tomada).
- [ ] **Probar la PWA instalada** en un iPhone y un Android reales.
- [ ] **Nombre e ícono definitivos.**

### 10.2 Para cerrar la Fase 5

| Criterio | Estado |
| --- | --- |
| La PWA se instala en Android y iOS desde el navegador | Construida y probada en Chrome de escritorio; falta en teléfonos reales |
| Builds internos en TestFlight y Google Play | No iniciado: requiere cuentas de Apple Developer y Google Play Console |
| Tres negocios piloto (uno por giro) operan una semana sin ayuda | No iniciado: depende de producción y de conseguir pilotos (hoy solo hay flujo completo para renta y barbería) |

### 10.3 Pendientes de fases anteriores

- [ ] **Fase 4 – Avisos:**
  - Ajustes → Avisos;
  - recordatorio al dueño con el mensaje de WhatsApp listo;
  - notificaciones push;
  - aviso de renta vencida.
  - Hoy solo existen los procesos automáticos de vencimiento y apartados.
- [ ] **Invitar usuarios** al negocio (empleados con su propio acceso): hoy solo entra el dueño, así que el límite de «usuarios con acceso» todavía no aplica.
- [ ] **Flujos propios de florería y uñas** antes de ponerlos en Público.
- [ ] **Borrado automático de INE** N días después de cerrar la renta (job `purge_id_documents` del plan).
- [ ] **Borrar la cuenta del usuario desde la app** (obligatorio para publicar en la App Store). Borrar negocios ya existe; falta borrar la cuenta en sí.
- [ ] **Regla de ciclo de vida en S3** para que las versiones anteriores de INE y evidencias se borren (p. ej. 30 días) después de borrar un negocio o un documento.

### 10.4 Después (fases siguientes)

- Catálogo público y reservas en línea (Fase 6), con la casilla de privacidad para el cliente.
- Cobro de suscripciones en línea y, opcionalmente, comisión por pagos en línea.
- Builds de tiendas con EAS (íconos de Android incluidos).
