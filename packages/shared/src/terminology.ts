export const VERTICALS = ['nails', 'flowers', 'tool_rental'] as const;
export type Vertical = (typeof VERTICALS)[number];

// null = the concept does not apply to that vertical (e.g. ends_at for flowers).
const TERMS = {
  booking: { nails: 'Cita', flowers: 'Pedido', tool_rental: 'Renta' },
  catalog_item: { nails: 'Servicio', flowers: 'Arreglo', tool_rental: 'Herramienta' },
  starts_at: { nails: 'Hora de la cita', flowers: 'Fecha de entrega', tool_rental: 'Fecha de salida' },
  ends_at: { nails: 'Hora de término', flowers: null, tool_rental: 'Fecha de regreso' },
} satisfies Record<string, Record<Vertical, string | null>>;

export type Term = keyof typeof TERMS;

export function term(vertical: Vertical, concept: Term): string | null {
  return TERMS[concept][vertical];
}
