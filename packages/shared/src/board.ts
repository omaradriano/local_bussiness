// Day board (feature dayBoard): the day's orders in columns by status, each moved ahead with one tap.
// Which moves exist comes from the API (`next`, from the state machine in Go); this only orders them.

/** The order flow, left to right. Optional columns show only when an order is in them. */
const FLOW = ['pending_deposit', 'confirmed', 'in_progress', 'ready', 'out_for_delivery', 'completed'] as const;
const OPTIONAL = new Set<string>(['pending_deposit', 'out_for_delivery']);
const SIDE = new Set<string>(['cancelled', 'expired', 'no_show']); // moves that don't go ahead

export type BoardStatus = (typeof FLOW)[number];

type Row = { status: string; starts_at: string };

export function boardColumns<T extends Row>(rows: T[]): { status: BoardStatus; rows: T[] }[] {
  const sorted = [...rows].sort((a, b) => a.starts_at.localeCompare(b.starts_at));
  return FLOW.map((status) => ({ status, rows: sorted.filter((r) => r.status === status) }))
    .filter((c) => c.rows.length > 0 || !OPTIONAL.has(c.status));
}

/** The one-tap move: the first that goes ahead. A ready order picked up at the counter is handed over directly. */
export function forwardMove(next: string[], fulfillment?: string | null): string | null {
  const ahead = next.filter((s) => !SIDE.has(s));
  if (fulfillment !== 'delivery' && ahead.includes('completed')) return 'completed';
  return ahead[0] ?? null;
}

const TO_HAND_OVER = new Set(['confirmed', 'in_progress', 'ready', 'out_for_delivery']);

/** Money still missing on a card: the advance before it's confirmed, or the rest to collect before it's handed over. */
export function paymentAlert(b: { status: string; due: number }): 'advance' | 'balance' | null {
  if (b.status === 'pending_deposit') return 'advance';
  if (TO_HAND_OVER.has(b.status) && b.due > 0) return 'balance';
  return null;
}
