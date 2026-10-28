import type { components } from '@lb/api-client';
import { strings } from '@lb/shared';
import { Ionicons } from '@expo/vector-icons';
import { View } from 'react-native';
import { Text, radius, useUI, type UIColors } from '../ui';

type Status = components['schemas']['Booking']['status'];
type Kind = components['schemas']['Booking']['kind'];

// Color + icon per status: blue = upcoming, indigo = with the customer, red = late,
// purple = back but not closed, green = closed, grey = fell through.
const look: Record<Status, { color: keyof UIColors; icon: keyof typeof Ionicons.glyphMap }> = {
  pending_deposit: { color: 'warning', icon: 'hourglass' },
  confirmed: { color: 'accent', icon: 'calendar' },
  in_progress: { color: 'indigo', icon: 'play-circle' },
  ready: { color: 'success', icon: 'checkmark-done' },
  out_for_delivery: { color: 'indigo', icon: 'car' },
  checked_out: { color: 'indigo', icon: 'arrow-up-circle' },
  overdue: { color: 'danger', icon: 'alert-circle' },
  returned: { color: 'purple', icon: 'arrow-down-circle' },
  completed: { color: 'success', icon: 'checkmark-circle' },
  cancelled: { color: 'gray', icon: 'close-circle' },
  expired: { color: 'gray', icon: 'time' },
  no_show: { color: 'gray', icon: 'person-remove' },
};

/** Descriptive label for a booking's state (rentals say where the tool is and what's next). */
export function statusLabel(kind: Kind, status: Status): string {
  return (kind === 'rental' ? strings.bookings.rentalStatus[status] : undefined) ?? strings.bookings.status[status];
}

/** Colored pill with the booking's state. */
export function StatusBadge({ kind, status, size = 'sm' }: { kind: Kind; status: Status; size?: 'sm' | 'md' }) {
  const ui = useUI();
  const { color, icon } = look[status] ?? look.confirmed;
  const c = ui[color];
  return (
    <View accessibilityLabel={statusLabel(kind, status)}
      style={{ flexDirection: 'row', alignItems: 'center', gap: 4, alignSelf: 'flex-start', borderRadius: radius.full,
        paddingHorizontal: size === 'md' ? 12 : 8, paddingVertical: size === 'md' ? 6 : 3, backgroundColor: `${c}26` }}>
      <Ionicons name={icon} size={size === 'md' ? 16 : 13} color={c} />
      <Text variant={size === 'md' ? 'subhead' : 'caption'} style={{ color: c, fontWeight: '600' }}>{statusLabel(kind, status)}</Text>
    </View>
  );
}
