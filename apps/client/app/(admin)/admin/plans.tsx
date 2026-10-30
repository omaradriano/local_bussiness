import { planHighlights, strings, TRIAL_DAYS, type PlanKey } from '@lb/shared';
import { Stack } from 'expo-router';
import { View } from 'react-native';
import { useMe } from '../../../lib/useMe';
import { Cell, Group, Notice, Screen, Text, useUI } from '../../../ui';

const t = strings.plans.page;
const PLANS: Exclude<PlanKey, 'trial'>[] = ['lite', 'pro', 'business'];

/** What each plan includes; prices and payment come later. */
export default function Plans() {
  const ui = useUI();
  const { businesses } = useMe();
  const plan = businesses?.[0]?.account.plan; // one plan per owner account
  const current = plan === 'trial' ? 'pro' : plan;

  return (
    <Screen>
      <Stack.Screen options={{ title: t.title }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 8 }}>{t.title}</Text>
      <Text variant="subhead" color="labelSecondary" style={{ marginBottom: 16 }}>{t.intro}</Text>
      <View style={{ gap: 8, marginBottom: 20 }}>
        <Notice tone="warning">{t.pricesPending}</Notice>
        <Notice>{t.trial(TRIAL_DAYS)}</Notice>
      </View>
      {PLANS.map((p) => (
        <Group key={p} label={p === current ? `${strings.plans.names[p]} · ${t.current}` : strings.plans.names[p]} footer={t.blurbs[p]}>
          {planHighlights(p).map((line) => <Cell key={line} icon="checkmark-circle" tint={ui.success} label={line} />)}
        </Group>
      ))}
    </Screen>
  );
}
