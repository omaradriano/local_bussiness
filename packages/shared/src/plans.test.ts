import { describe, expect, it } from 'vitest';
import { FEATURES } from './verticals';
import { planCan, planComparison, planDef, TRIAL_DAYS } from './plans';

describe('plans', () => {
  it('reads the same catalog the API enforces', () => {
    expect(TRIAL_DAYS).toBe(14);
    expect(planDef('trial')).toEqual(planDef('pro'));
    expect(planDef('lite').limits.catalog_items).toBe(30);
    expect(planDef('business').limits.staff).toBeNull();
  });

  it('hides what Lite does not include', () => {
    expect(planCan('lite', 'expenses')).toBe(true);
    expect(planCan('lite', 'payroll')).toBe(false);
    expect(planCan('lite', 'recurring_expenses')).toBe(false);
    expect(planCan('trial', 'recurring_expenses')).toBe(true);
  });

  it('only excludes features that exist', () => {
    for (const p of ['lite', 'pro', 'business'] as const) {
      for (const f of planDef(p).excluded_features) expect(FEATURES).toContain(f);
    }
  });
});

describe('planComparison', () => {
  const rows = planComparison();
  const cell = (label: string) => rows.find((r) => r.label === label)!.cells;
  it('limits side by side', () => {
    expect(cell('Negocios')).toEqual({ lite: '1', pro: '1', business: '5' });
    expect(cell('Usuarios')).toEqual({ lite: '1', pro: '5', business: 'Ilimitado' });
  });
  it('the team is Pro and Business only', () => {
    expect(cell('Equipo (personas)')).toEqual({ lite: false, pro: '10', business: 'Ilimitado' });
    expect(cell('Sueldos y comisiones')).toEqual({ lite: false, pro: true, business: true });
  });
  it('every capability of any plan has a row', () => {
    expect(cell('Gastos')).toEqual({ lite: true, pro: true, business: true });
    expect(cell('Comparar negocios')).toEqual({ lite: false, pro: false, business: true });
  });
});
