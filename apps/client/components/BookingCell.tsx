import type { components } from '@lb/api-client';
import { formatMXN, formatPhone, strings, WALK_IN_PHONE } from '@lb/shared';
import { Ionicons } from '@expo/vector-icons';
import { router } from 'expo-router';
import { Pressable, View } from 'react-native';
import { Text, useUI } from '../ui';
import { StatusBadge } from './StatusBadge';

type Summary = components['schemas']['BookingSummary'];

/** One booking in a list (Hoy, calendario, rentas): dates + what, customer, its state as a colored tag, and the total. */
export function BookingCell({ businessId, b, time }: { businessId: string; b: Summary; time: string }) {
  const ui = useUI();
  const what = `${b.items}${b.units.length ? ` · ${b.units.join(', ')}` : ''}${b.staff.length ? ` · ${b.staff.join(', ')}` : ''}`;
  return (
    <Pressable accessibilityRole="button" accessibilityLabel={`${what}, ${b.customer_name}`}
      onPress={() => router.push({ pathname: '/admin/[businessId]/bookings/[bookingId]', params: { businessId, bookingId: b.id } })}
      style={({ pressed }) => ({ flexDirection: 'row', alignItems: 'center', gap: 12, paddingVertical: 12, paddingHorizontal: 16, backgroundColor: pressed ? ui.fill : 'transparent' })}>
      <View style={{ flex: 1, minWidth: 0, gap: 4 }}>
        <Text numberOfLines={1}>{what}</Text>
        <Text variant="footnote" color="labelSecondary" numberOfLines={1}>
          {[time, b.customer_name, b.customer_phone !== WALK_IN_PHONE ? formatPhone(b.customer_phone) : null].filter(Boolean).join(' · ')}
        </Text>
        {b.fulfillment && (
          <Text variant="footnote" color="labelSecondary" numberOfLines={1}>
            {b.fulfillment === 'delivery' ? `${strings.board.delivery}${b.delivery_address ? `: ${b.delivery_address}` : ''}` : strings.board.pickup}
          </Text>
        )}
        <StatusBadge status={b.status} />
      </View>
      <View style={{ alignItems: 'flex-end', gap: 2 }}>
        <Text color="labelSecondary" style={{ fontVariant: ['tabular-nums'] }}>{formatMXN(b.total)}</Text>
        {b.due > 0 && <Text variant="footnote" color="warning" style={{ fontVariant: ['tabular-nums'] }}>{strings.bookings.dueShort(formatMXN(b.due))}</Text>}
      </View>
      <Ionicons name="chevron-forward" size={18} color={ui.labelSecondary} style={{ opacity: 0.6 }} />
    </Pressable>
  );
}
