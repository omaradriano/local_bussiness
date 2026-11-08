// Production sheet (with the day board): what the workshop has to make that day, added up by
// arrangement, and each order with its time, description and card, to print.

const TO_MAKE = new Set(['pending_deposit', 'confirmed', 'in_progress']);
const DONE = new Set(['ready', 'out_for_delivery', 'completed']);

type Row = { status: string; starts_at: string; items: string };

export function productionSheet<T extends Row>(rows: T[]) {
  const sorted = [...rows].sort((a, b) => a.starts_at.localeCompare(b.starts_at));
  const toMake = sorted.filter((r) => TO_MAKE.has(r.status));
  const counts = new Map<string, number>();
  for (const r of toMake) {
    // shortcut: reads the list's «2 × Ramo» text (one item per order today); use structured lines if an order ever has several.
    for (const part of r.items.split(', ')) {
      const m = /^(\d+) × (.+)$/.exec(part);
      const name = m ? m[2] : part;
      counts.set(name, (counts.get(name) ?? 0) + (m ? Number(m[1]) : 1));
    }
  }
  const totals = [...counts].map(([name, quantity]) => ({ name, quantity })).sort((a, b) => b.quantity - a.quantity || a.name.localeCompare(b.name, 'es'));
  return { totals, toMake, done: sorted.filter((r) => DONE.has(r.status)) };
}
