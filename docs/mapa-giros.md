# Mapa por giro

Cómo se define cada tipo de negocio (renta de herramienta, uñas, florería, lavado de autos, lavado de salas, tienda…) para que la plataforma se adapte a muchos giros sin reescribir pantallas, y qué parte configura el superadministrador y qué parte el dueño.

## Tres niveles

| Nivel | Qué es | Quién lo define | Dónde vive |
| --- | --- | --- | --- |
| **Motor** | Cómo se aparta algo: agenda por horario, entrega con fecha, renta por unidades, venta sin fecha. Reglas de disponibilidad, estados, precio, pagos. | Código (Go + tests) | `services/api/internal/{availability,bookings}` |
| **Giro** | Un tipo de negocio montado sobre un motor: cómo se llama todo, qué estados usa, qué ajustes y funciones trae, valores por defecto, catálogo y pasos de ejemplo, vista pública. | Superadministrador | Hoy en código; después en la tabla `business_types` |
| **Negocio** | Un cliente concreto: sus ajustes, horario, catálogo, pasos y documentos. | Dueño (Ajustes); el superadministrador puede activar funciones o corregir | `businesses.settings`, `workflow_steps`, catálogo |

Regla: **un giro nuevo no debe requerir código** si cabe en un motor existente. Solo un motor nuevo (otra forma de apartar) requiere código.

## Motores

| Motor | Cómo se aparta | Ejemplos | Estado |
| --- | --- | --- | --- |
| `service` en el local | Horario de una persona (duración + margen) | Uñas, barbería, lavado de autos en el local | Hecho (fase 3) |
| `service` a domicilio | Igual que el anterior + dirección del cliente, zona y tiempo de traslado entre citas | Lavado de salas, lavado de autos a domicilio | **Falta**: modalidad "a domicilio" sobre `service` |
| `product` con fecha | Cupo por día y anticipación; entrega o recoger | Florería, pasteles | Hecho (fase 3); falta el armador de producto |
| `rental` | Unidades físicas por rango de fechas | Renta de herramienta, de mobiliario, de vestidos | Hecho; es el primer mapa completo |
| `sale` sin fecha | Existencias (stock), sin calendario; recoger o envío | Tienda de productos | **Falta**: motor nuevo |

La máquina de estados ya es por motor (`internal/bookings/machine.go`): un giro solo elige textos para los estados de su motor.

## Qué define un mapa de giro

1. **Identidad:** clave (`tool_rental`), nombre para el alta («Renta de herramienta»), motor.
2. **Terminología:** reserva («Renta» / «Cita» / «Pedido»), artículo, catálogo, inicio y fin («Fecha de salida» / «Hora de la cita»).
3. **Textos:** plural, estados con su concordancia, ayudas del inicio, de nueva reserva y del catálogo. Hoy: `packages/shared/src/verticalTexts.ts`.
4. **Ajustes:** secciones que ve el dueño y sus valores por defecto. Hoy: `settingsSectionsFor()` + `settings` de la plantilla.
5. **Funciones:** qué partes de la app trae encendidas (aval y documentos del cliente, inventario, extender, recargos, zonas de entrega, equipo/empleadas, armador de producto…). Hoy son condiciones por motor en el cliente; pasarán a `features` del giro con override por negocio (`settings.features`).
6. **Plantilla de alta:** catálogo, horario y pasos de ejemplo. Hoy: `services/api/internal/businesses/templates/<giro>.json`.
7. **Flujo del detalle:** pasos numerados que ve el dueño al operar una reserva. Hoy solo existe el de renta.
8. **Avisos:** plantillas de mensajes por evento (fase 4, `notification_templates` por giro).
9. **Vista pública:** plantilla genérica del catálogo para ese giro (fase 6) y, si el negocio lo contrata, landing personalizada (ver abajo).

## Mapa de renta de herramienta (`tool_rental`) — primer mapa completo

| Parte | Definición |
| --- | --- |
| Motor | `rental` (unidades por rango de fechas, precio por día) |
| Terminología | Renta · Herramienta · Herramientas · Fecha de salida · Fecha de regreso |
| Estados | Apartada · falta anticipo → Agendada · por salir → Rentada · con el cliente → (Vencida · no ha regresado) → Devuelta · por cerrar → Cerrada; Cancelada, Expirada, No se presentó |
| Ajustes | Anticipo y apartados (compartida) · Rentas: depósito de garantía, multiplicador por día de retraso, días cerrados (pasar y cobrar) · Revisión entre rentas (horas) · Documentos adicionales (p. ej. Pagaré) |
| Valores por defecto | Anticipo 0 %, apartado 24 h, depósito activo, multiplicador ×1, pasar y cobrar días cerrados, sin revisión |
| Funciones | Unidades con código y estado · Inventario con filtros · Clientes con dirección, aval, INE, comprobante y «de confianza» · Documentos reutilizados de la ficha · Documentos adicionales configurables y libres (foto o PDF) · Extender renta · Recargo por retraso y cargo por daño solo tras la devolución · Evidencia de pago |
| Pasos de ejemplo | Foto del INE · Comprobante de domicilio (de la ficha) · Pagaré (apagado) · Fotos al salir · Fotos al regresar |
| Flujo del detalle | 1 Confirmación · 2 Salida (documentos, adicionales, fotos, cobro, entregar) · 3 En renta y devolución (extender, fotos, recibir) · 4 Cierre (cobros finales y depósito, completar) |
| Vista pública | Pendiente (fase 6) |

## Mapa de barbería (`barbershop`) — mapa base

| Parte | Definición |
| --- | --- |
| Motor | `service` (agenda por barbero: duración y margen por servicio) |
| Terminología | Cita · Servicio · Servicios · Hora de la cita · Hora de término |
| Estados | Apartada · falta anticipo → Confirmada · por atender → En la silla → Atendida; Cancelada, Expirada, No llegó |
| Ajustes | Anticipo y apartados |
| Valores por defecto | Sin anticipo, apartado de 2 h, lunes a sábado 10:00–20:00 |
| Funciones | Equipo (barberos con sus servicios y horario) |
| Catálogo de ejemplo | Corte de cabello, Corte y barba, Arreglo de barba, Corte infantil |
| Pendiente | Requerimientos del cliente: flujo del detalle, lista de espera o turnos sin cita, propinas, etc. |

Los mapas de uñas (`nails`, motor `service`) y florería (`flowers`, motor `product`) ya tienen terminología, textos y secciones de Ajustes; les falta su flujo del detalle y sus funciones propias (equipo y servicios por persona en uñas; armador y zonas de entrega en flores).

## Dónde vive hoy cada giro

Desde 2026-10-28 cada giro es **un archivo de mapa** (`packages/shared/src/verticals/<giro>.ts`) y **una plantilla de la API** (`services/api/internal/businesses/templates/<giro>.json`) que repite su motor y sus funciones; un test verifica que coincidan. De ahí salen la lista de giros, el motor, la terminología, los textos, las secciones de Ajustes y las funciones; las pantallas y la API preguntan por funciones (`guarantor`, `inventory`, `staff`…), no por el giro. Lo único fijo que queda es el `check` de `businesses.vertical` en la base. Receta en `CLAUDE.md`.

## Funciones por negocio

- El **giro** trae funciones por defecto (`features` en su mapa y su plantilla).
- El **superadministrador** puede **dar** o **quitar** una función a un negocio concreto (`businesses.feature_grants`, que el dueño no puede cambiar), siempre que el motor la admita: por ejemplo, documentos del cliente para una barbería, pero nunca inventario, porque es solo de renta.
- El **dueño** enciende o apaga en Ajustes → Funciones las que tenga disponibles (`settings.features`).
- **Activa** = disponible y encendida. La API (por ejemplo, los campos obligatorios del cliente) y las pantallas (`has()`) usan solo las activas, que llegan en `/v1/me`.
- El catálogo de funciones y los motores que admite cada una está en `services/api/internal/features/features.json`, que también lee la app.

## Camino propuesto

1. **Fuente única en código (hecho, 2026-10-28).** Un archivo por giro con todo su mapa (identidad, motor, terminología, textos, secciones, valores por defecto, funciones, plantilla), del que salen la lista de giros, `kindFor`, los textos y la plantilla de alta. El cliente y la API dejan de preguntar «¿es `tool_rental`?» y preguntan «¿tiene la función `guarantor`?». Agregar un giro sobre un motor existente = agregar un archivo y sus tests.
2. **Funciones por negocio (hecho, 2026-10-28).** `settings.features` permite que el superadministrador encienda o apague una función para un negocio concreto, por encima de lo que trae su giro (por ejemplo, documentos del cliente en un lavado de salas).
3. **Panel de superadministrador** (ver plan): lista de negocios, su giro, plan, funciones y ajustes; entrar como el negocio con bitácora de auditoría.
4. **Giros en la base (`business_types`).** Cuando haya varios giros y el panel de superadministrador, los mapas pasan a una tabla editable desde ese panel, validada contra los motores; `businesses.vertical` apunta a esa tabla en lugar del `check`. Los motores, los tipos de paso y los tipos de sección de Ajustes siguen en código.
5. **Motores nuevos** según los giros que entren: modalidad a domicilio sobre `service` (lavado de salas, autos a domicilio) y `sale` sin fecha (tienda).

## Vista pública y landing personalizada

- **Vista genérica por giro (base, incluida en todos los planes):** catálogo `/<slug>` armado con la plantilla visual del giro, con logo, colores y portada del negocio.
- **Landing personalizada (servicio extra, de pago):** una página de inicio propia para el negocio que lo contrate. Dos caminos, a decidir al llegar a la fase 6:
  - **Bloques configurables:** secciones predefinidas (portada, galería, testimonios, mapa, preguntas, llamada a reservar) que se ordenan y llenan desde el panel y se guardan como JSON; la misma app las pinta.
  - **Página a la medida:** diseño hecho para el cliente que reutiliza los componentes de reserva del catálogo.
  - En ambos casos: dominio propio opcional (`www.su-negocio.com`) apuntando a su página, y la reserva siempre pasa por el mismo flujo del catálogo.
