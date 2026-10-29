import { toCSV } from './csv';
import { parsePesos } from './money';
import { strings } from './strings';

/**
 * Reading a CSV to import (customers, catalog, products): the columns are the export's (so an
 * exported file comes back as is), matched without caring about case, accents or spaces. Values
 * become what the API expects (centavos, numbers, true/false); the API validates every row.
 */
export type ImportKind = 'customers' | 'catalog' | 'products';

type FieldType = 'text' | 'money' | 'int' | 'bool' | 'priceUnit';
type Field = { key: string; header: string; type: FieldType; required?: boolean };

const h = strings.exports.headers;
const FIELDS: Record<ImportKind, Field[]> = {
  customers: [
    { key: 'name', header: h.name, type: 'text', required: true }, { key: 'phone', header: h.phone, type: 'text', required: true },
    { key: 'email', header: h.email, type: 'text' }, { key: 'address', header: h.address, type: 'text' },
    { key: 'guarantor_name', header: h.guarantor, type: 'text' }, { key: 'guarantor_phone', header: h.guarantorPhone, type: 'text' },
    { key: 'notes', header: h.notes, type: 'text' },
  ],
  catalog: [
    { key: 'name', header: h.name, type: 'text', required: true }, { key: 'price', header: h.price, type: 'money', required: true },
    { key: 'description', header: h.description, type: 'text' }, { key: 'duration_minutes', header: h.duration, type: 'int' },
    { key: 'price_unit', header: h.priceUnit, type: 'priceUnit' }, { key: 'security_deposit', header: h.deposit, type: 'money' },
    { key: 'late_fee_per_day', header: h.lateFee, type: 'money' }, { key: 'active', header: h.active, type: 'bool' },
  ],
  products: [
    { key: 'name', header: h.name, type: 'text', required: true }, { key: 'price', header: h.price, type: 'money', required: true },
    { key: 'cost', header: h.cost, type: 'money' }, { key: 'stock', header: h.stock, type: 'int' }, { key: 'active', header: h.active, type: 'bool' },
  ],
};

const norm = (s: string) => s.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase().replace(/\s+/g, ' ').trim();

/** CSV text → rows of cells. Handles the BOM, quotes, CRLF, and ";" files (Excel in some locales). */
export function parseCSV(text: string): string[][] {
  const src = text.replace(/^﻿/, '');
  const firstLine = src.split(/\r?\n/, 1)[0] ?? '';
  const sep = (firstLine.match(/;/g)?.length ?? 0) > (firstLine.match(/,/g)?.length ?? 0) ? ';' : ',';
  const rows: string[][] = [];
  let row: string[] = [];
  let cell = '';
  let quoted = false;
  for (let i = 0; i < src.length; i++) {
    const c = src[i];
    if (quoted) {
      if (c === '"' && src[i + 1] === '"') { cell += '"'; i++; } else if (c === '"') quoted = false; else cell += c;
    } else if (c === '"') quoted = true;
    else if (c === sep) { row.push(cell); cell = ''; }
    else if (c === '\n' || c === '\r') {
      if (c === '\r' && src[i + 1] === '\n') i++;
      row.push(cell); rows.push(row); row = []; cell = '';
    } else cell += c;
  }
  if (cell !== '' || row.length) { row.push(cell); rows.push(row); }
  return rows.filter((r) => r.some((c) => c.trim() !== ''));
}

const UNITS: Record<string, string> = Object.fromEntries(Object.entries(strings.catalog.priceUnits).map(([k, v]) => [norm(v), k]));

function value(type: FieldType, raw: string): unknown {
  const v = raw.trim();
  if (v === '') return undefined;
  switch (type) {
    case 'money': return parsePesos(v) ?? -1; // the API reports the bad amount for that row
    case 'int': return /^-?\d+$/.test(v) ? Number(v) : -1;
    case 'bool': return ['si', 'sí', 'yes', 'true', '1', 'activo'].includes(norm(v));
    case 'priceUnit': return UNITS[norm(v)] ?? v;
    default: return v;
  }
}

export type ParsedImport = { rows: Record<string, unknown>[]; missing: string[]; ignored: string[] };

/** The file's rows as the API's fields; `missing` lists required columns that aren't there. */
export function rowsFromCSV(kind: ImportKind, text: string): ParsedImport {
  const [header = [], ...lines] = parseCSV(text);
  const fields = FIELDS[kind];
  const byHeader = new Map(fields.map((f) => [norm(f.header), f]));
  const columns = header.map((name) => byHeader.get(norm(name)));
  const missing = fields.filter((f) => f.required && !columns.includes(f)).map((f) => f.header);
  const ignored = header.filter((_, i) => !columns[i] && header[i].trim() !== '');
  const rows = lines.map((cells) => {
    const row: Record<string, unknown> = {};
    columns.forEach((f, i) => {
      const v = f ? value(f.type, cells[i] ?? '') : undefined;
      if (f && v !== undefined) row[f.key] = v;
    });
    return row;
  });
  return { rows, missing, ignored };
}

/** An empty file with the columns to fill, for the owner to start from. */
export function importTemplate(kind: ImportKind): string {
  return toCSV(FIELDS[kind].map((f) => ({ header: f.header, value: () => '' })), []);
}
