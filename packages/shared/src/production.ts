// Production sheet (with the day board): what the workshop has to make that day, added up by
// arrangement, and each order with its time, description and card, to print.

const TO_MAKE = new Set(['pending_deposit', 'confirmed', 'in_progress']);
const DONE = new Set(['ready', 'out_for_delivery', 'completed']);

type Row = { status: string; starts_at: string; items: string };

export function productionSheet<T extends Row>(rows: T[]) {
  const sorted = [...rows].sort((a, b) => a.starts_at.localeCompare(b.starts_at));
  const toMake = sorted.filter((r) => TO_MAKE.has(r.status));
  const done = sorted.filter((r) => DONE.has(r.status));
  return { totals: addUp(toMake), toMake, done, doneTotals: addUp(done) };
}

/** Pieces by arrangement, most first. */
function addUp(rows: Row[]) {
  const counts = new Map<string, number>();
  for (const r of rows) {
    // shortcut: reads the list's «2 × Ramo» text (one item per order today); use structured lines if an order ever has several.
    for (const part of r.items.split(', ')) {
      const m = /^(\d+) × (.+)$/.exec(part);
      const name = m ? m[2] : part;
      counts.set(name, (counts.get(name) ?? 0) + (m ? Number(m[1]) : 1));
    }
  }
  return [...counts].map(([name, quantity]) => ({ name, quantity })).sort((a, b) => b.quantity - a.quantity || a.name.localeCompare(b.name, 'es'));
}

// Still to hand over: everything made or being made, until it's delivered or picked up.
const ON_ROUTE = new Set([...TO_MAKE, 'ready', 'out_for_delivery']);

type Stop = Row & { fulfillment: string | null; delivery_zone: string | null };

/** Route sheet: deliveries by zone (A–Z, orders without a zone last), each by time; pick-ups apart. */
export function routeSheet<T extends Stop>(rows: T[]) {
  const sorted = [...rows].filter((r) => ON_ROUTE.has(r.status)).sort((a, b) => a.starts_at.localeCompare(b.starts_at));
  const byZone = new Map<string | null, T[]>();
  for (const r of sorted.filter((r) => r.fulfillment === 'delivery')) {
    byZone.set(r.delivery_zone, [...(byZone.get(r.delivery_zone) ?? []), r]);
  }
  const zones = [...byZone].map(([zone, stops]) => ({ zone, stops }))
    .sort((a, b) => (a.zone === null ? 1 : b.zone === null ? -1 : a.zone.localeCompare(b.zone, 'es')));
  return { zones, pickups: sorted.filter((r) => r.fulfillment !== 'delivery') };
}
