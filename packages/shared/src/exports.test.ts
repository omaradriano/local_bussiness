import { describe, expect, it } from 'vitest';
import { toCSV } from './csv';
import { exportColumns, exportFileName } from './exports';
import { textsFor } from './verticals';

describe('exports', () => {
  it('writes bookings in the giro’s words, in pesos and local time', () => {
    const rows = [{ id: 'b0b0b0b0-aaaa', status: 'checked_out', source: 'admin', starts_at: '2026-10-05T15:00:00Z', ends_at: null,
      customer: 'Ana', phone: '+526141234567', items: 'Rotomartillo × 2', total: 60000, paid: 20000, due: 40000, extra_charges: 5000,
      created_at: '2026-10-01T18:00:00Z' }];
    const csv = toCSV(exportColumns('bookings', textsFor('tool_rental'), 'America/Chihuahua'), rows as never[]);
    const [header, line] = csv.replace('﻿', '').split('\r\n');
    expect(header).toBe('Folio,Estado,Inicio,Fin,Cliente,Teléfono,Artículos,Total,Pagado,Pendiente,Recargos,Origen,Alta');
    expect(line).toBe(`b0b0b0b0,${textsFor('tool_rental').bookings.status.checked_out},2026-10-05 09:00,,Ana,+526141234567,Rotomartillo × 2,600.00,200.00,400.00,50.00,Panel,2026-10-01 12:00`);
  });

  it('names the expense category and the file', () => {
    const csv = toCSV(exportColumns('expenses', textsFor('barbershop'), 'America/Chihuahua'),
      [{ spent_on: '2026-10-03T00:00:00Z', category: 'rent', description: null, amount: 800000, recurring: true }] as never[]);
    expect(csv).toContain('2026-10-03,Renta del local,,8000.00,Sí');
    expect(exportFileName('barber360', 'Ventas de productos', '2026-10-29')).toBe('barber360-ventas-de-productos-2026-10-29.csv');
  });
});
