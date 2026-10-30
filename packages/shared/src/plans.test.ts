import { describe, expect, it } from 'vitest';
import { FEATURES } from './verticals';
import { planCan, planDef, planHighlights, TRIAL_DAYS } from './plans';

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

describe('planHighlights', () => {
  it('Lite: limits, no team, only expenses', () => {
    const h = planHighlights('lite');
    expect(h).toContain('1 negocio');
    expect(h).toContain('Hasta 30 artículos en el catálogo');
    expect(h).toContain('Gastos');
    expect(h.some((x) => x.startsWith('Equipo'))).toBe(false);
    expect(h).not.toContain('Sueldos y comisiones');
  });
  it('Pro: team and payroll', () => {
    const h = planHighlights('pro');
    expect(h).toContain('Equipo: hasta 10 personas');
    expect(h).toContain('Sueldos y comisiones');
    expect(h).toContain('Hasta 5 usuarios');
  });
  it('Business: unlimited and compare businesses', () => {
    const h = planHighlights('business');
    expect(h).toContain('Hasta 5 negocios');
    expect(h).toContain('Usuarios ilimitados');
    expect(h).toContain('Equipo sin límite de personas');
    expect(h).toContain('Comparar negocios');
  });
});
