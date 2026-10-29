import { describe, expect, it } from 'vitest';
import { periodLabel, periodOf, shiftPeriod } from './period';

describe('periods', () => {
  it('week is Monday to Sunday', () => {
    expect(periodOf('week', '2026-11-05')).toEqual({ from: '2026-11-02', to: '2026-11-08' }); // Thursday
    expect(periodOf('week', '2026-11-08')).toEqual({ from: '2026-11-02', to: '2026-11-08' }); // Sunday
    expect(periodOf('week', '2026-12-30')).toEqual({ from: '2026-12-28', to: '2027-01-03' }); // crosses the year
  });

  it('month and year', () => {
    expect(periodOf('month', '2028-02-10')).toEqual({ from: '2028-02-01', to: '2028-02-29' }); // leap year
    expect(periodOf('year', '2026-07-04')).toEqual({ from: '2026-01-01', to: '2026-12-31' });
  });

  it('moves to the previous and next period', () => {
    expect(shiftPeriod('month', '2026-01-01', -1)).toEqual({ from: '2025-12-01', to: '2025-12-31' });
    expect(shiftPeriod('week', '2026-11-02', 1)).toEqual({ from: '2026-11-09', to: '2026-11-15' });
    expect(shiftPeriod('year', '2026-01-01', 1)).toEqual({ from: '2027-01-01', to: '2027-12-31' });
  });

  it('labels', () => {
    expect(periodLabel('year', '2026-01-01', '2026-12-31')).toBe('2026');
    expect(periodLabel('week', '2026-11-02', '2026-11-08')).toMatch(/^2 – 8 nov/);
  });
});
