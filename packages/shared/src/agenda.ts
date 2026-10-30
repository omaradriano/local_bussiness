import { bookingDays } from './datetime';
import { engineFor } from './engines';
import type { ItemKind } from './verticals/types';

type Row = { kind: ItemKind; status: string; starts_at: string; ends_at: string | null };

export type AgendaGroup = 'overdue' | 'awaitingDeposit' | 'appointments' | 'deliveries' | 'goingOut' | 'comingBack';

/**
 * Splits active bookings into the day's to-do groups (a booking may appear in more than one,
 * e.g. a same-day rental goes out and comes back). Dates are local "YYYY-MM-DD".
 */
export function agendaGroups<T extends Row>(rows: T[], today: string): Record<AgendaGroup, T[]> {
  const g: Record<AgendaGroup, T[]> = { overdue: [], awaitingDeposit: [], appointments: [], deliveries: [], goingOut: [], comingBack: [] };
  const day = (iso: string) => bookingDays(iso, null)[0];
  for (const b of rows) {
    if (b.status === 'overdue') { g.overdue.push(b); continue; }
    if (b.status === 'pending_deposit') g.awaitingDeposit.push(b);
    const e = engineFor(b.kind);
    if (e.timing === 'slot' && day(b.starts_at) === today) g.appointments.push(b);
    if (e.timing === 'day' && day(b.starts_at) === today) g.deliveries.push(b);
    if (e.custody && day(b.starts_at) === today && b.status !== 'checked_out') g.goingOut.push(b);
    if (e.custody && b.ends_at && day(b.ends_at) === today && (b.status === 'checked_out' || b.status === 'confirmed')) g.comingBack.push(b);
  }
  for (const k of Object.keys(g) as AgendaGroup[]) g[k].sort((a, b) => a.starts_at.localeCompare(b.starts_at));
  return g;
}
