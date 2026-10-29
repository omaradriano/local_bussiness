import { describe, expect, it } from 'vitest';
import { FEATURES } from './verticals';
import { planCan, planDef, TRIAL_DAYS } from './plans';

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
