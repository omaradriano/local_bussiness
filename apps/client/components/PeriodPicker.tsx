import { Ionicons } from '@expo/vector-icons';
import { firstOfMonth, periodLabel, periodOf, shiftPeriod, strings, todayISO, type PeriodKind } from '@lb/shared';
import { useState } from 'react';
import { View } from 'react-native';
import { PressableScale, Segmented, Text, useUI } from '../ui';
import { Calendar } from './Calendar';

const t = strings.finances;
export type Period = { kind: PeriodKind; from: string; to: string };

export function currentMonth(): Period {
  return { kind: 'month', ...periodOf('month', todayISO()) };
}

/** Week / month / year with ‹ › to move, or a custom range picked on the calendar (monthOnly: just months, for Lite). */
export function PeriodPicker({ value, onChange, monthOnly }: { value: Period; onChange: (p: Period) => void; monthOnly?: boolean }) {
  const ui = useUI();
  const [month, setMonth] = useState(firstOfMonth(value.from));
  const [pending, setPending] = useState<string | null>(null); // range: first day picked

  function pickKind(kind: PeriodKind) {
    if (kind === 'range') return onChange({ ...value, kind });
    onChange({ kind, ...periodOf(kind, todayISO()) });
  }

  return (
    <View style={{ gap: 12 }}>
      {!monthOnly && <Segmented options={(['week', 'month', 'year', 'range'] as PeriodKind[]).map((k) => ({ value: k, label: t.periods[k] }))} value={value.kind} onChange={pickKind} />}
      {value.kind !== 'range' ? (
        <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' }}>
          <PressableScale accessibilityRole="button" accessibilityLabel="Periodo anterior" hitSlop={10} style={{ padding: 6 }}
            onPress={() => onChange({ kind: value.kind, ...shiftPeriod(value.kind as Exclude<PeriodKind, 'range'>, value.from, -1) })}>
            <Ionicons name="chevron-back" size={22} color={ui.accent} />
          </PressableScale>
          <Text variant="headline" style={{ textTransform: 'capitalize' }}>{periodLabel(value.kind, value.from, value.to)}</Text>
          <PressableScale accessibilityRole="button" accessibilityLabel="Periodo siguiente" hitSlop={10} style={{ padding: 6 }}
            onPress={() => onChange({ kind: value.kind, ...shiftPeriod(value.kind as Exclude<PeriodKind, 'range'>, value.from, 1) })}>
            <Ionicons name="chevron-forward" size={22} color={ui.accent} />
          </PressableScale>
        </View>
      ) : (
        <View style={{ gap: 8 }}>
          <Text variant="subhead" color="labelSecondary">{pending ? t.pickRange : periodLabel('range', value.from, value.to)}</Text>
          <Calendar month={month} onMonthChange={setMonth} from={pending ?? value.from} to={pending ? null : value.to}
            onSelect={(d) => {
              if (!pending) return setPending(d);
              const [from, to] = d < pending ? [d, pending] : [pending, d];
              setPending(null);
              onChange({ kind: 'range', from, to });
            }} />
        </View>
      )}
    </View>
  );
}
