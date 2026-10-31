# Plan MVP: Plataforma de reservas, pedidos y rentas

Oct 24, 2026 · @Omar

## Visión y alcance del MVP

Una sola plataforma multi-negocio donde un local pequeño publica su catálogo, recibe reservas o pedidos con fecha, cobra anticipos y recuerda a sus clientes, sustituyendo la libreta y el WhatsApp desordenado. El MVP cubre tres giros de Chihuahua que validan los tres tipos de catálogo:

| Giro | Tipo de artículo | Qué se aparta | Lo crítico |
| --- | --- | --- | --- |
| Puesta de uñas | Servicio | Un horario con una empleada | Duración, empleada, no encimar citas |
| Florería bajo pedido | Producto bajo pedido | Un arreglo para una fecha de entrega | Cupo por día, personalización, entrega o recoger |
| Renta de herramienta | Renta | Unidades físicas por un rango de fechas | Disponibilidad por unidad, depósito, salida y devolución |

El diseño queda abierto para sumar después limosinas, salones de eventos y casas por noche, que son el mismo caso de "renta de un bien único".

**Usuarios**

- **Dueño y empleados:** usan la app móvil (iOS/Android) o la PWA de administración.
- **Clientes finales:** usan el catálogo web público del negocio, sin instalar nada y sin crear cuenta (se identifican con nombre y teléfono).

**Entra en el MVP**

- Registro del negocio y elección de giro, con plantilla de catálogo y terminología propia.
- Catálogo con fotos y precios, público en `dominio.com/<slug-del-negocio>`.
- Reservas, pedidos y rentas con verificación de disponibilidad en el servidor.
- Registro manual de pagos: anticipo, saldo y depósito (efectivo, transferencia, terminal).
- Recordatorios: enlace de WhatsApp generado (`wa.me`) que el dueño envía con un toque, y notificaciones push al dueño.
- Agenda del día y de la semana, lista de pedidos y de rentas activas.
- Reporte básico: ingresos del mes, reservas por estado, rentas vencidas.

**Fuera del MVP (fase 2 o posterior)**

- Pagos en línea del cliente final (Mercado Pago / Stripe).
- WhatsApp Business Cloud API con envío automático.
- Cobro de la suscripción del SaaS dentro de la app (al inicio se cobra por fuera).
- Facturación CFDI, inventario de insumos, nómina, multi-sucursal.
- Renta de casas a largo plazo (contratos y cobro mensual).

## Arquitectura y stack

Un solo código TypeScript con Expo genera la app iOS/Android y la web (catálogo público + PWA de administración); Supabase guarda datos y sesiones; AWS S3 guarda los archivos; el servidor Go concentra la lógica que no debe vivir en el cliente. El diagrama de componentes va al final de esta sección.

| Pieza | Tecnología | Responsabilidad |
| --- | --- | --- |
| App móvil | Expo (React Native) + Expo Router, TypeScript | Administración para dueño y empleados; push notifications |
| Web y PWA | Mismo proyecto Expo con salida web (react-native-web) | Rutas públicas `/<slug>` para clientes; rutas `/admin` como PWA instalable |
| API | Go 1.23+, chi, pgx, sqlc, en contenedor Docker | Disponibilidad, creación de reservas sin choques, cambios de estado, pagos, enlaces de WhatsApp, URLs prefirmadas de S3, endpoints públicos del catálogo |
| Worker | Mismo binario Go con otro comando | Jobs programados: recordatorios, rentas vencidas, expiración de apartados sin anticipo, purga de INE |
| Base de datos y auth | Supabase (Postgres, Auth) | Datos multi-negocio con RLS y login de dueños y empleados |
| Archivos | AWS S3 + CloudFront como CDN, AWS SDK for Go v2, libvips | Fotos del catálogo, INE y evidencias de renta; subida directa con URL prefirmada |
| Hosting de la API | Railway | Servicios `api` y `worker` desde la misma imagen Docker, entornos staging y production |
| Hosting web | Vercel | Export web estático de Expo (catálogo público + PWA admin), CDN, dominio propio y preview por cada PR |
| CI/CD | GitHub Actions | Checks en cada PR y despliegues a staging y production (ver CI/CD y despliegue) |

**Reglas de división de trabajo**

- **Lecturas simples del panel** (catálogo, clientes, listados) pueden ir directo a Supabase desde el cliente, protegidas por RLS.
- **Toda escritura con reglas de negocio** pasa por la API Go: crear o modificar reservas, cambiar estados, registrar pagos, salida y devolución de rentas.
- **El cliente final nunca toca Supabase directamente:** el catálogo público y la creación de pedidos van por endpoints públicos de la API con límite de peticiones.
- **La API valida el JWT de Supabase** con el JWKS del proyecto y resuelve el negocio y rol del usuario; usa la service role key solo en servidor.
- **Contrato único:** la API se describe con OpenAPI; de ahí se generan los tipos de Go (oapi-codegen) y el cliente TypeScript (openapi-typescript). Los tipos de tablas salen de `supabase gen types`.
- **Fechas:** se guardan en UTC (`timestamptz`) y cada negocio tiene su zona horaria (por defecto `America/Chihuahua`); la disponibilidad se calcula en la zona del negocio.

**Por qué Expo para la web:** un solo proyecto cubre las tres plataformas y comparte pantallas, lógica y validaciones. El riesgo es el SEO del catálogo público; se mitiga con la salida web estática de Expo Router y metadatos por negocio. Si el SEO llega a importar mucho, el catálogo público puede migrar a Next.js reutilizando los paquetes compartidos (ver Decisiones pendientes).

&#91;embedded content: arquitectura · cliente Expo, Railway y Supabase\]

El cliente final solo habla con la API; el panel lee directo de Supabase con RLS, escribe a través de la API y sube archivos directo a S3 con URLs que firma la API.

## Estructura del monorepo

Un repositorio con pnpm workspaces + Turborepo para TypeScript y un módulo Go independiente; la carpeta `supabase/` es la única fuente de verdad del esquema.

```
/
├─ apps/
│  └─ client/              # Expo: iOS, Android y web (catálogo público + PWA admin)
│     ├─ app/
│     │  ├─ (public)/[slug]/ # catálogo, detalle, reservar/pedir, confirmación
│     │  ├─ (auth)/          # login, registro, recuperar acceso
│     │  └─ (admin)/         # panel del negocio
│     ├─ public/            # manifest.json, iconos, service worker
│     └─ vercel.json        # rewrites de SPA y headers del service worker
├─ packages/
│  ├─ shared/              # esquemas zod, enums, terminología por giro, utilidades de fechas
│  ├─ api-client/          # cliente generado desde openapi.yaml
│  └─ ui/                  # componentes base compartidos (sin diseño final por ahora)
├─ services/
│  └─ api/                 # Go
│     ├─ cmd/api/           # servidor HTTP
│     ├─ cmd/worker/        # jobs programados
│     ├─ internal/          # auth, availability, bookings, payments, notify, catalog
│     ├─ db/queries/        # SQL para sqlc
│     ├─ Dockerfile
│     └─ railway.json       # build, comando y healthcheck por servicio
├─ supabase/
│  ├─ migrations/          # esquema, RLS, funciones, triggers
│  └─ seed.sql             # negocios demo: uñas, flores, herramienta
├─ infra/aws/              # buckets S3, IAM, CORS y CloudFront como código
├─ openapi.yaml            # contrato de la API
├─ CLAUDE.md               # convenciones para Claude Code
└─ .github/workflows/      # ci.yml, deploy-staging.yml, deploy-production.yml, mobile.yml
```

**Convenciones**

- Variables de entorno por app con un `.env.example` cada una; nunca la service role key en `apps/client`.
- Migraciones solo con Supabase CLI; sqlc lee el esquema desde `supabase/migrations`.
- Comandos raíz: `pnpm dev`, `pnpm test`, `pnpm typecheck`, `pnpm gen` (tipos de Supabase + cliente OpenAPI), `make -C services/api test`.

## Modelo de datos

Todas las tablas de negocio llevan `business_id`, y el choque de horarios o unidades lo impide Postgres con restricciones de exclusión, no el código. Un solo catálogo distingue tres tipos de artículo; una sola entidad `bookings` cubre citas, pedidos y rentas.

**Tablas núcleo**

| Tabla | Campos clave | Notas |
| --- | --- | --- |
| `businesses` | id, slug, name, vertical (`nails` \| `flowers` \| `tool_rental`), timezone, phone\_whatsapp, currency, settings jsonb, plan, status | `settings` guarda políticas: anticipo %, horas para cancelar, cupo diario, recargo por retraso |
| `business_members` | business\_id, user\_id (auth.users), role (`owner` \| `staff`), staff\_id | Base de todas las políticas RLS |
| `staff` | business\_id, name, active, color | Empleadas que atienden servicios |
| `business_hours` | business\_id, staff\_id nullable, weekday, opens\_at, closes\_at | Horario general o por empleada |
| `blackouts` | business\_id, staff\_id / item\_unit\_id nullable, starts\_at, ends\_at, reason | Días cerrados, vacaciones, unidad en reparación |
| `catalog_items` | business\_id, kind (`service` \| `product` \| `rental`), name, description, price, active, sort, campos por tipo (abajo) | Un solo catálogo para los tres giros |
| `catalog_item_images` | item\_id, s3\_key, variants jsonb, width, height, blurhash, status, sort | Bucket de catálogo en S3, servido por CloudFront; miniaturas en \`variants\` |
| `item_options` | item\_id, name, type (`text` \| `choice`), choices jsonb, extra\_price, required | Personalización: mensaje de tarjeta, color, tamaño |
| `staff_services` | staff\_id, item\_id | Qué servicios hace cada empleada |
| `item_units` | item\_id, code (número de serie o etiqueta), status (`available` \| `maintenance` \| `retired`), notes | Cada pieza física rentable |
| `customers` | business\_id, name, phone (E.164), email, notes, id\_document\_key | Clientes por negocio; INE solo para rentas, bucket privado de S3 |
| `bookings` | business\_id, customer\_id, kind, status, sub\_status, starts\_at, ends\_at, fulfillment (`pickup` \| `delivery`), delivery\_address, delivery\_window, subtotal, deposit\_required, total, notes, source (`public` \| `admin`), public\_token | Cita, pedido o renta |
| `booking_items` | booking\_id, item\_id, quantity, unit\_price, options jsonb | Líneas del pedido |
| `booking_resources` | booking\_id, staff\_id \| item\_unit\_id, during tstzrange | Lo que queda ocupado; aquí vive la exclusión |
| `payments` | booking\_id, type (`advance` \| `balance` \| `security_deposit` \| `refund` \| `late_fee` \| `damage_fee`), method (`cash` \| `transfer` \| `card`), amount, paid\_at, recorded\_by | Libro de pagos; saldo = total − suma |
| `rental_checkouts` | booking\_id, item\_unit\_id, out\_at, out\_condition, out\_photos, returned\_at, return\_condition, return\_photos | Salida y devolución con evidencia |
| `booking_events` | booking\_id, from\_status, to\_status, actor, at, note | Bitácora de cambios de estado |
| `notifications` | business\_id, booking\_id, channel (`whatsapp_link` \| `push`), template, scheduled\_for, sent\_at, status | Cola y registro |
| `jobs` | kind, payload jsonb, run\_at, attempts, locked\_at, done\_at, error | Cola del worker en Postgres |

**Campos por tipo en `catalog_items`**

- `service`: `duration_minutes`, `buffer_minutes`.
- `product`: `prep_hours` (anticipación mínima), `daily_capacity` (opcional, cupo por día del artículo; el cupo global va en `settings`).
- `rental`: `price_unit` (`day` | `weekend` | `week`), `security_deposit`, `late_fee_per_day`. Las unidades viven en `item_units`.

Se validan con un `CHECK` por `kind` para que cada tipo tenga sus campos obligatorios.

**Reglas de disponibilidad**

1. **Servicio:** el horario cabe dentro de `business_hours` de la empleada, no cae en un `blackout`, y la empleada no tiene otro `booking_resources` que se cruce (`EXCLUDE USING gist (staff_id WITH =, during WITH &&)`). Sin empleada elegida, la API asigna la primera libre que hace ese servicio.
2. **Producto bajo pedido:** la fecha de entrega respeta `prep_hours`, el día no está cerrado y no se supera el cupo diario (conteo dentro de la misma transacción con bloqueo por negocio y fecha).
3. **Renta:** existe al menos una `item_unit` disponible por cada cantidad pedida, sin cruce en el rango (`EXCLUDE USING gist (item_unit_id WITH =, during WITH &&)`). El rango de una unidad incluye un margen configurable para revisión entre rentas.

Las reservas en estado `cancelled` o `expired` liberan sus recursos (se borran sus filas de `booking_resources` o se marca el rango como vacío). Requiere la extensión `btree_gist`.

**Terminología por giro** (en `packages/shared`, no en la base de datos):

| Concepto | Uñas | Flores | Herramienta |
| --- | --- | --- | --- |
| booking | Cita | Pedido | Renta |
| catalog\_item | Servicio | Arreglo | Herramienta |
| starts\_at | Hora de la cita | Fecha de entrega | Fecha de salida |
| ends\_at | (calculado) | (no aplica) | Fecha de regreso |

## Flujos por giro y estados

Los tres giros comparten una misma máquina de estados; cada uno solo usa los estados que le aplican, y toda transición la valida la API y queda en `booking_events`.

&#91;embedded content: estados de bookings · 3 ramas por giro\]

Un pedido de flores para recoger salta `out_for_delivery`, y una renta devuelta a tiempo nunca pasa por `overdue`.

**Uñas (servicio)**

1. La clienta abre `/<slug>`, elige servicio y, si quiere, empleada.
2. La app muestra horarios libres del día elegido (calculados por la API).
3. Captura nombre y teléfono; se crea la cita en `pending_deposit` si el negocio pide anticipo, o en `confirmed` si no.
4. El negocio ve la cita, registra el anticipo y la confirma; se genera el enlace de WhatsApp de confirmación.
5. Un día antes, el worker crea el recordatorio (push al dueño con enlace de WhatsApp listo).
6. Al atenderla: `in_progress` → `completed`, registro del saldo. Si no llegó: `no_show`.

**Flores (producto bajo pedido)**

1. El cliente elige arreglos, llena opciones (mensaje de tarjeta, color) y fecha de entrega.
2. Elige recoger o entrega a domicilio (dirección y ventana horaria; costo de envío fijo por zona en `settings`).
3. La API valida anticipación mínima y cupo del día; el pedido queda en `pending_deposit`.
4. Con anticipo: `confirmed` → `in_progress` (en preparación) → `ready` → `out_for_delivery` (si es a domicilio) → `completed`.
5. Vista del día para el negocio: pedidos por entregar ordenados por hora.

**Herramienta (renta)**

1. El cliente elige herramienta, cantidad y rango de fechas; la API confirma unidades libres.
2. Se crea la renta en `pending_deposit` con depósito de garantía calculado.
3. En mostrador: el negocio registra INE, anticipo y depósito, asigna unidades concretas y hace la salida con fotos y estado: `checked_out`.
4. Si pasa la fecha de regreso, el worker la marca `overdue` y avisa al dueño.
5. Devolución: fotos, estado, cargos por retraso o daño, devolución del depósito (pago tipo `refund`): `returned` → `completed`.

**Reglas comunes**

- Un apartado sin anticipo expira a las N horas (configurable) y libera el horario o las unidades.
- Cancelación por el cliente: con su `public_token` desde el enlace de confirmación, solo antes del límite que fije el negocio.
- Cada cambio de estado puede generar una notificación según una tabla de plantillas por giro.

## Pasos configurables

Los pasos intermedios de cada negocio se resuelven con dos piezas genéricas, no con código por giro: **requisitos de transición** (lo que el negocio completa antes de avanzar un estado) y **armadores de producto** (lo que el cliente arma al pedir). El código define un catálogo fijo de tipos de paso; el negocio elige cuáles usa, en qué orden y si son obligatorios.

**1. Requisitos de transición (lado del negocio)**

| Giro | Antes de pasar a | Pasos de ejemplo |
| --- | --- | --- |
| Herramienta | `checked_out` | Foto del INE (obligatorio), comprobante de domicilio si el valor de la herramienta supera un monto, fotos de la herramienta, firma del contrato |
| Herramienta | `completed` | Fotos de devolución, checklist de estado, cargos aplicados |
| Flores | `ready` | Foto del arreglo terminado para enviarla al cliente |
| Uñas | `completed` | Notas del diseño y tonos usados |

Tipos de paso del catálogo: `file_upload`, `photo`, `checklist`, `text`, `number`, `signature`, `payment_check` (exige un pago registrado) y `approval` (otro miembro aprueba).

| Tabla | Campos clave | Notas |
| --- | --- | --- |
| `workflow_steps` | business\_id, applies\_to\_kind, item\_id nullable, actor (`business` \| `customer`), moment (`on_booking` \| `before_status`), to\_status nullable, step\_type, label, config jsonb, condition jsonb, required, sort, active | Definición del paso; `actor` y `moment` permiten que el cliente suba documentos o conteste preguntas al reservar |
| `booking_steps` | booking\_id, step\_snapshot jsonb, status (`pending` \| `done` \| `skipped`), completed\_by, completed\_at, data jsonb, s3\_keys | Copia de los pasos aplicables al crear la reserva y su resultado |

- Al crear la reserva se copian los pasos aplicables, así un cambio de configuración no rompe reservas en curso.
- `POST .../transition` revisa que el estado destino no tenga pasos obligatorios pendientes; si faltan, responde `422 steps_pending` con la lista.
- Nuevo endpoint `POST /v1/b/{id}/bookings/{bid}/steps/{stepId}` para completar un paso; los archivos suben a S3 con URL prefirmada.
- Si el cliente ya tiene un INE guardado y vigente, el paso puede completarse reutilizándolo.

**2. Armadores de producto (lado del cliente)**

Un artículo puede tener grupos de componentes con cantidad, y el cliente lo arma paso a paso en el catálogo público.

| Tabla | Campos clave | Notas |
| --- | --- | --- |
| `item_component_groups` | item\_id, name, min\_total, max\_total, required, sort | Ej. "Flores" (12 a 36 piezas), "Follaje", "Base" |
| `item_components` | group\_id, name, unit\_price, max\_qty, active | Ej. rosa roja, girasol, lilis; caja o florero |

- Precio = base del artículo + suma de componente × cantidad, calculado y validado en la API; la composición elegida y sus precios quedan congelados en `booking_items.options`.
- Asistente en el catálogo: 1) tamaño y base, 2) flores y cantidades, 3) tarjeta y entrega, con el precio actualizándose en cada paso.
- El mismo mecanismo sirve después para paquetes de salones (mobiliario, meseros) o extras de limosinas (horas, decoración).

**Qué entra al MVP**

- Motor de requisitos con plantillas por defecto por giro: INE obligatorio antes de `checked_out` y fotos de devolución en rentas.
- Armador de arreglos para flores, configurable desde la ficha del artículo.
- Queda para después: editor para que el dueño cree sus propios pasos y condiciones, y firma electrónica con validez legal.

**Cómo agregar un paso nuevo después**

La regla de diseño: configurar un paso que ya existe para otro negocio no requiere código, y crear un tipo de paso nuevo toca solo tres lugares, sin migraciones, porque la configuración y los datos viven en `jsonb`.

| Caso | Qué se hace | ¿Requiere desplegar? |
| --- | --- | --- |
| Un negocio necesita un paso con un tipo existente (otro documento, otro checklist) | Se agrega una fila en `workflow_steps` desde el panel o un script | No |
| Se necesita un tipo de paso nuevo | Implementar el tipo en Go, su esquema y su componente en el cliente (receta abajo) | Sí, sin migración |
| Un negocio quiere un estado propio solo visible ("en revisión de documentos") | Etiqueta en `bookings.sub_status`, libre por negocio, sin efecto en la lógica | No |
| Se necesita un estado con reglas propias | Agregarlo a la tabla de transiciones en un solo archivo y a la terminología | Sí |

**Receta para un tipo de paso nuevo**

1. **Go:** implementar la interfaz `StepType` (`Key`, `ValidateConfig`, `ValidateData`, `OnComplete`) en `internal/steps/` y registrarla en `registry.go`.
2. **Esquemas:** definir en `packages/shared/steps/<tipo>.ts` el esquema de `config` y de `data`; de ahí se exporta JSON Schema que también usa Go para validar.
3. **Cliente:** registrar en `stepRenderers` el componente de captura (para negocio o cliente) y el de configuración. Si no se registra uno propio, el cliente genera un formulario a partir del JSON Schema, así los tipos simples no necesitan interfaz a mano.
4. **Tests** del tipo (config válida e inválida, datos completos e incompletos) y activarlo solo para el negocio que lo pidió con `settings.features`.

**Reglas que hacen esto posible desde el MVP**

- La máquina de estados vive en un solo lugar (`internal/bookings/machine.go`) como tabla de transiciones por tipo de artículo, nunca repartida en `if` por el código.
- Ningún flujo revisa un tipo de paso por nombre; todo pasa por el registro.
- Las plantillas de pasos por giro viven en `packages/shared/templates/` y se aplican al dar de alta un negocio.
- Esta receta se copia a `CLAUDE.md` para que Claude Code la siga cuando se pida un paso nuevo.

## API Go, jobs y notificaciones

La API expone dos grupos de rutas: públicas por `slug` para el cliente final y privadas bajo `/v1/b/{businessId}` para el panel; todas versionadas y descritas en `openapi.yaml`.

**Endpoints públicos** (sin sesión, con rate limit por IP y teléfono)

| Método y ruta | Uso |
| --- | --- |
| `GET /v1/public/{slug}` | Datos del negocio, horario, políticas visibles |
| `GET /v1/public/{slug}/catalog` | Artículos activos con imágenes y opciones |
| `GET /v1/public/{slug}/availability` | Horarios o fechas libres; parámetros `item_id`, `date` o `from`/`to`, `quantity`, `staff_id` |
| `POST /v1/public/{slug}/bookings` | Crea cita, pedido o renta; devuelve `public_token` |
| `GET /v1/public/bookings/{token}` | Estado del pedido para el cliente |
| `POST /v1/public/bookings/{token}/cancel` | Cancelación dentro de política |

**Endpoints privados** (JWT de Supabase + rol en `business_members`)

| Método y ruta | Uso |
| --- | --- |
| `POST /v1/businesses` | Alta del negocio con plantilla según giro |
| `PATCH /v1/b/{id}/settings` | Políticas, horario, datos de contacto |
| `GET/POST/PATCH /v1/b/{id}/catalog` | Catálogo, opciones, unidades de renta |
| `GET /v1/b/{id}/bookings` | Filtros por fecha, estado, tipo |
| `POST /v1/b/{id}/bookings` | Alta manual desde mostrador o teléfono |
| `POST /v1/b/{id}/bookings/{bid}/transition` | Cambio de estado validado por la máquina |
| `POST /v1/b/{id}/bookings/{bid}/payments` | Registrar anticipo, saldo, depósito, reembolso o cargo |
| `POST /v1/b/{id}/bookings/{bid}/checkout` y `/return` | Salida y devolución de renta con fotos |
| `POST /v1/b/{id}/uploads` | URL prefirmada de S3 para subir foto o documento (tipo, tamaño y destino validados) |
| `GET /v1/b/{id}/files/{key}` | URL prefirmada de lectura para archivos privados |
| `GET /v1/b/{id}/bookings/{bid}/whatsapp-link` | Enlace `wa.me` con mensaje armado según plantilla |
| `GET /v1/b/{id}/reports/summary` | Ingresos, reservas por estado, rentas vencidas por periodo |

**Lineamientos de implementación**

- Paquetes por dominio en `internal/`: `auth`, `catalog`, `availability`, `bookings`, `payments`, `rentals`, `notify`, `reports`.
- Creación de reservas en una sola transacción: validar, insertar `bookings`, `booking_items` y `booking_resources`; un choque de la exclusión regresa `409 slot_taken`.
- Endpoints de creación con `Idempotency-Key` para evitar duplicados por doble toque.
- Errores con formato único `{code, message, details}`; logs estructurados con `slog`; `GET /healthz` para Railway.
- Montos en centavos (`bigint`) en MXN, nunca flotantes.

**Worker y jobs**

Cola en la tabla `jobs` tomada con `SELECT … FOR UPDATE SKIP LOCKED` (o la librería River sobre Postgres). Jobs del MVP:

- `expire_unpaid_holds`: cada 5 minutos, expira apartados sin anticipo vencidos.
- `schedule_reminders`: al confirmar, programa recordatorio 24 h antes (citas) o el día anterior (pedidos y rentas).
- `mark_overdue_rentals`: cada hora, pasa rentas a `overdue` y avisa.
- `daily_digest`: cada mañana, push al dueño con la agenda del día.

**Notificaciones**

- **MVP:** push al dueño vía Expo Push, con un botón que abre WhatsApp con el mensaje para el cliente ya escrito (`https://wa.me/<tel>?text=...`). Sin costo por mensaje y sin aprobación de Meta.
- **Después:** el canal para clientes (WhatsApp Cloud API, correo o SMS) se decide más adelante; el sistema se construye desde el MVP para que agregar cualquiera sea solo implementar un canal nuevo, sin tocar los flujos.

**Avisos configurables por el dueño**

Cada negocio decide qué avisos se envían, a quién, por qué canal y con qué anticipación; el código solo conoce eventos y canales, nunca "manda un WhatsApp" directo.

| Pieza | Detalle |
| --- | --- |
| Eventos | `booking_created`, `booking_confirmed`, `reminder`, `ready`, `out_for_delivery`, `checked_out`, `return_due`, `overdue`, `cancelled`, `hold_expiring`, `daily_digest` |
| Destinatario | `customer` o `owner` (dueño y empleados con el rol indicado) |
| Canales | Interfaz `Channel` en `internal/notify/channels/` con `Send(ctx, message)`; MVP: `push` y `whatsapp_link`; después: `whatsapp_api`, `email`, `sms` |
| `notification_rules` | business\_id, event, audience, channels\[\] en orden de preferencia, offset (por ejemplo 24 h antes), enabled, template\_key |
| `notification_templates` | business\_id nullable (null = plantilla por defecto del giro), event, channel, body con variables (`{{cliente}}`, `{{fecha}}`, `{{articulo}}`, `{{saldo}}`, `{{link}}`) |
| `customers` | Agregar `email`, `preferred_channel` y consentimientos por canal con fecha (`consent_whatsapp_at`, `consent_sms_at`, `consent_email_at`) |
| `notifications` | Ampliar `channel` a todos los canales y agregar `recipient`, `provider_message_id` y `error` para seguimiento |

- Al ocurrir un evento, `notify` busca las reglas activas, arma el mensaje con la plantilla del canal y encola un job por canal; si el primer canal falla o el cliente no tiene ese dato o consentimiento, intenta el siguiente de la lista.
- Las plantillas por defecto de cada giro viven en `packages/shared/templates/` y el dueño solo las ajusta si quiere.
- Pantalla **Ajustes → Avisos** en el panel: una fila por evento con interruptor, destinatario, canales disponibles, anticipación y botón para editar el texto con vista previa. Los canales no conectados aparecen deshabilitados con la leyenda "próximamente".
- Agregar un canal nuevo = implementar `Channel`, registrar sus credenciales y su plantilla por defecto; las reglas existentes lo pueden usar sin migrar datos.
- Tests: suite de contrato común para todos los canales y tests de las reglas (evento sin regla, canal sin consentimiento, respaldo al siguiente canal, horario de envío).

## Seguridad, privacidad y almacenamiento

Ningún negocio debe poder ver datos de otro, y eso se garantiza en la base de datos con RLS, no solo en la API.

**RLS en Supabase**

- RLS activo en todas las tablas. Función `is_member(business_id, roles[])` basada en `business_members` y `auth.uid()`.
- RLS da **solo lectura** (decidido 2026-10-27): `anon` y `authenticated` no tienen permisos de escritura en `public`; toda escritura pasa por la API Go con la service role, que aplica los permisos por rol (`owner`: todo su negocio; `staff`: estados y pagos de las reservas que atiende).
- Lectura: cualquier miembro lee los datos de su negocio; avisos y equipo completo solo el `owner`.
- El rol `anon` no lee tablas directamente; el catálogo público se sirve desde la API.
- Tests de RLS en SQL (pgTAP o scripts con dos usuarios de negocios distintos) como parte del CI.

**Archivos en AWS S3**

| Bucket (uno por entorno) | Acceso | Contenido |
| --- | --- | --- |
| `catalog` | Privado en S3; lectura pública solo a través de CloudFront (Origin Access Control); escritura con URL prefirmada | Fotos de artículos y logo |
| `private-docs` | Privado; lectura con URL prefirmada de 5 minutos | INE de clientes de renta |
| `rental-evidence` | Privado; lectura con URL prefirmada | Fotos de salida y devolución |

- La API Go firma URLs prefirmadas (PUT para subir, GET para leer) solo después de verificar que el usuario es miembro del negocio; el archivo va directo del cliente a S3 sin pasar por la API.
- Llaves con prefijo `{business_id}/{tipo}/{id}/{uuid}.{ext}`; la API solo firma llaves del negocio del usuario.
- La firma fija `Content-Type` y tamaño máximo (imágenes JPG, PNG o WebP, hasta 5 MB).
- Block Public Access activo en los tres buckets, cifrado SSE-S3 y versionado en los privados.
- Usuario o rol IAM exclusivo de la API con permisos mínimos (`s3:PutObject`, `s3:GetObject`, `s3:DeleteObject`) sobre sus buckets; credenciales solo en variables de Railway.
- CORS de los buckets limitado a los dominios de Vercel del entorno.
- Configuración de buckets e IAM versionada en `infra/aws/` (Terraform u OpenTofu), incluida la distribución de CloudFront.

**Imágenes: CloudFront como CDN y miniaturas**

Las imágenes públicas se sirven desde CloudFront con caché de larga duración, y cada foto se guarda en varios tamaños para que listas, catálogo y detalle descarguen solo lo que muestran.

| Pieza | Detalle |
| --- | --- |
| Dominio | Mientras no haya dominio propio, el dominio `*.cloudfront.net` de la distribución; después `img.<dominio>` con certificado de ACM (us-east-1) |
| Distribución | CloudFront con el bucket de catálogo como origen privado vía Origin Access Control (OAC); la política del bucket solo permite `s3:GetObject` a esa distribución. El bucket nunca es público |
| Caché | Llaves inmutables (cada versión de una foto tiene su propio uuid), así se usa `Cache-Control: public, max-age=31536000, immutable` y nunca hace falta purgar |
| Variantes | `thumb` 160 px, `card` 480 px, `detail` 1080 px y el original; en WebP (y AVIF cuando convenga), sin metadatos EXIF |
| Generación | Job `generate_image_variants` del worker con libvips (govips): al confirmar la subida, descarga el original, crea las variantes, las sube a S3 y llena `variants`, `width`, `height` y `blurhash` |
| Cliente | `expo-image` con la variante según el tamaño en pantalla (`srcset` en web) y el `blurhash` como marcador mientras carga |

- Flujo de subida: `POST /v1/b/{id}/uploads` devuelve la URL prefirmada → el cliente sube el original → `POST /v1/b/{id}/uploads/{uploadId}/complete` valida que el objeto exista y encola el job. Mientras se generan las variantes, la imagen queda en `status = processing` y el panel muestra el marcador.
- El cliente reduce la foto antes de subirla (lado mayor de 2048 px) para ahorrar datos móviles y tiempo de procesamiento.
- Las fotos privadas (INE y evidencias de renta) también tienen miniatura para que el panel cargue rápido, pero se quedan en sus buckets privados y se leen con URL prefirmada, nunca por el CDN.
- Sin credenciales nuevas: CloudFront se configura con las mismas credenciales de AWS (`infra/aws/`); la API recibe solo la URL base pública del CDN (`CATALOG_CDN_URL`).
- Claude Code puede hacer `plan` de la distribución en staging; crear o cambiar la distribución, invalidaciones y cualquier acción en producción requieren tu aprobación.
- Tests: el job genera las variantes esperadas con dimensiones y formato correctos, rechaza archivos que no son imagen y es idempotente si se ejecuta dos veces; CloudFront no sirve llaves fuera del bucket de catálogo y el bucket rechaza lecturas directas.

**Endpoints públicos**

- Rate limit por IP y por teléfono; límite de reservas pendientes por teléfono para evitar apartados falsos.
- `public_token` aleatorio de 32 bytes, no adivinable; nunca exponer IDs internos de otros clientes.
- Validación de entrada con zod en el cliente y de nuevo en Go.

**Privacidad (México)**

- Se manejan datos personales (nombre, teléfono, domicilio, INE), así que se necesita aviso de privacidad conforme a la ley federal de protección de datos personales: uno de la plataforma y una plantilla que cada negocio muestre en su catálogo. Conviene revisarlo con un abogado antes de vender.
- Casilla de aceptación del aviso en el formulario público; guardar fecha y versión aceptada.
- Borrado o anonimización de cliente a solicitud; borrado automático del INE en S3 N días después de cerrar la renta (job \`purge\_id\_documents\` del worker).

**Secretos y entornos**

- Dos entornos remotos: **pruebas** (`staging`) y **producción** (`production`), cada uno con su propio proyecto de Supabase, sus servicios en Railway, su proyecto en Vercel y sus buckets de S3. El desarrollo local usa Supabase CLI (`supabase start`) y no consume ninguno de los dos proyectos.
- Service role key, JWKS URL, credenciales de AWS y tokens de Expo solo como variables de Railway y secretos de GitHub; la web en Vercel solo recibe valores públicos (URL de la API, URL de Supabase y anon key). Nunca en el repo.

## Pantallas (lista funcional)

Son 20 pantallas para el MVP; el diseño visual se define después, así que en esta etapa se construyen con componentes base y sin pulir.

| Área | Pantalla | Plataforma | Qué permite |
| --- | --- | --- | --- |
| Público | Catálogo del negocio | Web | Ver artículos, precios, fotos, horario y contacto |
| Público | Detalle de artículo | Web | Opciones, empleada (uñas), cantidad (renta) |
| Público | Elegir fecha y hora | Web | Horarios o fechas libres según el tipo |
| Público | Datos y confirmación | Web | Nombre, teléfono, entrega, aviso de privacidad |
| Público | Estado del pedido | Web | Ver estado por `public_token`, cancelar dentro de política |
| Acceso | Login y registro | Móvil y PWA | Correo con enlace mágico o contraseña (Supabase Auth) |
| Alta | Onboarding del negocio | Móvil y PWA | Nombre, slug, giro, horario, plantilla de catálogo |
| Panel | Hoy | Móvil y PWA | Agenda o entregas del día, rentas que salen y regresan |
| Panel | Calendario | Móvil y PWA | Vista semana y día, por empleada o por unidad |
| Panel | Lista de reservas | Móvil y PWA | Filtros por estado, fecha y tipo; búsqueda por cliente |
| Panel | Detalle de reserva | Móvil y PWA | Cambiar estado, pagos, enlace de WhatsApp, bitácora |
| Panel | Nueva reserva manual | Móvil y PWA | Alta desde mostrador o llamada |
| Renta | Salida de renta | Móvil y PWA | Asignar unidades, INE, fotos, depósito |
| Renta | Devolución | Móvil y PWA | Fotos, estado, cargos, reembolso de depósito |
| Catálogo | Lista y edición de artículos | Móvil y PWA | Alta, fotos, opciones, activo o inactivo |
| Catálogo | Unidades de renta | Móvil y PWA | Piezas, estado, mantenimiento |
| Equipo | Empleadas y horarios | Móvil y PWA | Alta, servicios que hace, horario, invitar usuario |
| Clientes | Lista y ficha de cliente | Móvil y PWA | Historial, notas, saldo pendiente |
| Ajustes | Configuración del negocio | Móvil y PWA | Políticas, anticipo, cupo y envío; sección Avisos con evento, destinatario, canal, anticipación y texto de cada aviso |
| Reportes | Resumen | Móvil y PWA | Ingresos, reservas por estado, rentas vencidas |

Las pantallas del panel cambian etiquetas y campos según el giro usando la terminología de `packages/shared`; no se duplican por giro.

## CI/CD y despliegue

GitHub Actions es el único camino a producción: cada PR corre checks y genera preview, cada merge a `main` despliega a staging, y producción sale solo de un tag `v*` con aprobación manual. Claude Code usa los CLIs de Railway, Vercel, GitHub y Supabase para operar staging y revisar estado, no para saltarse el pipeline.

**Flujo de ramas y entornos**

| Evento | Qué corre | Destino |
| --- | --- | --- |
| PR abierto o actualizado | `ci.yml`: lint, typecheck, tests de Go y TS, tests de RLS con `supabase start`, build de la imagen Docker, `vercel build` | Preview de la web en Vercel (URL comentada en el PR); la API no se despliega |
| Merge a `main` | `deploy-staging.yml`: migraciones con `supabase db push`, `railway up` de `api` y `worker`, `vercel deploy --prebuilt` | Supabase, Railway y Vercel de staging |
| Tag `v*` | `deploy-production.yml` con GitHub Environment `production` (requiere aprobación): mismas etapas en orden migraciones → API → web | Producción |
| Tag `mobile-v*` o manual | `mobile.yml`: `eas build` para iOS y Android y `eas submit` a TestFlight y prueba interna de Play; `eas update` para cambios solo de JS | Tiendas y canales de EAS |

**Reglas del pipeline**

- Protección de `main`: PR obligatorio, checks en verde y al menos una revisión.
- Orden de despliegue siempre: migración (compatible hacia atrás) → API y worker → web, para que la web nueva nunca hable con una API vieja.
- Railway: healthcheck en `/healthz` definido en `railway.json`; si falla, Railway mantiene la versión anterior.
- Vercel: `vercel pull`, `vercel build` y `vercel deploy --prebuilt` desde Actions (desactivar el despliegue automático por Git de Vercel para no duplicar). `vercel.json` con rewrites de SPA y `Cache-Control: no-cache` para `sw.js` y `manifest.json`.
- La API acepta CORS solo de los dominios de Vercel del entorno (incluido el patrón de previews en staging).
- Dependabot o Renovate para dependencias de Go, npm y GitHub Actions.

**Secretos en GitHub** (por Environment `staging` y `production`)

`RAILWAY_TOKEN`, `VERCEL_TOKEN`, `VERCEL_ORG_ID`, `VERCEL_PROJECT_ID`, `SUPABASE_ACCESS_TOKEN`, `SUPABASE_DB_PASSWORD`, `SUPABASE_PROJECT_REF`, `EXPO_TOKEN`.

**CLIs conectados a Claude Code**

| CLI | Para qué lo usa Claude Code | Requiere tu aprobación |
| --- | --- | --- |
| `gh` | Crear ramas y PRs, leer checks y logs de Actions, crear tags | Crear tags de release y hacer merge a `main` |
| `railway` | `status`, `logs`, `variables` de lectura, `up` a staging | Cambiar variables y cualquier acción en production |
| `vercel` | `ls`, `inspect`, `logs`, deploy de preview | `env add/rm` y `--prod` |
| `supabase` | `start`, `db reset`, `migration new`, `gen types` en local; `db push` a staging | `db push` a production |
| `aws` | `s3 ls` y revisar objetos, políticas y CORS en staging; `plan` de `infra/aws/` | Crear o cambiar buckets o IAM, borrar objetos y cualquier acción en production |
| `eas` | `build` de prueba, `update` al canal de preview | `submit` a tiendas y `update` a production |

**Modelo de trabajo con Claude Code**

Tú colocas las credenciales en un archivo local que nunca se sube al repo; Claude Code opera todo el proyecto desde consola con esos CLIs, y tú apruebas cada cambio sensible.

- **Archivo de credenciales:** `.env.local` en la raíz (en `.gitignore`), con un `.env.local.example` que lista solo los nombres. Se carga con direnv (`.envrc`) o `dotenv` antes de cada comando.
- **Variables que leen los CLIs:** `GH_TOKEN`, `RAILWAY_TOKEN`, `VERCEL_TOKEN` (+ `VERCEL_ORG_ID`, `VERCEL_PROJECT_ID`), `SUPABASE_ACCESS_TOKEN`, `AWS_PROFILE` o `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`, `EXPO_TOKEN`.
- **Tokens con el menor alcance posible:** token de proyecto de Railway por entorno, token de Vercel limitado al equipo del proyecto, usuario IAM de AWS solo con permisos sobre los recursos del proyecto, token de GitHub de grano fino limitado a este repositorio.
- **Recomendado:** en el `.env.local` solo van credenciales de **staging**. Producción se despliega únicamente desde GitHub Actions con sus secretos, así un error en consola nunca toca a clientes reales.
- **Aprobaciones forzadas por configuración:** `.claude/settings.json` define permisos de comandos: lectura y local permitidos, modificaciones de staging con aprobación (`ask`), y bloqueados (`deny`) `--prod`, `db push` contra producción, borrados en S3 y cambios de IAM.
- **Protección de secretos:** gitleaks como hook de pre-commit y en CI; Claude Code nunca imprime, copia ni hace commit de valores de credenciales, y nunca edita `.env.local` por su cuenta.

**Orden para ir conectando servicios**

| Paso | Servicio | Qué se conecta | Fase |
| --- | --- | --- | --- |
| 1 | GitHub | Repo, protección de `main`, `gh` con token del repo | 0 |
| 2 | Supabase local | `supabase start`, sin credenciales externas | 0 |
| 3 | Railway staging | Proyecto, servicios `api` y `worker`, healthcheck | 0 |
| 4 | Vercel | Proyecto web, preview por PR, despliegue desde Actions | 0 |
| 5 | Supabase staging | Proyecto remoto, migraciones con `db push` | 1 |
| 6 | AWS S3 y CloudFront (staging) | Buckets, IAM y distribución de CloudFront desde `infra/aws/` | 1 |
| 7 | Expo / EAS | Builds de prueba y push | 5 |
| 8 | Producción | Proyectos de producción y secretos en el Environment `production` de GitHub | 6 |

Cada paso se valida antes del siguiente: Claude Code confirma con un comando de solo lectura (`status`, `ls`, `whoami`) que la conexión funciona y reporta el resultado.

**Entornos de la app móvil**

La app tiene credenciales de pruebas y de producción, y como hay un solo dispositivo físico, el entorno se alterna a mano al compilar o arrancar la app.

|  | Pruebas | Producción |
| --- | --- | --- |
| Variable que elige el entorno | `APP_ENV=development` | `APP_ENV=production` |
| Archivo de variables | `apps/client/.env.development` | `apps/client/.env.production` |
| Apunta a | API de Railway staging, Supabase de pruebas, buckets de staging | API de Railway production, Supabase de producción, buckets de producción |
| Perfil de EAS | `development` (dev client) y `preview` | `production` |
| Bundle ID / package | `com.<app>.dev` | `com.<app>` |
| Indicador visual | Franja "PRUEBAS" fija en pantalla y nombre de app con sufijo "Dev" | Ninguno |

- `app.config.ts` lee `APP_ENV` y arma nombre, bundle ID, ícono y variables `EXPO_PUBLIC_*` (`API_URL`, `SUPABASE_URL`, `SUPABASE_ANON_KEY`). Solo van valores públicos; ninguna llave secreta entra a la app.
- Comandos para alternar: `pnpm app:dev` (arranca con `APP_ENV=development`) y `pnpm app:prod`; `eas build --profile development` o `--profile production` para instalar.
- Al usar bundle IDs distintos, las dos versiones pueden quedar instaladas al mismo tiempo en el mismo teléfono; alternar sigue siendo manual, pero sin reinstalar cada vez.
- La pantalla de ajustes muestra el entorno y la versión, para confirmar siempre contra qué datos se está probando.
- Las notificaciones push se registran por entorno; un token de pruebas nunca se guarda en producción.

## Estrategia de pruebas

Todo lo que tenga lógica lleva test unitario, y cada criterio de aceptación de las fases tiene al menos un test automatizado que lo comprueba; una fase no se cierra ni un PR se integra con pruebas en rojo.

| Capa | Qué se prueba | Herramienta |
| --- | --- | --- |
| Dominio en Go | Disponibilidad, máquina de estados, cálculo de saldo, precios del armador, cada tipo de paso | `testing` con tests de tabla, sin base de datos |
| Integración en Go | Transacciones, restricciones de exclusión, `409 slot_taken` concurrente, idempotencia, jobs del worker | Postgres real con Supabase local o testcontainers-go |
| API HTTP | Handlers, validación de JWT y roles, formato de errores, respuestas contra `openapi.yaml` | `httptest` + kin-openapi |
| Base de datos | Políticas RLS, `CHECK` por tipo de artículo, triggers | pgTAP |
| TypeScript compartido | Esquemas zod, terminología por giro, fechas y zona horaria, cálculo de precio en cliente | Vitest |
| Componentes y pantallas | Formularios, renderizado de pasos por esquema, estados de carga y error | React Native Testing Library con jest-expo |
| End-to-end | Reservar desde el catálogo y operar la reserva en el panel, por giro | Playwright contra el preview web; Maestro en móvil desde la fase 5 |

**Reglas**

- **Test primero donde el requerimiento es claro:** disponibilidad, transiciones, precios, saldos y pasos se escriben empezando por el test.
- **Cobertura mínima de 80 %** en `internal/availability`, `internal/bookings`, `internal/steps`, `internal/payments` y `packages/shared`. En pantallas se prueban comportamientos, no se persigue un porcentaje.
- **Casos de borde obligatorios:** cambio de día en la zona del negocio, reservas simultáneas, montos en centavos, transiciones inválidas, permisos por rol y por negocio, pasos obligatorios pendientes.
- **Reloj inyectable:** nada llama a `time.Now()` o `Date.now()` directo en la lógica, para poder probar recordatorios, vencimientos y expiraciones.
- **Suite de contrato para pasos:** cada tipo registrado en `StepType` debe pasar la misma batería (config válida e inválida, datos completos e incompletos, `OnComplete`).
- **Datos de prueba:** factories en Go y TS, y el seed de los tres negocios demo como base para integración y E2E.
- **Cada bug lleva primero un test** que lo reproduce y después el arreglo.

**Trazabilidad con los requerimientos**

- Los criterios de aceptación se identifican por fase y posición: `F3-2` es el segundo criterio de la fase 3.
- Los tests que verifican un criterio llevan su ID en el nombre (por ejemplo `TestF3_2_ReservaSimultaneaDevuelve409`).
- Al cerrar cada fase, Claude Code entrega una tabla criterio → test → resultado; un criterio sin test no cuenta como cumplido.
- CI corre unitarios, integración, pgTAP y Vitest en cada PR, y Playwright contra el preview de Vercel.

## Fases de desarrollo

> **Desde 2026-10-30 el orden del trabajo pendiente lo marca `docs/plan-ventas.md`** (priorizado por lo que vende): adelanta en partes pequeñas la fase 4 (recordatorio de WhatsApp de un toque) y la fase 6 (link de reservas público).

Siete fases en orden, con prioridad al panel de administración: primero se construye y prueba todo lo que usa el dueño, y el catálogo público con su diseño llamativo llega en la fase 6. Cada fase termina con algo demostrable y no se pasa a la siguiente sin cumplir sus criterios de aceptación, cada uno verificado por un test (ver Estrategia de pruebas).

**Trabajo por partes**

Cada fase se divide en partes pequeñas que se revisan antes de continuar, para confirmar que lo construido se ajusta a lo que se busca antes de apilar más encima.

1. Al iniciar la fase, Claude Code propone la lista de partes (por ejemplo, fase 3: disponibilidad de servicios → de pedidos → de rentas → Hoy y calendario → detalle y transiciones → pagos → salida y devolución → armador de flores) y espera tu visto bueno.
2. Construye una parte a la vez, con sus tests, en su propia rama y PR.
3. Al terminarla se detiene y entrega: qué hizo, cómo verlo (preview de Vercel, build de pruebas en el teléfono o comando), resultado de los tests y lo que quedó pendiente.
4. Tú revisas y apruebas, pides cambios o cambias el rumbo; nada se integra a `main` sin tu aprobación.
5. Solo con tu confirmación sigue con la siguiente parte.

**Fase 0 — Cimientos**

Monorepo, Supabase local, API Go con `/healthz`, Expo corriendo en iOS, Android y web, CI/CD en GitHub Actions, API en Railway staging y web en Vercel.

- [ ] `pnpm dev` levanta Supabase local, la API y el cliente con un solo comando.
- [ ] CI corre lint, typecheck y tests de Go y TypeScript en cada PR.
- [ ] La API en Railway staging responde `/healthz` y la web queda publicada en Vercel con preview automático por PR.

**Fase 1 — Esquema, RLS y auth**

Migraciones de todas las tablas, exclusiones, RLS, buckets de S3 con IAM, CloudFront frente al bucket de catálogo, y seed con tres negocios demo.

- [ ] Un usuario de un negocio no puede leer ni escribir datos de otro (tests de RLS).
- [ ] Insertar dos `booking_resources` cruzados para la misma empleada o unidad falla.
- [ ] La API valida JWT de Supabase y resuelve negocio y rol.

**Fase 2 — Alta de negocio y catálogo**

Onboarding con giro y plantilla, CRUD de catálogo con fotos y opciones, empleadas y horarios, unidades de renta.

- [ ] Un dueño nuevo deja listo un catálogo de su giro en menos de 10 minutos con la plantilla.
- [ ] Las fotos del catálogo suben a S3 con URL prefirmada y se ven en el panel, en móvil y en la PWA.

**Fase 3 — Disponibilidad y operación del panel**

Motor de disponibilidad para los tres tipos y operación completa desde el panel: reservas capturadas en mostrador o por teléfono, Hoy, calendario, lista y detalle, transiciones con requisitos configurables, pagos, salida y devolución de rentas, y armador de arreglos de flores dentro del alta manual.

- [ ] Tests de tabla en Go cubren: horario fuera de turno, blackout, empleada ocupada, cupo diario lleno, sin unidades libres, cambio de día en zona horaria del negocio.
- [ ] Dos reservas creadas al mismo tiempo para el mismo horario o unidad: una gana, la otra recibe `409 slot_taken`.
- [ ] Un apartado sin anticipo expira y libera el recurso.
- [ ] Cada giro recorre su flujo completo de principio a fin desde el panel.
- [ ] El saldo pendiente siempre cuadra con `total − pagos`.
- [ ] Transiciones inválidas o con pasos obligatorios pendientes se rechazan, y todas las válidas quedan en la bitácora.

**Fase 4 — Avisos y jobs**

Worker con los jobs, sistema de avisos configurable con push y enlaces de WhatsApp, y pantalla Ajustes → Avisos.

- [ ] El dueño recibe el recordatorio a tiempo con el mensaje de WhatsApp ya armado.
- [ ] Rentas vencidas pasan a `overdue` y generan aviso.
- [ ] Desactivar un aviso en Ajustes → Avisos detiene su envío sin desplegar.

**Fase 5 — PWA admin, reportes y piloto del panel**

Manifest, service worker e instalación de la PWA admin; reporte de resumen; aviso de privacidad; builds de prueba con EAS.

- [ ] La PWA admin se instala en Android y iOS desde el navegador.
- [ ] Builds internos en TestFlight y en la prueba interna de Google Play.
- [ ] Tres negocios piloto reales (uno por giro) operan una semana con el panel sin ayuda.

**Fase 6 — Diseño y catálogo público**

Catálogo público `/<slug>` con un diseño llamativo por giro, reservas y pedidos desde la web, asistente del armador de flores para el cliente y estado del pedido por enlace. Antes de construir se define la dirección visual (tipografía, color, movimiento y una plantilla por giro) y después se aplica la misma capa visual al panel.

- **Skills de diseño:** se usan las skills de Emil Kowalski para Claude Code (repositorio `emilkowalski/skills`, empezando por `emil-design-eng`), instaladas en el proyecto al iniciar esta fase. Guían animaciones, transiciones, tiempos y detalles de interacción.
- **Movimiento con propósito:** las animaciones apoyan la navegación y la respuesta a acciones (elegir horario, armar el ramo, confirmar), y se desactivan con `prefers-reduced-motion`.

* [ ] Cada giro tiene su plantilla visual de catálogo, y el negocio puede usar su logo y colores.
* [ ] **Landing personalizada (servicio extra de pago):** página de inicio propia para el negocio que la contrate, con bloques configurables o diseño a la medida y dominio propio opcional; la reserva usa el mismo flujo del catálogo. Ver `docs/mapa-giros.md`.
* [ ] Una reserva o pedido hecho desde la web aparece en el panel con el mismo flujo de la fase 3.
* [ ] El cliente arma un ramo con tipos y cantidades y ve el precio actualizarse en cada paso.
* [ ] Lighthouse móvil del catálogo con 90 o más en rendimiento y accesibilidad.
* [ ] Playwright cubre reservar, pedir y rentar desde el catálogo de cada giro.

**Fase 7 (posterior)** — WhatsApp Cloud API, pagos en línea del cliente, cobro de la suscripción, nuevos giros (lavado de autos, lavado de salas, tiendas, limosinas, salones, casas por noche) sobre el mapa por giro y editor para que cada dueño configure sus propios pasos.

## Mapa por giro y superadministrador

La plataforma debe adaptarse a muchos giros (renta de herramienta, uñas, florería, lavado de autos, lavado de salas, tiendas…) sin reescribir pantallas. Cada giro se describe con un **mapa** montado sobre un **motor** (cómo se aparta: agenda, entrega con fecha, renta por unidades, venta sin fecha) y cada negocio ajusta sus propios parámetros. El diseño completo, el mapa de renta de herramienta (el primero terminado) y el camino para llegar están en [`docs/mapa-giros.md`](mapa-giros.md).

- [x] Textos y secciones de Ajustes por giro (`verticalTexts.ts`, 2026-10-28).
- [x] Fuente única por giro en código: un archivo con todo su mapa; las pantallas preguntan por funciones, no por giro (2026-10-28).
- [x] Funciones por negocio (2026-10-28): el giro trae unas, el superadministrador da o quita por negocio (`businesses.feature_grants`) y el dueño enciende o apaga las disponibles en Ajustes (`settings.features`).
- [ ] Panel de superadministrador: negocios, su giro, plan, funciones y ajustes; entrar como el negocio con bitácora de auditoría. *Primera parte hecha (2026-10-28): rol `platform_admins`, lista de negocios y funciones por negocio.*
- [ ] Giros en la base (`business_types`), editables desde el panel de superadministrador y validados contra los motores.
- [ ] Motores nuevos según los giros que entren: servicio a domicilio (dirección, zona y traslado) y venta sin fecha (existencias).

## Pendientes para después

### Notas de UI/UX (2026-10-27)

- En el detalle de una renta, los pasos para **registrar la salida** (documentos, fotos, cobro y botón de salida) deben quedar más arriba: es lo que más se usa en mostrador. El rediseño por pasos numerados ya los acerca; revisar con uso real.
- Diseño: sistema estilo iOS en `apps/client/ui` (mismos tokens que fastmarketcuu, curvas de Emil Kowalski). Todo el panel ya lo usa; falta el catálogo público cuando se construya.

Lista de puntos a considerar, agrupados por cuándo conviene resolverlos; ninguno bloquea arrancar la fase 0, y cada uno se integra a una fase cuando se decida hacerlo.

**Antes de los pilotos**

- [ ] **Monitoreo de errores y caídas:** Sentry en Go, app y web; monitor de disponibilidad sobre `/healthz` con alertas al teléfono.
- [ ] **Respaldos y restauración:** backups diarios en Supabase de producción (y restauración a un punto en el tiempo si el plan lo permite), versionado en S3 y al menos un simulacro de restauración antes de tener clientes.
- [ ] **Antispam en el catálogo público:** captcha invisible (Cloudflare Turnstile) en el formulario de reserva, además del rate limit.
- [ ] **Requisitos de las tiendas:** borrar la cuenta desde la app (obligatorio en Apple), etiquetas de privacidad en App Store y sección de seguridad de datos en Google Play.
- [ ] **Términos y condiciones:** de la plataforma y plantillas para los negocios, como el contrato de renta de herramienta.

**Ajustes de renta (pedido por el dueño, 2026-10-28)**

- [ ] **Rentas sobre días cerrados:** opción en Ajustes del negocio "Permitir rentas que pasen por días cerrados" (`settings.rentals_span_closed_days`, por defecto sí). Si es no, un rango que incluya un día cerrado no está disponible.
- [ ] **Cobro de días cerrados:** con la opción anterior activa, "Cobrar los días cerrados" (`settings.charge_closed_days`, por defecto sí). Si es no, los días cerrados no cuentan en el precio por día. Decisión de negocio: algunos cobran fines de semana y otros no.
- Hoy: no se puede salir ni regresar en un día cerrado; un día cerrado en medio de la renta se permite y se cobra.

**Antes de vender**

- [ ] **Planes y cobro de la suscripción:** periodo de prueba, límites por plan (empleadas, unidades en renta, reservas al mes), avisos de pago vencido y suspensión.
- [ ] **Panel de superadministrador:** ver negocios, activar funciones, dar soporte y entrar como un negocio con registro de auditoría (detalle en «Mapa por giro y superadministrador»).
- [ ] **Alta rápida:** importar catálogo desde Excel o CSV y generar QR y link imprimibles para el local.
- [ ] **Marca del negocio en su catálogo:** logo, colores y foto de portada.
- [ ] **Link de pago para anticipos:** link de Mercado Pago generado por la API y conciliado con `payments`.
- [ ] **Decidir los canales de avisos a clientes:** WhatsApp Cloud API, correo (Resend o similar) y/o SMS (Twilio o similar); comparar costo por mensaje, aprobación de plantillas y entregabilidad en México, y si el costo lo absorbe el plan o se cobra aparte. El sistema de avisos ya queda listo para conectarlos.

**Seguridad adicional**

- [ ] **Análisis automático en CI:** CodeQL o gosec para el código y Trivy para la imagen Docker, además de gitleaks.
- [ ] **Bitácora de auditoría** de acciones administrativas: cambios de precios, ajustes, permisos y borrados.
- [ ] **Gestión de sesiones:** revocar sesiones al dar de baja a un empleado, invitaciones con expiración y MFA opcional para dueños.
- [ ] **Rotación de credenciales** periódica y procedimiento escrito por si una se filtra.
- [ ] **Salida de un cliente:** exportar sus datos y borrarlos al cancelar la suscripción.

**Producto y calidad**

- [ ] **Versión mínima de la app:** la API rechaza versiones viejas y la app pide actualizar.
- [ ] **Analítica de uso** (PostHog o similar) para ver qué funciones usan los pilotos y dónde se atoran.
- [ ] **Conexión inestable:** reintentos y estados claros cuando falla la red, sobre todo en salida y devolución de rentas.
- [ ] **Accesibilidad básica:** tamaños de texto, contraste y soporte de lector de pantalla.

**Ya mencionados en el plan como posteriores**

- [ ] WhatsApp Cloud API con envío automático y confirmación con botón.
- [ ] Pagos en línea completos del cliente final.
- [ ] Editor para que cada dueño configure sus propios pasos, condiciones encadenadas y firma electrónica con validez legal.
- [ ] Nuevos giros: limosinas, salones de eventos y casas por noche.
- [ ] Facturación CFDI, inventario de insumos, nómina y multi-sucursal.
- [ ] Renta de casas a largo plazo (contratos y cobro mensual).

## Decisiones pendientes e instrucciones para Claude Code

Cinco decisiones conviene cerrarlas antes de la fase 1; el resto del plan ya se puede ejecutar.

**Decisiones pendientes**

- [x] **Web pública:** Expo web (decidido 2026-10-26). Si el SEO llega a ser prioritario, el catálogo migra a Next.js reutilizando `packages/shared`.
- [x] **URL de cada negocio:** ruta `dominio.com/<slug>` (decidido 2026-10-26).
- [x] **Acceso de clientes finales:** solo nombre y teléfono (decidido 2026-10-26); apartados falsos se controlan con rate limit, límite de pendientes por teléfono y expiración sin anticipo.
- [x] **Cola de jobs:** librería River sobre Postgres (decidido 2026-10-26).
- [x] **Giros en código o en base:** un archivo por giro en código (tipado y probado) y pasar a `business_types` cuando exista el panel de superadministrador (decidido 2026-10-28). Los giros se agregan uno a uno conforme lleguen negocios reales con sus requerimientos.
- [ ] **Landing personalizada:** bloques configurables, diseño a la medida o ambos según el plan contratado.
- [ ] **Pagos en línea (fase 7):** Mercado Pago, Stripe o Conekta, según comisiones y lo que usen los negocios piloto.

**Instrucciones para Claude Code** (copiar a `CLAUDE.md`)

1. Trabaja una fase a la vez, en orden, y dentro de cada fase por partes pequeñas (ver Trabajo por partes). Al terminar cada parte, detente, entrega cómo verla y sus tests, y espera mi confirmación antes de seguir. Al cerrar la fase, entrega la tabla criterio → test → resultado.
2. Antes de cada fase, propón el plan de archivos y espera aprobación si cambia algo de este documento.
3. El esquema solo cambia con nuevas migraciones en `supabase/migrations`; nunca edites una migración ya aplicada.
4. Después de cambiar el esquema o `openapi.yaml`, corre `pnpm gen` y confirma que typecheck pasa.
5. Toda regla de negocio (disponibilidad, estados, pagos) vive en Go y se escribe empezando por sus tests unitarios; el cliente solo valida formato, y también lleva tests donde haya lógica.
6. Montos en centavos, fechas en UTC con zona del negocio; nada de `float` para dinero ni fechas locales sin zona.
7. Textos visibles en español de México, centralizados en `packages/shared` para poder adaptarlos por giro.
8. Nada de diseño visual antes de la fase 6: componentes base funcionales y accesibles. En la fase 6 se instalan y usan las skills de emilkowalski/skills para el diseño del catálogo y después del panel.
9. Nunca subas secretos; usa `.env.example` y variables de Railway y Vercel, y secretos de GitHub; para cualquier CLI sigue las reglas de la sección CI/CD y despliegue.
10. Si algo de este plan es ambiguo o contradictorio, pregunta antes de construir.
11. Los commits y PRs no llevan ninguna atribución a Claude: nada de `Co-Authored-By: Claude`, ni firmas o enlaces de "Generated with Claude Code". Desactívalo también en la configuración de atribución de `.claude/settings.json` para que no dependa de recordarlo. Mensajes de commit en formato Conventional Commits (`feat:`, `fix:`, `test:`, `chore:`).
