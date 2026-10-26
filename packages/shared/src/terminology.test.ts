import { describe, expect, it } from 'vitest';
import { VERTICALS, term } from './terminology';

describe('term', () => {
  it('uses each vertical vocabulary for booking', () => {
    expect(term('nails', 'booking')).toBe('Cita');
    expect(term('flowers', 'booking')).toBe('Pedido');
    expect(term('tool_rental', 'booking')).toBe('Renta');
  });

  it('returns null where a concept does not apply', () => {
    expect(term('flowers', 'ends_at')).toBeNull();
  });

  it('labels catalog items and start dates for every vertical', () => {
    for (const v of VERTICALS) {
      expect(term(v, 'catalog_item')).toBeTruthy();
      expect(term(v, 'starts_at')).toBeTruthy();
    }
  });
});
