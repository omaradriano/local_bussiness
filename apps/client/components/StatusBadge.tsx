import type { components } from '@lb/api-client';
import { Ionicons } from '@expo/vector-icons';
import { View } from 'react-native';
import { useTexts } from '../lib/useTexts';
import { Text, radius, useUI, type UIColors } from '../ui';

type Status = components['schemas']['Booking']['status'];

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

/** Colored pill with the booking's state. */
export function StatusBadge({ status, size = 'sm' }: { status: Status; size?: 'sm' | 'md' }) {
  const ui = useUI();
  const label = useTexts().texts.bookings.status[status]; // in the words of the business type
  const { color, icon } = look[status] ?? look.confirmed;
  const c = ui[color];
  return (
    <View accessibilityLabel={label}
      style={{ flexDirection: 'row', alignItems: 'center', gap: 4, alignSelf: 'flex-start', borderRadius: radius.full,
        paddingHorizontal: size === 'md' ? 12 : 8, paddingVertical: size === 'md' ? 6 : 3, backgroundColor: `${c}26` }}>
      <Ionicons name={icon} size={size === 'md' ? 16 : 13} color={c} />
      <Text variant={size === 'md' ? 'subhead' : 'caption'} style={{ color: c, fontWeight: '600' }}>{label}</Text>
    </View>
  );
}
