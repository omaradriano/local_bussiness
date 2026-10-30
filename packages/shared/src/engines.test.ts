import { describe, expect, it } from 'vitest';
import modules from '../../../services/api/internal/engines/engines.json';
import { engineFor } from './engines';
import type { ItemKind } from './verticals/types';

const kinds: ItemKind[] = ['service', 'product', 'rental'];
const allowed = {
  timing: ['slot', 'day', 'range'],
  fulfillment: ['atPlaceOrHome', 'pickupOrDelivery', 'atPlace'],
  resource: ['staff', 'dailyCapacity', 'units'],
  pricing: ['fixed', 'perPeriod'],
};

describe('engines', () => {
  it('the API engines are exactly ItemKind', () => {
    expect(Object.keys(modules).sort()).toEqual([...kinds].sort());
  });
  it('every engine names known modules', () => {
    for (const k of kinds) {
      const m = engineFor(k);
      for (const [module, names] of Object.entries(allowed)) expect(names).toContain(m[module as keyof typeof allowed]);
      expect(typeof m.custody).toBe('boolean');
    }
  });
  it('today: appointments by slot, orders by day, rentals by range with custody', () => {
    expect(engineFor('service')).toMatchObject({ timing: 'slot', resource: 'staff', custody: false });
    expect(engineFor('product')).toMatchObject({ timing: 'day', fulfillment: 'pickupOrDelivery' });
    expect(engineFor('rental')).toMatchObject({ timing: 'range', resource: 'units', pricing: 'perPeriod', custody: true });
  });
});
