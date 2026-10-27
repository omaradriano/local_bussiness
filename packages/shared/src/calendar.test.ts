import { describe, expect, it } from 'vitest';
import { addDays, addMonths, inRange, lastOfMonth, monthGrid, monthLabel } from './calendar';

describe('calendar', () => {
  it('builds a Monday-first grid', () => {
    const weeks = monthGrid('2026-11-01'); // Nov 1 2026 is a Sunday
    expect(weeks[0]).toEqual([null, null, null, null, null, null, '2026-11-01']);
    expect(weeks[1][0]).toBe('2026-11-02');
    expect(weeks.flat().filter(Boolean)).toHaveLength(30);
    expect(weeks.every((w) => w.length === 7)).toBe(true);
  });

  it('handles month and leap-year edges', () => {
    expect(addDays('2026-12-31', 1)).toBe('2027-01-01');
    expect(addDays('2028-03-01', -1)).toBe('2028-02-29');
    expect(addMonths('2026-12-01', 1)).toBe('2027-01-01');
    expect(addMonths('2026-01-01', -1)).toBe('2025-12-01');
    expect(lastOfMonth('2027-02-01')).toBe('2027-02-28');
  });

  it('checks ranges (end optional)', () => {
    expect(inRange('2026-11-03', '2026-11-02', '2026-11-04')).toBe(true);
    expect(inRange('2026-11-05', '2026-11-02', '2026-11-04')).toBe(false);
    expect(inRange('2026-11-02', '2026-11-02', null)).toBe(true);
    expect(inRange('2026-11-02', null, null)).toBe(false);
  });

  it('labels months in Spanish', () => expect(monthLabel('2026-11-01')).toBe('noviembre 2026'));
});
