import { addDays, capFirst, strings, todayISO } from '@lb/shared';
import { Ionicons } from '@expo/vector-icons';
import { View } from 'react-native';
import { Button, PressableScale, Text, radius, useUI } from '../ui';

const t = strings.board;

/** Day selector of the day screens (tablero, hoja de producción): ‹ día › and a way back to today. */
export function DayNav({ day, onChange }: { day: string; onChange: (day: string) => void }) {
  const date = capFirst(new Date(`${day}T12:00:00`).toLocaleDateString('es-MX', { weekday: 'long', day: 'numeric', month: 'long' }));
  const label = day === todayISO() ? `${t.today} · ${date}` : date;
  return (
    <View style={{ gap: 8, marginBottom: 12 }}>
      <View style={{ flexDirection: 'row', alignItems: 'center', gap: 8 }}>
        <DayButton icon="chevron-back" label={t.prevDay} onPress={() => onChange(addDays(day, -1))} />
        <Text variant="headline" style={{ flex: 1, textAlign: 'center' }}>{label}</Text>
        <DayButton icon="chevron-forward" label={t.nextDay} onPress={() => onChange(addDays(day, 1))} />
      </View>
      {day !== todayISO() && <Button size="sm" onPress={() => onChange(todayISO())}>{t.today}</Button>}
    </View>
  );
}

function DayButton({ icon, label, onPress }: { icon: 'chevron-back' | 'chevron-forward'; label: string; onPress: () => void }) {
  const ui = useUI();
  return (
    <PressableScale accessibilityRole="button" accessibilityLabel={label} onPress={onPress} hitSlop={8}
      style={{ width: 36, height: 36, borderRadius: radius.full, backgroundColor: ui.fill, alignItems: 'center', justifyContent: 'center' }}>
      <Ionicons name={icon} size={20} color={ui.accent} />
    </PressableScale>
  );
}
