import { addMonths, inRange, monthGrid, monthLabel, WEEKDAYS_SHORT } from '@lb/shared';
import { Pressable, Text, View } from 'react-native';

export type DayInfo = { disabled?: boolean; badge?: string };

/**
 * Month grid (Monday first). Days come as "YYYY-MM-DD". The parent decides what each tap means
 * (single day, or start/end of a range) and which days are disabled or carry a badge.
 */
export function Calendar({ month, onMonthChange, dayInfo, from, to, onSelect, label }: {
  month: string; // first day of the visible month
  onMonthChange: (m: string) => void;
  dayInfo?: (date: string) => DayInfo;
  from: string | null;
  to?: string | null;
  onSelect: (date: string) => void;
  label?: string;
}) {
  return (
    <View accessibilityLabel={label} style={{ gap: 6, maxWidth: 420 }}>
      <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' }}>
        <Pressable accessibilityRole="button" accessibilityLabel="Mes anterior" hitSlop={10} onPress={() => onMonthChange(addMonths(month, -1))}>
          <Text style={{ fontSize: 20, paddingHorizontal: 8 }}>‹</Text>
        </Pressable>
        <Text style={{ fontWeight: '600' }}>{monthLabel(month)}</Text>
        <Pressable accessibilityRole="button" accessibilityLabel="Mes siguiente" hitSlop={10} onPress={() => onMonthChange(addMonths(month, 1))}>
          <Text style={{ fontSize: 20, paddingHorizontal: 8 }}>›</Text>
        </Pressable>
      </View>
      <View style={{ flexDirection: 'row' }}>
        {WEEKDAYS_SHORT.map((w, i) => (
          <Text key={i} style={{ flex: 1, textAlign: 'center', opacity: 0.6 }}>{w}</Text>
        ))}
      </View>
      {monthGrid(month).map((week, wi) => (
        <View key={wi} style={{ flexDirection: 'row', gap: 2 }}>
          {week.map((date, di) => {
            if (!date) return <View key={di} style={{ flex: 1, aspectRatio: 1 }} />;
            const info = dayInfo?.(date) ?? {};
            const selected = inRange(date, from, to ?? null);
            const edge = date === from || date === to;
            return (
              <Pressable
                key={date}
                accessibilityRole="button"
                accessibilityLabel={`${date}${info.badge ? `, ${info.badge}` : ''}${info.disabled ? ', no disponible' : ''}`}
                accessibilityState={{ selected, disabled: !!info.disabled }}
                disabled={info.disabled}
                onPress={() => onSelect(date)}
                style={{
                  flex: 1, aspectRatio: 1, borderRadius: 6, alignItems: 'center', justifyContent: 'center',
                  borderWidth: edge ? 2 : selected ? 1 : 0, backgroundColor: selected ? '#00000014' : 'transparent',
                  opacity: info.disabled ? 0.3 : 1,
                }}
              >
                <Text style={{ fontWeight: edge ? '700' : '400' }}>{Number(date.slice(8))}</Text>
                {info.badge && <Text style={{ fontSize: 10 }}>{info.badge}</Text>}
              </Pressable>
            );
          })}
        </View>
      ))}
    </View>
  );
}
