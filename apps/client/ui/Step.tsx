import { Ionicons } from '@expo/vector-icons';
import type { ReactNode } from 'react';
import { StyleSheet, View } from 'react-native';
import { Text } from './Text';
import { radius, space, useUI } from './tokens';

export type StepState = 'done' | 'current' | 'upcoming';

/**
 * One numbered step of a flow (e.g. "2 · Salida"): number or check, title and what it's for.
 * Its content (only for the current step, usually) hangs below on a timeline line as separate
 * cards (SubStep), so each task reads on its own instead of one long box.
 */
export function Step({ n, title, description, state = 'current', right, inline, children }: {
  n: number; title: string; description?: string; state?: StepState; right?: ReactNode;
  /** Content inside the step's own card (a short form) instead of separate cards below. */
  inline?: boolean; children?: ReactNode;
}) {
  const ui = useUI();
  const done = state === 'done';
  const current = state === 'current';
  return (
    <View accessibilityLabel={`Paso ${n}: ${title}`}>
      <View style={[styles.step, { backgroundColor: ui.surface, opacity: state === 'upcoming' ? 0.55 : 1 },
        current && { borderColor: ui.accent }]}>
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
        {inline && children ? <View style={styles.body}>{children}</View> : null}
      </View>
      {!inline && children ? (
        <View style={[styles.rail, { borderColor: current ? ui.accent : ui.separator }]}>{children}</View>
      ) : null}
    </View>
  );
}

/** One task inside a step (e.g. "Fotos de salida"), as its own card on the step's timeline. */
export function SubStep({ title, description, done, children }: { title: string; description?: string; done?: boolean; children?: ReactNode }) {
  const ui = useUI();
  return (
    <View style={[styles.sub, { backgroundColor: ui.surface }]}>
      <View style={styles.subHead}>
        {done !== undefined && (
          <Ionicons name={done ? 'checkmark-circle' : 'ellipse-outline'} size={20} color={done ? ui.success : ui.labelSecondary} />
        )}
        <View style={styles.texts}>
          <Text variant="subhead" style={{ fontWeight: '600' }}>{title}</Text>
          {description ? <Text variant="footnote" color="labelSecondary">{description}</Text> : null}
        </View>
      </View>
      {children}
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
  body: { gap: space.md, marginTop: space.md },
  rail: { marginLeft: space.lg + 13, paddingLeft: space.lg, paddingTop: space.md, gap: space.md, borderLeftWidth: 2 },
  sub: { borderRadius: radius.md, padding: space.lg, gap: space.md },
  subHead: { flexDirection: 'row', gap: space.sm, alignItems: 'flex-start' },
  row: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'baseline', gap: space.md },
  notice: { flexDirection: 'row', gap: space.sm, alignItems: 'flex-start', padding: space.md, borderRadius: radius.sm },
});
