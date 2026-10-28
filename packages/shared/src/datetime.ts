import { normalizeTime } from './hours';

/** Today as YYYY-MM-DD in the device's time zone. */
export function todayISO(now = new Date()): string {
  const p = (n: number) => String(n).padStart(2, '0');
  return `${now.getFullYear()}-${p(now.getMonth() + 1)}-${p(now.getDate())}`;
}

/**
 * "2026-11-02" + "9:30" -> ISO instant, read in the device's time zone; null if either part is invalid
 * (including impossible dates like 2026-02-30).
 * shortcut: assumes the device is in the business time zone; pass the business zone once the API exposes it.
 */
export function localToISO(date: string, time: string): string | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date.trim());
  const t = normalizeTime(time);
  if (!m || !t) return null;
  const [y, mo, d] = [Number(m[1]), Number(m[2]), Number(m[3])];
  const [h, mi] = t.split(':').map(Number);
  const dt = new Date(y, mo - 1, d, h, mi);
  if (dt.getFullYear() !== y || dt.getMonth() !== mo - 1 || dt.getDate() !== d) return null;
  return dt.toISOString();
}

/** ISO instant -> "9:30" style local time for slot buttons. */
export function hhmm(iso: string): string {
  const d = new Date(iso);
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}

/** Local dates (YYYY-MM-DD) a booking touches, from its start to its end (an end at 00:00 doesn't count that day). */
export function bookingDays(startISO: string, endISO: string | null): string[] {
  const start = new Date(startISO);
  const end = endISO ? new Date(new Date(endISO).getTime() - 1) : start;
  const days: string[] = [];
  const d = new Date(start.getFullYear(), start.getMonth(), start.getDate());
  while (d <= end && days.length < 400) {
    days.push(todayISO(d));
    d.setDate(d.getDate() + 1);
  }
  return days.length ? days : [todayISO(start)];
}

/** [start of the month, start of next month) as ISO instants in the device time zone. */
export function monthWindow(monthStart: string): { from: string; to: string } {
  const [y, m] = monthStart.split('-').map(Number);
  return { from: new Date(y, m - 1, 1).toISOString(), to: new Date(y, m, 1).toISOString() };
}

/** The next full hour after `now` ("15:02" → "16:00"); 23:xx stays "23:00" (the last start of the day). */
export function nextHour(now = new Date()): string {
  const h = Math.min(now.getHours() + 1, 23);
  return `${String(h).padStart(2, '0')}:00`;
}

/**
 * A start time that isn't in the past: on today's date, a time at or before now becomes the next
 * full hour; other days (and unreadable times) are left as they are. After 23:00 there is no later
 * start that day, so it stays 23:00 (the API then refuses it).
 */
export function futureTime(date: string, time: string, now = new Date()): string {
  if (date !== todayISO(now)) return time;
  const t = normalizeTime(time);
  if (!t) return time;
  const [h, m] = t.split(':').map(Number);
  return h * 60 + m <= now.getHours() * 60 + now.getMinutes() ? nextHour(now) : t;
}
