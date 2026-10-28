import { normalizeTime, strings, type DayRow } from '@lb/shared';
import { StyleSheet, Switch, TextInput, View } from 'react-native';
import { Text, radius, useUI } from '../ui';

const t = strings.hours;

/** Weekly hours: one row per day with open/closed and the first shift's times. Extra shifts are kept untouched. */
export function HoursEditor({ week, onChange, editable }: { week: DayRow[]; onChange: (w: DayRow[]) => void; editable: boolean }) {
  const ui = useUI();
  const update = (d: number, patch: Partial<DayRow>) => onChange(week.map((row, i) => (i === d ? { ...row, ...patch } : row)));
  // Monday first, Sunday last — how people read a week here.
  const order = [1, 2, 3, 4, 5, 6, 0];

  return (
    <View style={{ backgroundColor: ui.surface, borderRadius: radius.sm, overflow: 'hidden', marginBottom: 28 }}>
      {order.map((d, i) => {
        const row = week[d];
        return (
          <View key={d}>
            {i > 0 && <View style={{ height: StyleSheet.hairlineWidth, backgroundColor: ui.separator, marginLeft: 16 }} />}
            <View style={{ flexDirection: 'row', alignItems: 'center', gap: 8, minHeight: 52, paddingHorizontal: 16, paddingVertical: 8 }}>
              <Text style={{ flex: 1 }}>{t.weekdays[d]}</Text>
              {row.open ? (
                (['opens_at', 'closes_at'] as const).map((k, j) => (
                  <View key={k} style={{ flexDirection: 'row', alignItems: 'center', gap: 6 }}>
                    {j === 1 && <Text color="labelSecondary">–</Text>}
                    <TextInput
                      accessibilityLabel={`${t.weekdays[d]} ${k === 'opens_at' ? t.from : t.to}`}
                      editable={editable}
                      value={row[k]}
                      onChangeText={(v) => update(d, { [k]: v })}
                      onBlur={() => update(d, { [k]: normalizeTime(row[k]) ?? row[k] })}
                      inputMode="numeric"
                      style={{ width: 64, minHeight: 36, borderRadius: 8, textAlign: 'center', fontSize: 16, backgroundColor: ui.fill, color: ui.label, fontVariant: ['tabular-nums'] }}
                    />
                  </View>
                ))
              ) : (
                <Text color="labelSecondary">{t.closed}</Text>
              )}
              <Switch accessibilityLabel={`${t.weekdays[d]}: ${row.open ? t.open : t.closed}`} value={row.open} disabled={!editable}
                onValueChange={(open) => update(d, { open })} />
            </View>
            {row.open && row.extra.length > 0 && (
              <Text variant="footnote" color="labelSecondary" style={{ paddingHorizontal: 16, paddingBottom: 8 }}>
                + {row.extra.map((s) => `${s.opens_at}–${s.closes_at}`).join(', ')}
              </Text>
            )}
          </View>
        );
      })}
    </View>
  );
}
