import { normalizeTime, strings, type DayRow } from '@lb/shared';
import { Pressable, Text, TextInput, View } from 'react-native';

const t = strings.hours;

/** Weekly hours: one row per day with open/closed and the first shift's times. Extra shifts are kept untouched. */
export function HoursEditor({ week, onChange, editable }: { week: DayRow[]; onChange: (w: DayRow[]) => void; editable: boolean }) {
  const update = (d: number, patch: Partial<DayRow>) => onChange(week.map((row, i) => (i === d ? { ...row, ...patch } : row)));
  // Monday first, Sunday last — how people read a week here.
  const order = [1, 2, 3, 4, 5, 6, 0];

  return (
    <View style={{ gap: 8 }}>
      {order.map((d) => {
        const row = week[d];
        return (
          <View key={d} style={{ flexDirection: 'row', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
            <Text style={{ width: 90 }}>{t.weekdays[d]}</Text>
            <Pressable
              accessibilityRole="switch"
              accessibilityLabel={`${t.weekdays[d]}: ${row.open ? t.open : t.closed}`}
              accessibilityState={{ checked: row.open, disabled: !editable }}
              disabled={!editable}
              onPress={() => update(d, { open: !row.open })}
              style={{ borderWidth: 1, borderRadius: 6, paddingVertical: 6, paddingHorizontal: 10, minWidth: 84 }}
            >
              <Text>{row.open ? t.open : t.closed}</Text>
            </Pressable>
            {row.open && (
              <>
                {(['opens_at', 'closes_at'] as const).map((k) => (
                  <TextInput
                    key={k}
                    accessibilityLabel={`${t.weekdays[d]} ${k === 'opens_at' ? t.from : t.to}`}
                    editable={editable}
                    value={row[k]}
                    onChangeText={(v) => update(d, { [k]: v })}
                    onBlur={() => update(d, { [k]: normalizeTime(row[k]) ?? row[k] })}
                    inputMode="numeric"
                    style={{ borderWidth: 1, borderRadius: 6, padding: 6, width: 70, textAlign: 'center' }}
                  />
                ))}
                {row.extra.length > 0 && (
                  <Text style={{ opacity: 0.7 }}>+ {row.extra.map((s) => `${s.opens_at}–${s.closes_at}`).join(', ')}</Text>
                )}
              </>
            )}
          </View>
        );
      })}
    </View>
  );
}
