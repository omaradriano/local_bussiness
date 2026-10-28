import { Ionicons } from '@expo/vector-icons';
import type { ReactNode } from 'react';
import { StyleSheet, View } from 'react-native';
import { Text } from './Text';
import { radius, space, useUI } from './tokens';

export type StepState = 'done' | 'current' | 'upcoming';

/**
 * One numbered step of a flow (e.g. "2 · Documentos"): number or check, title, what it's for,
 * and its content. Upcoming steps are dimmed but readable so the whole path is predictable.
 */
export function Step({ n, title, description, state = 'current', right, children }: {
  n: number; title: string; description?: string; state?: StepState; right?: ReactNode; children?: ReactNode;
}) {
  const ui = useUI();
  const done = state === 'done';
  const current = state === 'current';
  return (
    <View accessibilityLabel={`Paso ${n}: ${title}`} style={[styles.step, { backgroundColor: ui.surface, opacity: state === 'upcoming' ? 0.6 : 1 },
      current && { borderColor: ui.accent, borderWidth: 1.5 }]}>
      <View style={styles.head}>
        <View style={[styles.badge, { backgroundColor: done ? ui.success : current ? ui.accent : ui.fill }]}>
          {done ? <Ionicons name="checkmark" size={16} color="#fff" />
            : <Text variant="subhead" style={{ color: current ? ui.onAccent : ui.label, fontWeight: '700' }}>{n}</Text>}
        </View>
        <View style={styles.texts}>
          <Text variant="headline">{title}</Text>
          {description ? <Text variant="footnote" color="labelSecondary">{description}</Text> : null}
        </View>
        {right}
      </View>
      {children ? <View style={styles.body}>{children}</View> : null}
    </View>
  );
}

/** A label/value line for summaries (totals, dates). */
export function Row({ label, value, strong }: { label: string; value: string; strong?: boolean }) {
  return (
    <View style={styles.row}>
      <Text variant={strong ? 'headline' : 'subhead'} color={strong ? 'label' : 'labelSecondary'}>{label}</Text>
      <Text variant={strong ? 'headline' : 'subhead'} style={{ fontVariant: ['tabular-nums'] }}>{value}</Text>
    </View>
  );
}

/** Colored note: info (accent), success, or warning. */
export function Notice({ tone = 'info', children }: { tone?: 'info' | 'success' | 'warning'; children: string }) {
  const ui = useUI();
  const bg = tone === 'success' ? ui.successFill : tone === 'warning' ? ui.warningFill : ui.accentFill;
  const icon = tone === 'success' ? 'checkmark-circle' : tone === 'warning' ? 'alert-circle' : 'information-circle';
  const fg = tone === 'success' ? ui.success : tone === 'warning' ? ui.warning : ui.accent;
  return (
    <View accessibilityRole="alert" style={[styles.notice, { backgroundColor: bg }]}>
      <Ionicons name={icon} size={18} color={fg} />
      <Text variant="subhead" style={{ flex: 1 }}>{children}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  step: { borderRadius: radius.md, padding: space.lg, gap: space.md, borderWidth: 1.5, borderColor: 'transparent' },
  head: { flexDirection: 'row', gap: space.md, alignItems: 'flex-start' },
  badge: { width: 28, height: 28, borderRadius: 14, alignItems: 'center', justifyContent: 'center' },
  texts: { flex: 1, gap: 2 },
  body: { gap: space.md },
  row: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'baseline', gap: space.md },
  notice: { flexDirection: 'row', gap: space.sm, alignItems: 'flex-start', padding: space.md, borderRadius: radius.sm },
});
