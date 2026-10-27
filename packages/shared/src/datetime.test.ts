import { describe, expect, it } from 'vitest';
import { bookingDays, hhmm, localToISO, monthWindow, todayISO } from './datetime';

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

describe('bookingDays', () => {
  const iso = (y: number, m: number, d: number, h = 0, mi = 0) => new Date(y, m - 1, d, h, mi).toISOString();
  it('spans every local day of a rental', () =>
    expect(bookingDays(iso(2026, 11, 2, 9), iso(2026, 11, 4, 18))).toEqual(['2026-11-02', '2026-11-03', '2026-11-04']));
  it('an end at midnight does not count that day', () =>
    expect(bookingDays(iso(2026, 11, 2, 9), iso(2026, 11, 4))).toEqual(['2026-11-02', '2026-11-03']));
  it('an order (no end) is its start day', () => expect(bookingDays(iso(2026, 11, 2, 18), null)).toEqual(['2026-11-02']));
});

describe('monthWindow', () => {
  it('covers the whole local month', () => {
    const w = monthWindow('2026-11-01');
    expect(new Date(w.from).getDate()).toBe(1);
    expect(new Date(w.to).getMonth()).toBe(11); // December
  });
});
