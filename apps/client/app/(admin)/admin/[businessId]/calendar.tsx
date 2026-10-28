import type { components } from '@lb/api-client';
import { bookingDays, firstOfMonth, formatMXN, monthWindow, strings, term, todayISO } from '@lb/shared';
import { Link, Stack, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useMemo, useState } from 'react';
import { ActivityIndicator, Pressable, ScrollView, Text, View } from 'react-native';
import { Calendar } from '../../../../components/Calendar';
import { InventoryList } from '../../../../components/InventoryList';
import { api } from '../../../../lib/api';
import { errorText } from '../../../../lib/errors';
import { useBusiness } from '../../../../lib/useMe';

type Summary = components['schemas']['BookingSummary'];
const t = strings.bookings;

const when = (iso: string) => new Date(iso).toLocaleString('es-MX', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });

/** Month view of active bookings: count per day; tap a day to see them. */
export default function BookingCalendar() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const { business } = useBusiness(businessId);
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

  return (
    <ScrollView contentContainerStyle={{ padding: 16, gap: 12, maxWidth: 560, width: '100%', alignSelf: 'center' }}>
      <Stack.Screen options={{ title: strings.business.sections.calendar }} />
      {business.vertical === 'tool_rental' && (
        <View accessibilityRole="tablist" style={{ flexDirection: 'row', gap: 8 }}>
          {(['inventory', 'month'] as const).map((v) => (
            <Pressable key={v} accessibilityRole="tab" accessibilityState={{ selected: view === v }} onPress={() => setView(v)}
              style={{ borderBottomWidth: view === v ? 2 : 0, paddingVertical: 6, paddingHorizontal: 10 }}>
              <Text style={{ fontWeight: view === v ? '600' : '400' }}>{v === 'inventory' ? strings.inventory.title : strings.inventory.month}</Text>
            </Pressable>
          ))}
        </View>
      )}
      {business.vertical === 'tool_rental' && view === 'inventory' ? <InventoryList businessId={businessId} /> : (<>
      <Calendar
        month={month}
        onMonthChange={setMonth}
        from={day}
        onSelect={setDay}
        dayInfo={(d) => (byDay[d] ? { badge: t.countBadge(byDay[d].length) } : {})}
      />
      {error && <Text accessibilityRole="alert">{error}</Text>}
      {list === null && !error && <ActivityIndicator />}
      <Text style={{ fontWeight: '600' }}>{new Date(`${day}T12:00:00`).toLocaleDateString('es-MX', { weekday: 'long', day: 'numeric', month: 'long' })}</Text>
      {list && selected.length === 0 && <Text>{t.noneThatDay}</Text>}
      {selected.map((b) => (
        <Link key={b.id} href={{ pathname: '/admin/[businessId]/bookings/[bookingId]', params: { businessId, bookingId: b.id } }}>
          <View style={{ borderWidth: 1, borderRadius: 8, padding: 10, gap: 2 }}>
            <Text style={{ fontWeight: '600' }}>{b.items}{b.units.length ? ` · ${b.units.join(', ')}` : ''}{b.staff.length ? ` · ${b.staff.join(', ')}` : ''}</Text>
            <Text>{b.customer_name} · {t.status[b.status]}</Text>
            <Text>
              {term(business.vertical, 'starts_at')}: {when(b.starts_at)}
              {b.kind === 'rental' && b.ends_at ? `  →  ${term(business.vertical, 'ends_at')}: ${when(b.ends_at)}` : ''}
            </Text>
            <Text>{t.total}: {formatMXN(b.total)}</Text>
          </View>
        </Link>
      ))}
      </>)}
    </ScrollView>
  );
}
