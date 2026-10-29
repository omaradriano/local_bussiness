import type { components } from '@lb/api-client';
import { strings } from '@lb/shared';
import { View } from 'react-native';
import { Text, radius, useUI } from '../ui';

type Account = components['schemas']['AccountPlan'];

/** Pill with the time left: the trial's days, a plan about to end (≤ 7 days), or read-only. Nothing otherwise. */
export function PlanBadge({ account }: { account: Account }) {
  const ui = useUI();
  const p = strings.plans;
  const expired = account.status === 'expired';
  const label = expired ? p.readOnly
    : account.status === 'trial' ? p.badgeTrial(account.days_left)
    : account.days_left >= 0 && account.days_left <= 7 ? p.badgeEnds(account.days_left)
    : null;
  if (!label) return null;
  const urgent = expired || account.days_left <= 3;
  return (
    <View accessibilityRole="text" style={{ alignSelf: 'flex-start', paddingHorizontal: 8, paddingVertical: 3, borderRadius: radius.full,
      backgroundColor: urgent ? ui.warningFill : ui.accentFill }}>
      <Text variant="caption" style={{ color: urgent ? ui.warning : ui.accent, fontWeight: '600' }}>{label}</Text>
    </View>
  );
}
