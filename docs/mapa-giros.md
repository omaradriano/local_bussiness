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

## Mapa de barbería (`barbershop`)

Requerimientos del cliente (2026-10-28).

| Parte | Definición |
| --- | --- |
| Motor | `service` (agenda por barbero; duración por servicio) |
| Terminología | Cita · Servicio · Servicios · Hora de la cita · Hora de término |
| Estados | Confirmada · por atender → Atendida (sin paso «Iniciar»); Cancelada, No llegó. «Falta anticipo» solo en citas agendadas por internet |
| Ajustes | Anticipo y apartados (anticipo **solo en reservas por internet**) · Agenda: **margen entre citas para todo el negocio** |
| Valores por defecto | Sin anticipo en mostrador, apartado de 2 h, margen de 5 min, lunes a sábado 10:00–20:00 |
| Funciones encendidas | Equipo · Clientes casuales (agendar sin registrar) |
| Funciones ofrecidas, apagadas | Servicio a domicilio (el barbero la enciende en Ajustes si la da) |
| Sin | Aval, documentos del cliente, documentos adicionales, dirección obligatoria |
| Catálogo de ejemplo | Corte de cabello, Corte y barba, Arreglo de barba, Corte infantil |

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
5. **Motores nuevos** según los giros que entren: modalidad a domicilio sobre `service` (lavado de salas, autos a domicilio), `sale` sin fecha (tienda) y membresías (gimnasio). Antes del cuarto motor, aplicar «Motores detrás de una interfaz» (abajo).

## Motores detrás de una interfaz (futuro)

**Hoy:** cada motor es un `switch`/`if kind == "rental"` repartido en Go (`bookings/create.go`, `quote.go`, `ops.go`, `get.go`, `availability/query.go`, `catalog/catalog.go`, `httpapi/router.go`, más su tabla en `machine.go`) y en ~15 archivos del cliente (calendario, nueva reserva, detalle, catálogo, finanzas, `agenda.ts`, `period.ts`). Con tres motores funciona; un cuarto toca todos esos lugares.

**Objetivo:** agregar un motor = implementar una interfaz y registrarla, igual que un tipo de paso (`internal/steps`). Ninguna pantalla ni handler vuelve a preguntar por el motor; preguntan al registro.

**Cuándo:** al entrar el primer cliente que necesite un cuarto motor (tienda o gimnasio), no antes.

### Go: `internal/engines`

```go
type Engine interface {
	Kind() string                                    // "service", "product", "rental"…
	Transitions() map[string][]string                // su tabla de estados
	ValidateItem(item catalog.Item) error            // campos obligatorios del artículo (duración, unidades, existencias…)
	Availability(ctx context.Context, q Query) (Answer, error)
	Quote(q QuoteInput) (Quote, error)               // precio, anticipo y depósito en centavos
	Reserve(ctx context.Context, tx pgx.Tx, b Booking) error // apartar: horario, cupo, unidades o existencias
	Release(ctx context.Context, tx pgx.Tx, b Booking) error // devolverlo al cancelar o expirar
}

var registry = map[string]Engine{} // Register(e) desde cada motor; Get(kind) en handlers
```

- `machine.go` sigue siendo la única puerta de los estados (`CanTransition`), pero lee la tabla de `engines.Get(kind).Transitions()`.
- Los handlers dejan de tener `switch kind`: `engines.Get(item.Kind).Quote(…)`.
- Cada motor vive en su carpeta (`engines/service`, `engines/rental`…) con sus tests.

### Cliente: `packages/shared/src/engines/<motor>.ts`

```ts
export const rental = {
  kind: 'rental',
  period: 'range',          // 'slot' | 'day' | 'range' | 'none': qué pide el calendario y la nueva reserva
  resource: 'units',        // 'person' | 'capacity' | 'units' | 'stock' | 'none'
  pricing: 'perDay',        // 'fixed' | 'perDay' | 'recurring'
  itemFields: ['units', 'securityDeposit'],
  detailFlow: ['confirm', 'checkout', 'inUse', 'close'], // secciones del detalle, cada una un componente registrado
} satisfies EngineMap;
```

- Las pantallas usan `engineFor(kind).period` en vez de `kind === 'rental'`; TypeScript exige que el mapa esté completo.
- Las secciones del detalle se registran como los `stepRenderers`: un motor nuevo reusa las existentes y solo agrega las suyas.
- Un test verifica que los motores de TS y de Go sean los mismos (como `maps match the API templates`).

### Piezas combinables (lo que lo hace barato)

Un motor es una combinación de **tiempo × recurso × cobro**. Cada pieza se escribe una vez, con sus tests, y los motores las reúsan:

| Motor | Tiempo | Recurso | Cobro |
| --- | --- | --- | --- |
| `service` | horario (`slot`) | persona | fijo |
| `product` | día | cupo diario | fijo |
| `rental` | rango | unidades | por día |
| `sale` (tienda, POS) | ninguno | existencias | fijo |
| `membership` (gimnasio) | rango (vigencia) | ninguno | recurrente |
| clases con cupo | horario | cupo por horario | fijo o membresía |

Así, `sale` solo agrega la pieza «existencias» (que ya existe en `productSales`) y `membership` solo agrega «cobro recurrente»; el resto es configuración.

### Pasos de migración

1. Extraer la interfaz con los tres motores actuales, **sin cambiar comportamiento**: los tests existentes son la red.
2. Mover uno a uno los `switch` de Go al registro; luego los del cliente a `engineFor()`.
3. Agregar el test de paridad Go ↔ TS y actualizar la regla de `CLAUDE.md` («la máquina de estados vive solo en `machine.go`») para decir que las tablas viven en cada motor y `machine.go` es la única puerta.
4. Recién entonces, el cuarto motor.

## Vista pública y landing personalizada

- **Vista genérica por giro (base, incluida en todos los planes):** catálogo `/<slug>` armado con la plantilla visual del giro, con logo, colores y portada del negocio.
- **Landing personalizada (servicio extra, de pago):** una página de inicio propia para el negocio que lo contrate. Dos caminos, a decidir al llegar a la fase 6:
  - **Bloques configurables:** secciones predefinidas (portada, galería, testimonios, mapa, preguntas, llamada a reservar) que se ordenan y llenan desde el panel y se guardan como JSON; la misma app las pinta.
  - **Página a la medida:** diseño hecho para el cliente que reutiliza los componentes de reserva del catálogo.
  - En ambos casos: dominio propio opcional (`www.su-negocio.com`) apuntando a su página, y la reserva siempre pasa por el mismo flujo del catálogo.
