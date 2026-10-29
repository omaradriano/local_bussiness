import { describe, expect, it } from 'vitest';
import { dateOnly, localDateTime, pesos, toCSV } from './csv';

describe('csv', () => {
  it('quotes what needs it and starts with a BOM for Excel', () => {
    const csv = toCSV([{ header: 'Nombre', value: (r: { n: string; t: boolean }) => r.n }, { header: 'Confianza', value: (r) => r.t }],
      [{ n: 'Ana "La Jefa", López', t: true }, { n: 'Línea\nnueva', t: false }]);
    expect(csv.startsWith('﻿Nombre,Confianza\r\n')).toBe(true);
    expect(csv).toContain('"Ana ""La Jefa"", López",Sí\r\n');
    expect(csv).toContain('"Línea\nnueva",No\r\n');
  });

  it('formats money and dates', () => {
    expect(pesos(123450)).toBe('1234.50');
    expect(pesos(null)).toBe('');
    // 15:00Z is 09:00 in Chihuahua (UTC-6).
    expect(localDateTime('2026-10-05T15:00:00Z', 'America/Chihuahua')).toBe('2026-10-05 09:00');
    expect(localDateTime(null, 'America/Chihuahua')).toBe('');
    expect(dateOnly('2026-10-03T00:00:00Z')).toBe('2026-10-03');
  });
});
