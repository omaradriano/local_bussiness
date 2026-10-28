import { addMonths, inRange, monthGrid, monthLabel, todayISO, WEEKDAYS_SHORT } from '@lb/shared';
import { Ionicons } from '@expo/vector-icons';
import { Pressable, View } from 'react-native';
import { PressableScale, Text, useUI } from '../ui';

export type DayInfo = { disabled?: boolean; badge?: string };

/**
 * Month grid (Monday first). Days come as "YYYY-MM-DD". The parent decides what each tap means
 * (single day, or start/end of a range) and which days are disabled or carry a badge.
 */
export function Calendar({ month, onMonthChange, dayInfo, from, to, onSelect, label, minMonth }: {
  month: string; // first day of the visible month
  onMonthChange: (m: string) => void;
  dayInfo?: (date: string) => DayInfo;
  from: string | null;
  to?: string | null;
  onSelect: (date: string) => void;
  label?: string;
  minMonth?: string; // can't go back past this month (e.g. the current one)
}) {
  const ui = useUI();
  const atMin = !!minMonth && month <= minMonth;
  const today = todayISO();
  return (
    <View accessibilityLabel={label} style={{ gap: 6, maxWidth: 420, width: '100%' }}>
      <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' }}>
        <Pressable accessibilityRole="button" accessibilityLabel="Mes anterior" accessibilityState={{ disabled: atMin }} disabled={atMin}
          hitSlop={10} onPress={() => onMonthChange(addMonths(month, -1))} style={{ padding: 6, opacity: atMin ? 0.25 : 1 }}>
          <Ionicons name="chevron-back" size={22} color={ui.accent} />
        </Pressable>
        <Text variant="headline" style={{ textTransform: 'capitalize' }}>{monthLabel(month)}</Text>
        <Pressable accessibilityRole="button" accessibilityLabel="Mes siguiente" hitSlop={10} onPress={() => onMonthChange(addMonths(month, 1))} style={{ padding: 6 }}>
          <Ionicons name="chevron-forward" size={22} color={ui.accent} />
        </Pressable>
      </View>
      <View style={{ flexDirection: 'row' }}>
        {WEEKDAYS_SHORT.map((w, i) => (
          <Text key={i} variant="caption" color="labelSecondary" style={{ flex: 1, textAlign: 'center', fontWeight: '600' }}>{w}</Text>
        ))}
      </View>
      {monthGrid(month).map((week, wi) => (
        <View key={wi} style={{ flexDirection: 'row' }}>
          {week.map((date, di) => {
            if (!date) return <View key={di} style={{ flex: 1, aspectRatio: 1 }} />;
            const info = dayInfo?.(date) ?? {};
            const selected = inRange(date, from, to ?? null);
            const edge = date === from || date === to;
            // Range: a continuous band with the two ends as filled circles (iOS-style).
            const bandLeft = selected && date !== from;
            const bandRight = selected && !!to && date !== to;
            return (
              <View key={date} style={{ flex: 1, aspectRatio: 1, justifyContent: 'center' }}>
                {(bandLeft || bandRight) && (
                  <View style={{ position: 'absolute', top: '12%', bottom: '12%', left: bandLeft ? 0 : '50%', right: bandRight ? 0 : '50%', backgroundColor: ui.accentFill }} />
                )}
                <PressableScale
                  accessibilityRole="button"
                  accessibilityLabel={`${date}${info.badge ? `, ${info.badge}` : ''}${info.disabled ? ', no disponible' : ''}`}
                  accessibilityState={{ selected, disabled: !!info.disabled }}
                  disabled={info.disabled}
                  onPress={() => onSelect(date)}
                  scaleTo={0.92}
                  containerStyle={{ alignSelf: 'center', width: '76%', aspectRatio: 1 }}
                  style={{
                    flex: 1, borderRadius: 999, alignItems: 'center', justifyContent: 'center',
                    backgroundColor: edge ? ui.accent : 'transparent', opacity: info.disabled ? 0.3 : 1,
                  }}
                >
                  <Text variant="callout" style={{
                    color: edge ? ui.onAccent : date === today ? ui.accent : ui.label,
                    fontWeight: edge || date === today ? '700' : '400', fontVariant: ['tabular-nums'],
                  }}>{Number(date.slice(8))}</Text>
                  {info.badge && <Text style={{ fontSize: 9, lineHeight: 11, color: edge ? ui.onAccent : ui.labelSecondary }}>{info.badge}</Text>}
                </PressableScale>
              </View>
            );
          })}
        </View>
      ))}
    </View>
  );
}
