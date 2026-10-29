/**
 * CSV for the owner's exports (and the importer's template): UTF-8 with BOM and CRLF so Excel opens
 * accents and columns right; amounts in pesos, dates in the business's time zone.
 */
export type Cell = string | number | boolean | null | undefined;
export type Column<T> = { header: string; value: (row: T) => Cell };

const BOM = '﻿';

function escape(v: Cell): string {
  if (v === null || v === undefined) return '';
  const s = typeof v === 'boolean' ? (v ? 'Sí' : 'No') : String(v);
  return /[",\r\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

export function toCSV<T>(columns: Column<T>[], rows: T[]): string {
  const lines = [columns.map((c) => escape(c.header)), ...rows.map((r) => columns.map((c) => escape(c.value(r))))];
  return BOM + lines.map((l) => l.join(',')).join('\r\n') + '\r\n';
}

/** Centavos → "1234.50" (no thousands separator: spreadsheets read it as a number). */
export const pesos = (centavos: number | null | undefined): string => (centavos == null ? '' : (centavos / 100).toFixed(2));

function parts(iso: string, timeZone: string) {
  const f = new Intl.DateTimeFormat('en-CA', { timeZone, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hourCycle: 'h23' });
  return Object.fromEntries(f.formatToParts(new Date(iso)).map((p) => [p.type, p.value]));
}

/** An instant as "YYYY-MM-DD HH:mm" in the business's time zone. */
export function localDateTime(iso: string | null | undefined, timeZone: string): string {
  if (!iso) return '';
  const p = parts(iso, timeZone);
  return `${p.year}-${p.month}-${p.day} ${p.hour}:${p.minute}`;
}

/** A calendar date (midnight UTC from the API) as "YYYY-MM-DD". */
export const dateOnly = (iso: string | null | undefined): string => (iso ? iso.slice(0, 10) : '');
