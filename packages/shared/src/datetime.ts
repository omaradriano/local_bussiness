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
