import { describe, expect, it } from 'vitest';
import flowersApi from '../../../../services/api/internal/businesses/templates/flowers.json';
import nailsApi from '../../../../services/api/internal/businesses/templates/nails.json';
import rentalApi from '../../../../services/api/internal/businesses/templates/tool_rental.json';
import { VERTICALS, hasFeature, kindFor, settingsSectionsFor, term, textsFor, verticalMap, type Vertical } from './index';

describe('terms', () => {
  it('uses each giro vocabulary', () => {
    expect(term('nails', 'booking')).toBe('Cita');
    expect(term('flowers', 'booking')).toBe('Pedido');
    expect(term('tool_rental', 'booking')).toBe('Renta');
    expect(term('flowers', 'ends_at')).toBeNull(); // no end for a delivery
  });
});

describe('texts', () => {
  it('every giro names its bookings and every state', () => {
    for (const v of VERTICALS) {
      expect(textsFor(v).bookings.plural).toBeTruthy();
      for (const label of Object.values(textsFor(v).bookings.status)) expect(label).toBeTruthy();
    }
  });

  it('uses each giro word and gender', () => {
    expect(textsFor('nails').bookings.status.confirmed).toMatch(/^Confirmada/);
    expect(textsFor('flowers').bookings.status.confirmed).toMatch(/^Confirmado/);
  });

  it('rental words stay in rentals', () => {
    for (const v of VERTICALS.filter((x) => kindFor(x) !== 'rental')) {
      expect(JSON.stringify(verticalMap(v))).not.toMatch(/renta|herramienta/i);
    }
  });
});

describe('Ajustes and features', () => {
  it('every giro starts with the shared section', () => {
    for (const v of VERTICALS) expect(settingsSectionsFor(v)[0]).toBe('general');
  });

  it('rental parts only where they apply', () => {
    expect(settingsSectionsFor('tool_rental')).toEqual(['general', 'rentals', 'inspection', 'documents']);
    expect(settingsSectionsFor('flowers')).toContain('delivery');
    expect(hasFeature('tool_rental', 'guarantor')).toBe(true);
    expect(hasFeature('nails', 'guarantor')).toBe(false);
    expect(hasFeature('nails', 'staff')).toBe(true);
  });
});

// The API reads its side of each map from its template; both must say the same.
describe('maps match the API templates', () => {
  const api: Record<Vertical, { kind: string; features: string[] }> = { nails: nailsApi, flowers: flowersApi, tool_rental: rentalApi };
  it.each(VERTICALS)('%s: same engine and features', (v) => {
    expect(api[v].kind).toBe(kindFor(v));
    expect([...api[v].features].sort()).toEqual([...verticalMap(v).features].sort());
  });
});
