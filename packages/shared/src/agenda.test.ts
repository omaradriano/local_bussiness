import { describe, expect, it } from 'vitest';
import { agendaGroups } from './agenda';

const iso = (d: number, h: number) => new Date(2026, 10, d, h).toISOString();
const today = '2026-11-02';

describe('agendaGroups', () => {
  const rows = [
    { id: 'a', kind: 'service' as const, status: 'confirmed', starts_at: iso(2, 11), ends_at: iso(2, 12) },
    { id: 'b', kind: 'service' as const, status: 'pending_deposit', starts_at: iso(2, 9), ends_at: iso(2, 10) },
    { id: 'c', kind: 'service' as const, status: 'confirmed', starts_at: iso(3, 9), ends_at: iso(3, 10) }, // tomorrow
    { id: 'd', kind: 'product' as const, status: 'ready', starts_at: iso(2, 18), ends_at: null },
    { id: 'e', kind: 'rental' as const, status: 'confirmed', starts_at: iso(2, 9), ends_at: iso(4, 9) },
    { id: 'f', kind: 'rental' as const, status: 'checked_out', starts_at: iso(1, 9), ends_at: iso(2, 17) },
    { id: 'g', kind: 'rental' as const, status: 'overdue', starts_at: iso(1, 9), ends_at: iso(1, 17) },
  ];
  const g = agendaGroups(rows, today);
  const ids = (k: keyof typeof g) => g[k].map((r) => r.id);

  it('orders today appointments by time and leaves tomorrow out', () => expect(ids('appointments')).toEqual(['b', 'a']));
  it('lists today deliveries', () => expect(ids('deliveries')).toEqual(['d']));
  it('splits rentals going out and coming back', () => {
    expect(ids('goingOut')).toEqual(['e']);
    expect(ids('comingBack')).toEqual(['f']);
  });
  it('puts overdue rentals first and only there', () => {
    expect(ids('overdue')).toEqual(['g']);
    expect(ids('comingBack')).not.toContain('g');
  });
  it('flags holds awaiting the advance', () => expect(ids('awaitingDeposit')).toEqual(['b']));
});
