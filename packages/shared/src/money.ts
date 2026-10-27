// Money is always integer centavos (MXN); these convert to/from what people type and read.

const mxn = new Intl.NumberFormat('es-MX', { style: 'currency', currency: 'MXN' });

export function formatMXN(centavos: number): string {
  return mxn.format(centavos / 100);
}

/** "1,250.5" | "$250" | "250.50" -> centavos; null if not a valid non-negative amount (max 2 decimals). */
export function parsePesos(input: string): number | null {
  const s = input.trim().replace(/^\$/, '').replace(/,/g, '').trim();
  const m = /^(\d+)(?:\.(\d{1,2}))?$/.exec(s);
  if (!m) return null;
  return Number(m[1]) * 100 + Number((m[2] ?? '').padEnd(2, '0'));
}

/** centavos -> "250" or "250.50", for pre-filling an input. */
export function centavosToInput(centavos: number): string {
  const pesos = Math.floor(centavos / 100);
  const cents = centavos % 100;
  return cents === 0 ? String(pesos) : `${pesos}.${String(cents).padStart(2, '0')}`;
}
