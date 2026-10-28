// Dates as "YYYY-MM-DD" strings (no time zone involved): safe to compare as text.

const pad = (n: number) => String(n).padStart(2, '0');
const fmt = (d: Date) => `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}`;
const parse = (s: string) => new Date(`${s}T00:00:00Z`);

export function addDays(date: string, n: number): string {
  const d = parse(date);
  d.setUTCDate(d.getUTCDate() + n);
  return fmt(d);
}

export function firstOfMonth(date: string): string {
  return `${date.slice(0, 7)}-01`;
}

export function addMonths(monthStart: string, n: number): string {
  const d = parse(monthStart);
  d.setUTCMonth(d.getUTCMonth() + n, 1);
  return fmt(d);
}

export function lastOfMonth(monthStart: string): string {
  return addDays(addMonths(monthStart, 1), -1);
}

/** Weeks of the month, Monday first; days outside the month are null. */
export function monthGrid(monthStart: string): (string | null)[][] {
  const first = parse(firstOfMonth(monthStart));
  const lead = (first.getUTCDay() + 6) % 7; // Monday = 0
  const last = lastOfMonth(firstOfMonth(monthStart));
  const cells: (string | null)[] = Array(lead).fill(null);
  for (let d = firstOfMonth(monthStart); d <= last; d = addDays(d, 1)) cells.push(d);
  while (cells.length % 7) cells.push(null);
  const weeks: (string | null)[][] = [];
  for (let i = 0; i < cells.length; i += 7) weeks.push(cells.slice(i, i + 7));
  return weeks;
}

export function inRange(date: string, from: string | null, to: string | null): boolean {
  if (!from) return false;
  return date >= from && date <= (to ?? from);
}

const MONTHS = ['enero', 'febrero', 'marzo', 'abril', 'mayo', 'junio', 'julio', 'agosto', 'septiembre', 'octubre', 'noviembre', 'diciembre'];

export function monthLabel(monthStart: string): string {
  return `${MONTHS[Number(monthStart.slice(5, 7)) - 1]} ${monthStart.slice(0, 4)}`;
}

export const WEEKDAYS_SHORT = ['L', 'M', 'M', 'J', 'V', 'S', 'D'];
