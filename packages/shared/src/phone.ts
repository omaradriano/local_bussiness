// Phones are stored in E.164 (+526141234567) so WhatsApp links work. The form asks for the
// country code and the national number separately (digits only).

export const COUNTRY_CODES = [
  { code: '+52', label: 'México', digits: 10 },
  { code: '+1', label: 'EE. UU. / Canadá', digits: 10 },
] as const;

export type CountryCode = (typeof COUNTRY_CODES)[number]['code'];

export const onlyDigits = (s: string) => s.replace(/\D/g, '');

/** country code + national digits -> E.164, or null if the length doesn't match the country. */
export function toE164(code: CountryCode, national: string): string | null {
  const c = COUNTRY_CODES.find((x) => x.code === code);
  const d = onlyDigits(national);
  return c && d.length === c.digits ? `${code}${d}` : null;
}

/** E.164 -> its country code and national digits (for editing); unknown codes keep everything as national. */
export function splitE164(e164: string | null | undefined): { code: CountryCode; national: string } {
  const s = e164 ?? '';
  for (const c of [...COUNTRY_CODES].sort((a, b) => b.code.length - a.code.length)) {
    if (s.startsWith(c.code)) return { code: c.code, national: s.slice(c.code.length) };
  }
  return { code: '+52', national: onlyDigits(s) };
}

/** "+526141234567" -> "+52 614 123 4567" for display. */
export function formatPhone(e164: string): string {
  const { code, national } = splitE164(e164);
  return national.length === 10 ? `${code} ${national.slice(0, 3)} ${national.slice(3, 6)} ${national.slice(6)}` : e164;
}
