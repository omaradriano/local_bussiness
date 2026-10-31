import { describe, expect, it } from 'vitest';
import { customInputOf, customValueOf, customValuesOf, customValueText } from './customFields';

describe('customValueOf', () => {
  it('text and lists are trimmed; empty is null', () => {
    expect(customValueOf('text', '  DeWalt ')).toBe('DeWalt');
    expect(customValueOf('select', 'Buena')).toBe('Buena');
    expect(customValueOf('text', '   ')).toBeNull();
  });
  it('numbers, with thousands separators', () => {
    expect(customValueOf('number', '1,500.5')).toBe(1500.5);
    expect(customValueOf('number', '-3')).toBe(-3);
    expect(customValueOf('number', 'mucho')).toBeUndefined();
    expect(customValueOf('number', '1.2.3')).toBeUndefined();
  });
  it('dates as DD/MM/AAAA or AAAA-MM-DD', () => {
    expect(customValueOf('date', '5/1/2026')).toBe('2026-01-05');
    expect(customValueOf('date', '2026-01-15')).toBe('2026-01-15');
    expect(customValueOf('date', '30/02/2026')).toBeUndefined();
    expect(customValueOf('date', 'ayer')).toBeUndefined();
  });
  it('yes/no', () => {
    expect(customValueOf('boolean', true)).toBe(true);
    expect(customValueOf('boolean', false)).toBe(false);
  });
});

describe('customInputOf and customValueText', () => {
  it('round trip for the form', () => {
    expect(customInputOf('date', '2026-01-15')).toBe('15/01/2026');
    expect(customInputOf('number', 1500)).toBe('1500');
    expect(customInputOf('boolean', undefined)).toBe(false);
    expect(customInputOf('text', undefined)).toBe('');
  });
  it('text for lists', () => {
    expect(customValueText('boolean', true)).toBe('Sí');
    expect(customValueText('date', '2026-01-15')).toBe('15/01/2026');
    expect(customValueText('number', 1500)).toBe('1,500');
    expect(customValueText('text', undefined)).toBe('');
  });
});

describe('customValuesOf', () => {
  const fields = [
    { id: 'brand', label: 'Marca', type: 'text' as const, required: true },
    { id: 'watts', label: 'Potencia', type: 'number' as const, required: false },
    { id: 'case', label: 'Estuche', type: 'boolean' as const, required: false },
  ];
  it('builds the values; empty ones go as null so the API removes them', () => {
    expect(customValuesOf(fields, { brand: 'Bosch', watts: '' })).toEqual({ values: { brand: 'Bosch', watts: null, case: false } });
  });
  it('reports unreadable and missing required fields', () => {
    expect(customValuesOf(fields, { brand: '', watts: 'x' }).errors).toEqual({ brand: 'Este campo es obligatorio.', watts: 'Escribe un número.' });
  });
});
