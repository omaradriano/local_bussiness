// Custom fields of the catalog (module customFields): turning what the owner types into the value
// the API stores, and back to text. Format only; the API validates against the definitions.

export type CustomFieldType = 'text' | 'number' | 'select' | 'date' | 'boolean';
export type CustomValue = string | number | boolean;
type Def = { id: string; label: string; type: CustomFieldType; required: boolean };

/** Form input of one field: text for text/number/date/select, true/false for yes/no. */
export type CustomInput = string | boolean;

/** A typed value -> what the form shows ("2026-01-15" -> "15/01/2026", 1500 -> "1500"). */
export function customInputOf(type: CustomFieldType, v: CustomValue | null | undefined): CustomInput {
  if (type === 'boolean') return v === true;
  if (v === null || v === undefined) return '';
  if (type === 'date' && typeof v === 'string') {
    const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(v);
    return m ? `${m[3]}/${m[2]}/${m[1]}` : v;
  }
  return String(v);
}

/** One input -> value to send: null = empty, undefined = can't be read. */
export function customValueOf(type: CustomFieldType, input: CustomInput): CustomValue | null | undefined {
  if (type === 'boolean') return input === true;
  const s = String(input).trim();
  if (!s) return null;
  if (type === 'number') {
    const n = Number(s.replace(/,/g, ''));
    return /^-?[\d,]*\.?\d+$/.test(s) && Number.isFinite(n) ? n : undefined;
  }
  if (type === 'date') {
    const m = /^(\d{1,2})\/(\d{1,2})\/(\d{4})$/.exec(s) ?? /^(\d{4})-(\d{2})-(\d{2})$/.exec(s);
    if (!m) return undefined;
    const [y, mo, d] = s.includes('/') ? [m[3], m[2], m[1]] : [m[1], m[2], m[3]];
    const iso = `${y}-${mo!.padStart(2, '0')}-${d!.padStart(2, '0')}`;
    const date = new Date(`${iso}T00:00:00Z`);
    return !Number.isNaN(date.getTime()) && date.toISOString().startsWith(iso) ? iso : undefined;
  }
  return s;
}

/** The form's inputs -> custom_values to send, or a message per field the owner must fix. */
export function customValuesOf(fields: Def[], inputs: Record<string, CustomInput>): { values?: Record<string, CustomValue | null>; errors?: Record<string, string> } {
  const values: Record<string, CustomValue | null> = {};
  const errors: Record<string, string> = {};
  for (const f of fields) {
    const v = customValueOf(f.type, inputs[f.id] ?? (f.type === 'boolean' ? false : ''));
    if (v === undefined) errors[f.id] = f.type === 'date' ? 'Usa el formato DD/MM/AAAA.' : 'Escribe un número.';
    else if (v === null && f.required) errors[f.id] = 'Este campo es obligatorio.';
    else values[f.id] = v;
  }
  return Object.keys(errors).length ? { errors } : { values };
}

/** A saved value as text for lists ("Sí", "15/01/2026", "1,500"). */
export function customValueText(type: CustomFieldType, v: CustomValue | null | undefined): string {
  if (v === null || v === undefined || v === '') return '';
  if (type === 'boolean') return v ? 'Sí' : 'No';
  if (type === 'number' && typeof v === 'number') return v.toLocaleString('es-MX');
  return String(customInputOf(type, v));
}
