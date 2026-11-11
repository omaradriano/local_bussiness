import type { components } from '@lb/api-client';
import { addDays, agendaGroups, dayStartISO, type AgendaGroup, strings, todayISO, capFirst } from '@lb/shared';
import { Ionicons } from '@expo/vector-icons';
import { router, Stack, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator, Linking, Pressable } from 'react-native';
import { BookingCell } from '../../../../components/BookingCell';
import { monthDay } from '../../../../components/CustomerDates';
import { api } from '../../../../lib/api';
import { errorText } from '../../../../lib/errors';
import { useBusiness } from '../../../../lib/useMe';
import { Button, Cell, EmptyState, Group, Notice, Screen, Text, useUI } from '../../../../ui';

type Summary = components['schemas']['BookingSummary'];
type DateSoon = components['schemas']['CustomerDate'] & { customer_id: string; customer_name: string; phone: string; days_left: number };
const order: AgendaGroup[] = ['overdue', 'late', 'awaitingDeposit', 'appointments', 'deliveries', 'goingOut', 'comingBack'];

const hour = (iso: string) => new Date(iso).toLocaleTimeString('es-MX', { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' });
const dayHour = (iso: string) => `${new Date(iso).toLocaleDateString('es-MX', { day: 'numeric', month: 'short' })} ${hour(iso)}`;

/** Today's to-do list: overdue rentals, holds to collect, appointments, deliveries, rentals going out and coming back. */
export default function Today() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const { business } = useBusiness(businessId);
  const ui = useUI();
  const [rows, setRows] = useState<Summary[] | null>(null);
  const [dates, setDates] = useState<DateSoon[]>([]);
  const [error, setError] = useState<string | null>(null);
  const today = todayISO();

  const tz = business?.timezone;
  const load = useCallback(async () => {
    if (!tz) return; // the day starts at midnight in the business zone
    setError(null);
    // Today's window, plus a lookback so rentals that should have come back (overdue) show up.
    const from = dayStartISO(addDays(today, -60), tz);
    const to = dayStartISO(addDays(today, 1), tz);
    const { data, error, response } = await api.GET('/v1/b/{businessId}/bookings', { params: { path: { businessId }, query: { from, to } } });
    if (data) setRows(data.bookings);
    else setError(errorText(error, response.status));
    api.GET('/v1/b/{businessId}/customer-dates/upcoming', { params: { path: { businessId } } }).then(({ data: d }) => setDates(d?.dates ?? []));
  }, [businessId, today, tz]);

  useFocusEffect(useCallback(() => { load(); }, [load]));

  if (!business) return <ActivityIndicator style={{ marginTop: 24 }} />;
  const groups = rows ? agendaGroups(rows, today) : null;
  const empty = groups && order.every((k) => groups[k].length === 0) && dates.length === 0;

  return (
    <Screen>
      <Stack.Screen options={{ title: strings.business.sections.today }} />
      <Text variant="footnote" color="labelSecondary">
        {capFirst(new Date().toLocaleDateString('es-MX', { weekday: 'long', day: 'numeric', month: 'long' }))}
      </Text>
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 20 }}>{strings.business.sections.today}</Text>
      {error && (<Group padded><Notice tone="warning">{error}</Notice><Button onPress={load} style={{ marginTop: 12 }}>{strings.common.retry}</Button></Group>)}
      {!rows && !error && <ActivityIndicator />}
      {empty && <EmptyState icon="sunny-outline" title={strings.today.nothing} />}
      {groups && order.filter((k) => groups[k].length).map((k) => (
        <Group key={k} label={`${strings.today.groups[k]} (${groups[k].length})`}>
          {groups[k].map((b) => (
            <BookingCell key={`${k}-${b.id}`} businessId={businessId} b={b} time={k === 'comingBack' && b.ends_at ? hour(b.ends_at) : k === 'late' ? dayHour(b.starts_at) : hour(b.starts_at)} />
          ))}
        </Group>
      ))}
      {dates.length > 0 && (
        <Group label={`${strings.today.dates} (${dates.length})`} footer={strings.today.datesHint}>
          {dates.map((x) => (
            <Cell key={x.id} icon="gift" tint={ui.purple} label={`${x.label} · ${x.customer_name}`}
              sublabel={`${strings.today.inDays(x.days_left)} · ${monthDay(x.month, x.day)}`}
              onPress={() => router.push({ pathname: '/admin/[businessId]/customers/[customerId]', params: { businessId, customerId: x.customer_id } })}
              accessory={<Pressable accessibilityRole="button" accessibilityLabel={strings.today.whatsapp(x.customer_name)} hitSlop={10}
                onPress={() => Linking.openURL(`https://wa.me/${x.phone.replace(/\D/g, '')}?text=${encodeURIComponent(strings.today.dateMessage(x.customer_name, x.label))}`)}>
                <Ionicons name="logo-whatsapp" size={22} color={ui.success} />
              </Pressable>} />
          ))}
        </Group>
      )}
    </Screen>
  );
}
