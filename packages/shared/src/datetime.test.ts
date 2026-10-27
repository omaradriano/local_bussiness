import { describe, expect, it } from 'vitest';
import { hhmm, localToISO, todayISO } from './datetime';

describe('localToISO', () => {
  it('builds a local instant and round-trips the clock time', () => {
    const iso = localToISO('2026-11-02', '9:30')!;
    expect(iso).not.toBeNull();
    expect(hhmm(iso)).toBe('09:30');
    const d = new Date(iso);
    expect([d.getFullYear(), d.getMonth(), d.getDate()]).toEqual([2026, 10, 2]);
  });

  it.each([
    ['2026-02-30', '10:00'],
    ['02/11/2026', '10:00'],
    ['2026-11-02', '25:00'],
    ['2026-11-02', 'diez'],
    ['', ''],
  ])('rejects %s %s', (date, time) => expect(localToISO(date, time)).toBeNull());
});

describe('todayISO', () => {
  it('formats the local date', () => expect(todayISO(new Date(2026, 0, 5, 23, 0))).toBe('2026-01-05'));
});
