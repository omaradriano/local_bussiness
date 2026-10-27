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
