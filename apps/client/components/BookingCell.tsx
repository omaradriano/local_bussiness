import type { components } from '@lb/api-client';
import { formatMXN, strings } from '@lb/shared';
import { router } from 'expo-router';
import { Cell } from '../ui';

type Summary = components['schemas']['BookingSummary'];

/** One booking in a list (Hoy, calendario): time + what, then customer, status and total; opens the detail. */
export function BookingCell({ businessId, b, time }: { businessId: string; b: Summary; time: string }) {
  const what = `${b.items}${b.units.length ? ` · ${b.units.join(', ')}` : ''}${b.staff.length ? ` · ${b.staff.join(', ')}` : ''}`;
  return (
    <Cell label={`${time} · ${what}`} sublabel={`${b.customer_name} · ${strings.bookings.status[b.status]}`} value={formatMXN(b.total)}
      onPress={() => router.push({ pathname: '/admin/[businessId]/bookings/[bookingId]', params: { businessId, bookingId: b.id } })} />
  );
}
