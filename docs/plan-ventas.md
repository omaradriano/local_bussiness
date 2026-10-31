# Plan de implementación priorizado por ventas

Fecha: 2026-10-30. Sale del análisis de competencia (AgendaPro, Booksy, Fresha, ReservaSimple, Booqable, Eventrix…) y de lo que piden los pilotos. Ordena el trabajo por **lo que más ayuda a vender**, sobre la base que ya existe (motores por módulos, giros por mapa, rentas completas, finanzas, planes). Complementa `docs/plan.md`: las fases 4 y 6 de ese plan se adelantan en partes pequeñas aquí.

## Principios

1. **Primero lo que se ve en la venta**, después lo que cierra con equipos, después lo que abre mercados nuevos, al final lo que cobra.
2. **Cada ola deja algo que se puede enseñar** a un cliente y medir con los pilotos.
3. **Reusar módulos**: nada se construye «para un giro»; se arma con motores, funciones y campos personalizados.
4. **Sin costos por mensaje ni comisiones** hasta tener clientes que paguen: WhatsApp de un toque antes que la API de WhatsApp; transferencia con comprobante antes que cobro en línea.
5. Mismo método de siempre: partes pequeñas, cada una con sus tests, PR, staging y aprobación.

Tamaños (aproximados, de desarrollo): **S** 1–2 días · **M** 3–5 días · **L** 1–2 semanas.

## Ola 0 — En curso

| # | Qué | Tamaño | Vende porque… |
| --- | --- | --- | --- |
| 0.1 | **Campos personalizados** (texto, número, lista, fecha, sí/no; del artículo o de cada unidad), primero en rentas | M | El cliente de renta los pidió: número de serie, marca, modelo, condición. |

## Ola 1 — Lo que se ve en la venta (barberías, tatuajes, Instagram)

| # | Qué | Tamaño | Vende porque… |
| --- | --- | --- | --- |
| 1.1 | **Link de reservas público** `khronno.com/<negocio>` para **citas**: datos del negocio, servicios con precio y duración, elegir barbero (opcional), día y hora libres, nombre y teléfono, aviso de privacidad. Crea la cita con `source = public` y su enlace de estado (`public_token`). | L | Es lo primero que comparan con AgendaPro y Booksy, y lo que hace funcionar Instagram («link en la bio»). |
| 1.2 | **Anticipo en el link**: si el negocio pide anticipo, la cita queda apartada con vencimiento (ya existe) y el cliente ve los datos de transferencia del negocio (CLABE y titular en Ajustes) y puede subir la foto del comprobante; el dueño lo confirma con un toque. | M | Tatuadores y barberos pierden dinero con clientes que no llegan. Sin comisión por cobro. |
| 1.3 | **Compartir mi link**: en el panel, botón para copiar el link y **QR del negocio** imprimible (para el mostrador y las historias). | S | El dueño lo pone en su bio y en su local el mismo día. |
| 1.4 | **Recordatorio por WhatsApp de un toque**: en Hoy y en el detalle, «Recordar» abre WhatsApp con el mensaje armado (plantillas por giro en `packages/shared`); lista «Recordar mañana» con las citas del día siguiente. Primera parte de la fase 4, sin costo por mensaje. | S | Reduce faltas; es lo que más valora el dueño. |
| 1.5 | **Antispam del link**: límite de peticiones por IP (ya en el plan) y captcha invisible (Cloudflare Turnstile). | S | Necesario antes de publicar links. |

**Criterios de aceptación**

- [ ] Una cita hecha desde el link aparece en Hoy y en el calendario con el mismo flujo que una de mostrador; dos personas que eligen el mismo horario: una gana, la otra ve que ya no está libre.
- [ ] Con anticipo, la cita sin comprobante expira y libera el horario; con comprobante, el dueño la confirma desde el detalle.
- [ ] El link carga en menos de 2 s en móvil (Lighthouse ≥ 90 en rendimiento y accesibilidad).
- [ ] «Recordar» abre WhatsApp con nombre, servicio, día y hora correctos en la zona del negocio.

## Ola 2 — Lo que cierra la venta con equipos (y la prueba con pilotos)

| # | Qué | Tamaño | Vende porque… |
| --- | --- | --- | --- |
| 2.1 | **Usuarios con su propia cuenta**: el dueño invita por correo; rol `staff` ve su agenda y registra cobros y estados, sin Finanzas ni Ajustes; quitar acceso revoca la sesión; respeta el límite de usuarios del plan. | M | Los planes ya prometen «usuarios»; un negocio con barberos o mostrador lo pide. |
| 2.2 | **Link de reservas para rentas**: elegir herramienta, rango de fechas y unidades libres, con depósito y documentos que se completan al recoger. | M | Abre el canal en línea para rentas, donde casi no hay competencia. |
| 2.3 | **Nota o contrato de renta imprimible (PDF)**: datos del negocio, cliente, aval, piezas con su clave y campos personalizados, fechas, montos, depósito y texto de condiciones editable en Ajustes (con plantilla base). | M | Las rentas lo esperan; la competencia mexicana de rentas lo tiene. |
| 2.4 | **Uso de los pilotos** en el superadministrador: citas o rentas por semana, última actividad y funciones usadas por negocio. | S | Saber quién usa la app de verdad antes de cobrarle. |
| 2.5 | **Giro Tatuajes** (cuando haya un tatuador interesado): mapa y plantilla sobre el motor de citas, anticipo obligatorio, campos personalizados (zona del cuerpo, tamaño, foto de referencia). | S | Nicho con dolor claro (anticipos) y mucha presencia en Instagram. |

## Ola 3 — Abrir el mercado de rentas grandes (mobiliario y eventos)

| # | Qué | Tamaño | Vende porque… |
| --- | --- | --- | --- |
| 3.1 | **Varias cosas en una sola renta** (10 sillas + 2 mesas + carpa): reserva con varias líneas, disponibilidad combinada por fecha, cotización y salida o devolución por pieza. Cambio de motor: se escribe empezando por sus tests. | L | Sin esto no se puede entrar a mobiliario para eventos. |
| 3.2 | **Entrega y recolección a domicilio en rentas**: combinar el módulo de custodia con el de entrega por zona (ya existe en pedidos). | M | Mobiliario y equipo de fiesta se entregan en el evento. |
| 3.3 | **Paquetes** (combo de varias piezas con precio propio). | M | Así venden las rentadoras de eventos. |
| 3.4 | **Giro Mobiliario para eventos** sobre lo anterior. | S | Mercado nuevo con software local caro o de escritorio. |

## Ola 4 — Cobrar (cuando haya pilotos usando la app cada semana)

| # | Qué | Tamaño | Notas |
| --- | --- | --- | --- |
| 4.1 | **Cobro de la suscripción**: precios definitivos, pago con tarjeta recurrente (Mercado Pago o Stripe), avisos de pago vencido y solo lectura al vencer (ya existe). | M | Precios de lanzamiento sugeridos: Lite $199, Pro $449, Business $999 MXN al mes con IVA; «precio fundador» congelado para pilotos. |
| 4.2 | **Anticipo en línea** (link de pago de Mercado Pago conciliado con `payments`). | M | Cuando los negocios lo pidan; hasta entonces, transferencia con comprobante. |
| 4.3 | **Recordatorios automáticos por WhatsApp** (Cloud API) y aviso de renta vencida. | M | Tiene costo por mensaje: decidir si lo absorbe el plan. |

## Ola 5 — Confianza y escala (según la demanda)

- App en App Store y Play Store (EAS; cuentas de Apple y Google).
- Factura CFDI 4.0 con un PAC, cuando un cliente lo pida.
- Sentry, respaldos diarios y simulacro de restauración (ya en pendientes).
- Marca del negocio en su link (logo, colores, portada) y landing personalizada como servicio extra.

## En paralelo: ventas

| Semana | Producto | Ventas |
| --- | --- | --- |
| 1–2 | Ola 0 y 1.1 | Visitas en persona con la barbería demo y el QR; 2 pilotos de renta. |
| 3–4 | 1.2–1.5 | Instagram a barberos y tatuadores con su link funcionando; 2 pilotos de barbería. |
| 5–6 | Ola 2 | Primer tatuador; medir uso de pilotos (2.4). |
| 7–8 | Decidir precios con datos | 2 testimonios; preparar el cobro (4.1). |

Meta a 60 días: **5 pilotos activos** (2 rentas, 2 barberías, 1 tatuador), **3 con uso semanal** y **2 testimonios**.

## Decisiones que necesito antes de empezar cada ola

- **Ola 1:** el anticipo del link se paga por **transferencia con comprobante** (sin comisión) — recomendado; el cobro en línea queda para la ola 4.
- **Ola 2:** qué puede hacer un empleado (propuesta: su agenda, cobros y estados; sin Finanzas, Ajustes ni borrar). El **texto legal del contrato** de renta lo aporta el negocio o un abogado; la app da una plantilla editable sin valor legal garantizado.
- **Ola 3:** confirmar con un cliente real de mobiliario antes de construirla (es la más grande).
- **Ola 4:** precios definitivos y procesador de pagos.
