import { Ionicons } from '@expo/vector-icons';
import { PAID_PLANS, planComparison, strings, TRIAL_DAYS, type PlanCell } from '@lb/shared';
import { Stack } from 'expo-router';
import { StyleSheet, View } from 'react-native';
import { useMe } from '../../../lib/useMe';
import { Notice, Screen, Text, radius, useUI } from '../../../ui';

const t = strings.plans.page;
const rows = planComparison();

/** Plans side by side: what each one includes and what it doesn't. Prices and payment come later. */
export default function Plans() {
  const ui = useUI();
  const { businesses } = useMe();
  const plan = businesses?.[0]?.account.plan; // one plan per owner account
  const current = plan === 'trial' ? 'pro' : plan;
  const column = (p: string) => (p === current ? { backgroundColor: ui.accentFill } : null);

  function cell(c: PlanCell) {
    if (c === true) return <Ionicons name="checkmark-circle" size={20} color={ui.success} accessibilityLabel={t.included} />;
    if (c === false) return <Text color="labelSecondary" accessibilityLabel={t.notIncluded}>—</Text>;
    return <Text variant="subhead">{c}</Text>;
  }

  return (
    <Screen>
      <Stack.Screen options={{ title: t.title }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 8 }}>{t.title}</Text>
      <Text variant="subhead" color="labelSecondary" style={{ marginBottom: 16 }}>{t.intro}</Text>
      <View style={{ gap: 8, marginBottom: 20 }}>
        <Notice tone="warning">{t.pricesPending}</Notice>
        <Notice>{t.trial(TRIAL_DAYS)}</Notice>
      </View>

      <View style={{ backgroundColor: ui.surface, borderRadius: radius.sm, overflow: 'hidden' }}>
        <View style={styles.row} accessibilityRole="header">
          <View style={styles.label} />
          {PAID_PLANS.map((p) => (
            <View key={p} style={[styles.cell, column(p)]}>
              <Text variant="headline">{strings.plans.names[p]}</Text>
              {p === current && <Text variant="caption" color="accent">{t.current}</Text>}
            </View>
          ))}
        </View>
        {rows.map((r) => (
          <View key={r.label} style={[styles.row, { borderTopWidth: StyleSheet.hairlineWidth, borderColor: ui.separator }]}>
            <View style={styles.label}><Text variant="subhead">{r.label}</Text></View>
            {PAID_PLANS.map((p) => <View key={p} style={[styles.cell, column(p)]}>{cell(r.cells[p])}</View>)}
          </View>
        ))}
      </View>

      <View style={{ gap: 6, marginTop: 16 }}>
        {PAID_PLANS.map((p) => (
          <Text key={p} variant="footnote" color="labelSecondary">{`${strings.plans.names[p]}: ${t.blurbs[p]}`}</Text>
        ))}
      </View>
    </Screen>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'stretch' },
  label: { flex: 1.6, paddingVertical: 10, paddingHorizontal: 12, justifyContent: 'center' },
  cell: { flex: 1, paddingVertical: 10, paddingHorizontal: 4, alignItems: 'center', justifyContent: 'center' },
});
