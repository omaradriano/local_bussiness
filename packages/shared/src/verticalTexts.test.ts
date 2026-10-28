import { describe, expect, it } from 'vitest';
import { VERTICALS } from './terminology';
import { settingsSectionsFor, textsFor } from './verticalTexts';

describe('texts per vertical', () => {
  it('every vertical names its bookings and every state', () => {
    for (const v of VERTICALS) {
      const t = textsFor(v);
      expect(t.bookings.plural).toBeTruthy();
      for (const label of Object.values(t.bookings.status)) expect(label).toBeTruthy();
    }
  });

  it('uses each vertical word and gender', () => {
    expect(textsFor('tool_rental').bookings.plural).toBe('Rentas');
    expect(textsFor('nails').bookings.status.confirmed).toMatch(/^Confirmada/);
    expect(textsFor('flowers').bookings.status.confirmed).toMatch(/^Confirmado/);
  });

  it('rental words stay in rentals', () => {
    for (const v of VERTICALS.filter((x) => x !== 'tool_rental')) {
      expect(JSON.stringify(textsFor(v))).not.toMatch(/renta|herramienta/i);
    }
  });
});

describe('Ajustes sections per vertical', () => {
  it('every business has the shared section first', () => {
    for (const v of VERTICALS) expect(settingsSectionsFor(v)[0]).toBe('general');
  });

  it('rental and delivery sections only where they apply', () => {
    expect(settingsSectionsFor('tool_rental')).toEqual(['general', 'rentals', 'inspection', 'documents']);
    expect(settingsSectionsFor('nails')).toEqual(['general']);
    expect(settingsSectionsFor('flowers')).toContain('delivery');
    expect(settingsSectionsFor('flowers')).not.toContain('rentals');
  });
});
