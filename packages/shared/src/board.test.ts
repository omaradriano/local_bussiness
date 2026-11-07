import { describe, expect, it } from 'vitest';
import { boardColumns, forwardMove } from './board';

const row = (id: string, status: string, starts_at: string, next: string[] = []) => ({ id, status, starts_at, next });

describe('boardColumns', () => {
  it('keeps the order flow, sorts by time and skips the optional columns when empty', () => {
    const cols = boardColumns([
      row('b', 'confirmed', '2026-11-02T18:00:00Z'),
      row('a', 'confirmed', '2026-11-02T16:00:00Z'),
      row('c', 'ready', '2026-11-02T17:00:00Z'),
      row('x', 'cancelled', '2026-11-02T17:00:00Z'),
    ]);
    expect(cols.map((c) => c.status)).toEqual(['confirmed', 'in_progress', 'ready', 'completed']);
    expect(cols[0].rows.map((r) => r.id)).toEqual(['a', 'b']);
  });

  it('shows «falta anticipo» and «en camino» only when an order is there', () => {
    const cols = boardColumns([row('a', 'pending_deposit', '2026-11-02T16:00:00Z'), row('b', 'out_for_delivery', '2026-11-02T16:00:00Z')]);
    expect(cols.map((c) => c.status)).toEqual(['pending_deposit', 'confirmed', 'in_progress', 'ready', 'out_for_delivery', 'completed']);
  });
});

describe('forwardMove', () => {
  it('is the first move that goes ahead, never cancelling', () => {
    expect(forwardMove(['in_progress', 'cancelled'])).toBe('in_progress');
    expect(forwardMove(['cancelled'])).toBeNull();
    expect(forwardMove([])).toBeNull();
  });

  it('ready: delivered at the counter unless it is a delivery', () => {
    expect(forwardMove(['out_for_delivery', 'completed'], 'pickup')).toBe('completed');
    expect(forwardMove(['out_for_delivery', 'completed'], 'delivery')).toBe('out_for_delivery');
  });
});
