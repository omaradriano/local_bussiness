import { describe, expect, it } from 'vitest';
import { productionSheet } from './production';

const row = (id: string, status: string, starts_at: string, items: string, details = '') => ({ id, status, starts_at, items, details });

describe('productionSheet', () => {
  it('adds up what is still to make, by arrangement, and lists the orders by time', () => {
    const sheet = productionSheet([
      row('a', 'confirmed', '2026-02-14T18:00:00Z', '2 × Ramo de 12 rosas'),
      row('b', 'pending_deposit', '2026-02-14T16:00:00Z', '1 × Ramo de 12 rosas'),
      row('c', 'in_progress', '2026-02-14T17:00:00Z', '1 × Arreglo personalizado', 'Girasoles'),
      row('d', 'ready', '2026-02-14T15:00:00Z', '3 × Arreglo en caja'),
      row('x', 'cancelled', '2026-02-14T15:00:00Z', '5 × Ramo de 12 rosas'),
    ]);
    expect(sheet.totals).toEqual([
      { name: 'Ramo de 12 rosas', quantity: 3 },
      { name: 'Arreglo personalizado', quantity: 1 },
    ]);
    expect(sheet.toMake.map((r) => r.id)).toEqual(['b', 'c', 'a']);
    expect(sheet.done.map((r) => r.id)).toEqual(['d']);
  });

  it('reads an item without a quantity as one', () => {
    expect(productionSheet([row('a', 'confirmed', '2026-02-14T18:00:00Z', 'Ramo')]).totals).toEqual([{ name: 'Ramo', quantity: 1 }]);
  });
});
