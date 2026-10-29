import { describe, expect, it } from 'vitest';
import { toCSV } from './csv';
import { exportColumns } from './exports';
import { importTemplate, parseCSV, rowsFromCSV } from './imports';
import { textsFor } from './verticals';

describe('imports', () => {
  it('parses quotes, CRLF, the BOM and ";" files', () => {
    expect(parseCSV('﻿Nombre,Notas\r\n"López, Ana","dice ""hola""\nsegunda línea"\r\n\r\n')).toEqual([
      ['Nombre', 'Notas'], ['López, Ana', 'dice "hola"\nsegunda línea'],
    ]);
    expect(parseCSV('Nombre;Precio\nCorte;150,50')).toEqual([['Nombre', 'Precio'], ['Corte', '150,50']]);
  });

  it('reads the columns in any case or accent, money in centavos', () => {
    const csv = 'NOMBRE,precio,Duracion (min),activo,Columna rara\nCorte,"$1,250.50",30,Sí,x\nBarba,abc,20,No,y';
    const got = rowsFromCSV('catalog', csv);
    expect(got.missing).toEqual([]);
    expect(got.ignored).toEqual(['Columna rara']);
    expect(got.rows[0]).toEqual({ name: 'Corte', price: 125050, duration_minutes: 30, active: true });
    expect(got.rows[1].price).toBe(-1); // the API says what's wrong with that row
  });

  it('says which required columns are missing', () => {
    expect(rowsFromCSV('customers', 'Nombre,Correo\nAna,a@b.mx').missing).toEqual(['Teléfono']);
  });

  it('reads back what the export wrote (and the template has the columns)', () => {
    const exported = toCSV(exportColumns('catalog', textsFor('tool_rental'), 'America/Chihuahua'),
      [{ kind: 'rental', name: 'Rotomartillo', price: 30000, price_unit: 'day', duration_minutes: null, security_deposit: 100000, late_fee_per_day: 30000, active: true }] as never[]);
    expect(rowsFromCSV('catalog', exported).rows[0]).toEqual({ name: 'Rotomartillo', price: 30000, price_unit: 'day', security_deposit: 100000, late_fee_per_day: 30000, active: true });
    expect(importTemplate('products')).toBe('﻿Nombre,Precio,Costo,Existencia,Activo\r\n');
  });
});
