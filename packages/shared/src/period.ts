import { addDays } from './calendar';

// Periods for the finances screens: a week (Monday to Sunday), a month, a year, or a custom range.
// Dates are "YYYY-MM-DD".

export type PeriodKind = 'week' | 'month' | 'year' | 'range';

const pad = (n: number) => String(n).padStart(2, '0');
const parts = (d: string) => d.split('-').map(Number) as [number, number, number];

/** The week, month or year that contains `day`. */
export function periodOf(kind: Exclude<PeriodKind, 'range'>, day: string): { from: string; to: string } {
  const [y, m, dd] = parts(day);
  if (kind === 'year') return { from: `${y}-01-01`, to: `${y}-12-31` };
  if (kind === 'month') {
    const last = new Date(Date.UTC(y, m, 0)).getUTCDate();
    return { from: `${y}-${pad(m)}-01`, to: `${y}-${pad(m)}-${pad(last)}` };
  }
  const weekday = (new Date(Date.UTC(y, m - 1, dd)).getUTCDay() + 6) % 7; // Monday = 0
  const from = addDays(day, -weekday);
  return { from, to: addDays(from, 6) };
}

/** The previous (-1) or next (+1) period of the same kind. */
export function shiftPeriod(kind: Exclude<PeriodKind, 'range'>, from: string, step: -1 | 1): { from: string; to: string } {
  const [y, m] = parts(from);
  if (kind === 'year') return periodOf('year', `${y + step}-01-01`);
  if (kind === 'month') {
    const d = new Date(Date.UTC(y, m - 1 + step, 1));
    return periodOf('month', `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-01`);
  }
  return periodOf('week', addDays(from, 7 * step));
}

/** "nov 2026", "2026", "3 – 9 nov" or "1 oct – 15 nov 2026". */
export function periodLabel(kind: PeriodKind, from: string, to: string): string {
  const fmt = (d: string, opts: Intl.DateTimeFormatOptions) => new Date(`${d}T12:00:00Z`).toLocaleDateString('es-MX', { timeZone: 'UTC', ...opts });
  if (kind === 'year') return from.slice(0, 4);
  if (kind === 'month') return fmt(from, { month: 'long', year: 'numeric' });
  if (from.slice(0, 7) === to.slice(0, 7)) return `${Number(from.slice(8))} – ${fmt(to, { day: 'numeric', month: 'short' })}`;
  return `${fmt(from, { day: 'numeric', month: 'short' })} – ${fmt(to, { day: 'numeric', month: 'short', year: 'numeric' })}`;
}
