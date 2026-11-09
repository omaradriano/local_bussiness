export type Shift = { weekday: number; opens_at: string; closes_at: string };

/** One row per weekday (0 = domingo) as the editor shows it: the first shift is editable, the rest are kept as-is. */
export type DayRow = { open: boolean; opens_at: string; closes_at: string; extra: Shift[] };

export function shiftsToWeek(shifts: Shift[], defaults = { opens_at: '10:00', closes_at: '19:00' }): DayRow[] {
  return Array.from({ length: 7 }, (_, d) => {
    const day = shifts.filter((s) => s.weekday === d).sort((a, b) => a.opens_at.localeCompare(b.opens_at));
    const [first, ...extra] = day;
    return first
      ? { open: true, opens_at: first.opens_at, closes_at: first.closes_at, extra }
      : { open: false, ...defaults, extra: [] };
  });
}

export function weekToShifts(week: DayRow[]): Shift[] {
  return week.flatMap((row, d) =>
    row.open ? [{ weekday: d, opens_at: row.opens_at, closes_at: row.closes_at }, ...row.extra] : [],
  );
}

/** "9:5" / "0930" / "9:30" -> "09:30"; null when it isn't a valid 24h time. */
export function normalizeTime(input: string): string | null {
  const m = /^(\d{1,2}):?(\d{2})$/.exec(input.trim());
  if (!m) return null;
  const h = Number(m[1]);
  const min = Number(m[2]);
  if (h > 23 || min > 59) return null;
  return `${String(h).padStart(2, '0')}:${String(min).padStart(2, '0')}`;
}

const toMin = (t: string) => Number(t.slice(0, 2)) * 60 + Number(t.slice(3, 5));
const toHHMM = (m: number) => `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`;

/**
 * A default start time inside the business hours of `date` ("YYYY-MM-DD"): `time` if it already fits
 * (and, today, isn't past), otherwise the first open time from the next full hour on; '' when that day
 * has no time left. Without hours set it only avoids the past, like futureTime.
 */
export function timeInHours(date: string, time: string, shifts: Shift[], now = new Date()): string {
  const [y, mo, d] = date.split('-').map(Number);
  const day = new Date(y, mo - 1, d);
  const today = day.getFullYear() === now.getFullYear() && day.getMonth() === now.getMonth() && day.getDate() === now.getDate();
  const earliest = today ? (now.getHours() + 1) * 60 : 0;
  const open = shifts.filter((s) => s.weekday === day.getDay()).sort((a, b) => toMin(a.opens_at) - toMin(b.opens_at));
  const t = normalizeTime(time);
  if (!shifts.length) return t && toMin(t) >= earliest ? t : earliest < 24 * 60 ? toHHMM(earliest) : '';
  if (t && toMin(t) >= earliest && open.some((s) => toMin(t) >= toMin(s.opens_at) && toMin(t) < toMin(s.closes_at))) return t;
  for (const s of open) {
    const m = Math.max(toMin(s.opens_at), earliest);
    if (m < toMin(s.closes_at)) return toHHMM(m);
  }
  return '';
}
