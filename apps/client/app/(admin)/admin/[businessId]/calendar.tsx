import type { components } from '@lb/api-client';
import { bookingDays, engineFor, firstOfMonth, monthWindow, strings, todayISO } from '@lb/shared';
import { Stack, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useMemo, useState } from 'react';
import { ActivityIndicator, View } from 'react-native';
import { Calendar } from '../../../../components/Calendar';
import { BookingCell } from '../../../../components/BookingCell';
import { InventoryList } from '../../../../components/InventoryList';
import { api } from '../../../../lib/api';
import { errorText } from '../../../../lib/errors';
import { useBusiness } from '../../../../lib/useMe';
import { useTexts } from '../../../../lib/useTexts';
import { Cell, Group, Notice, Screen, Segmented, Text } from '../../../../ui';

type Summary = components['schemas']['BookingSummary'];
const t = strings.bookings;

const when = (iso: string) => new Date(iso).toLocaleString('es-MX', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });

/** Month view of active bookings: count per day; tap a day to see them. */
export default function BookingCalendar() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const { business } = useBusiness(businessId);
  const { texts, has } = useTexts();
  const [month, setMonth] = useState(firstOfMonth(todayISO()));
  const [day, setDay] = useState(todayISO());
  const [list, setList] = useState<Summary[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [view, setView] = useState<'inventory' | 'month'>('inventory');

  const load = useCallback(async () => {
    setList(null);
    const { data, error, response } = await api.GET('/v1/b/{businessId}/bookings', {
      params: { path: { businessId }, query: monthWindow(month) },
    });
    if (data) setList(data.bookings);
    else setError(errorText(error, response.status));
  }, [businessId, month]);

  useFocusEffect(useCallback(() => { load(); }, [load]));

  // date -> bookings touching that day
  const byDay = useMemo(() => {
    const map: Record<string, Summary[]> = {};
    for (const b of list ?? []) for (const d of bookingDays(b.starts_at, b.ends_at)) (map[d] ??= []).push(b);
    return map;
  }, [list]);

  if (!business) return <ActivityIndicator style={{ marginTop: 24 }} />;
  const selected = byDay[day] ?? [];

  const rental = has('inventory'); // the Inventario tab
  return (
    <Screen>
      <Stack.Screen options={{ title: strings.business.sections.calendar }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 16 }}>{strings.business.sections.calendar}</Text>
      {rental && (
        <View style={{ marginBottom: 16 }}>
          <Segmented options={[{ value: 'inventory', label: strings.inventory.title }, { value: 'month', label: strings.inventory.month }]} value={view} onChange={setView} />
        </View>
      )}
      {rental && view === 'inventory' ? <InventoryList businessId={businessId} /> : (
        <>
          <Group padded>
            <Calendar month={month} onMonthChange={setMonth} from={day} onSelect={setDay}
              dayInfo={(d) => (byDay[d] ? { badge: t.countBadge(byDay[d].length) } : {})} />
          </Group>
          {error && <Notice tone="warning">{error}</Notice>}
          {list === null && !error && <ActivityIndicator />}
          <Group label={new Date(`${day}T12:00:00`).toLocaleDateString('es-MX', { weekday: 'long', day: 'numeric', month: 'long' })}>
            {list && selected.length === 0 ? <Cell label={texts.bookings.noneThatDay} /> : null}
            {selected.map((b) => (
              <BookingCell key={b.id} businessId={businessId} b={b}
                time={engineFor(b.kind).timing === 'range' && b.ends_at ? `${when(b.starts_at)} → ${when(b.ends_at)}` : when(b.starts_at)} />
            ))}
          </Group>
        </>
      )}
    </Screen>
  );
}
