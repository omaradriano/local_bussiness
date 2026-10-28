import type { components } from '@lb/api-client';
import { addDays, agendaGroups, type AgendaGroup, strings, todayISO } from '@lb/shared';
import { Stack, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator } from 'react-native';
import { BookingCell } from '../../../../components/BookingCell';
import { api } from '../../../../lib/api';
import { errorText } from '../../../../lib/errors';
import { useBusiness } from '../../../../lib/useMe';
import { Button, EmptyState, Group, Notice, Screen, Text } from '../../../../ui';

type Summary = components['schemas']['BookingSummary'];
const order: AgendaGroup[] = ['overdue', 'awaitingDeposit', 'appointments', 'deliveries', 'goingOut', 'comingBack'];

const hour = (iso: string) => new Date(iso).toLocaleTimeString('es-MX', { hour: '2-digit', minute: '2-digit' });

/** Today's to-do list: overdue rentals, holds to collect, appointments, deliveries, rentals going out and coming back. */
export default function Today() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const { business } = useBusiness(businessId);
  const [rows, setRows] = useState<Summary[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const today = todayISO();

  const load = useCallback(async () => {
    setError(null);
    // Today's window, plus a lookback so rentals that should have come back (overdue) show up.
    const from = new Date(`${addDays(today, -60)}T00:00:00`).toISOString();
    const to = new Date(`${addDays(today, 1)}T00:00:00`).toISOString();
    const { data, error, response } = await api.GET('/v1/b/{businessId}/bookings', { params: { path: { businessId }, query: { from, to } } });
    if (data) setRows(data.bookings);
    else setError(errorText(error, response.status));
  }, [businessId, today]);

  useFocusEffect(useCallback(() => { load(); }, [load]));

  if (!business) return <ActivityIndicator style={{ marginTop: 24 }} />;
  const groups = rows ? agendaGroups(rows, today) : null;
  const empty = groups && order.every((k) => groups[k].length === 0);

  return (
    <Screen>
      <Stack.Screen options={{ title: strings.business.sections.today }} />
      <Text variant="footnote" color="labelSecondary" style={{ textTransform: 'capitalize' }}>
        {new Date().toLocaleDateString('es-MX', { weekday: 'long', day: 'numeric', month: 'long' })}
      </Text>
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 20 }}>{strings.business.sections.today}</Text>
      {error && (<Group padded><Notice tone="warning">{error}</Notice><Button onPress={load} style={{ marginTop: 12 }}>{strings.common.retry}</Button></Group>)}
      {!rows && !error && <ActivityIndicator />}
      {empty && <EmptyState icon="sunny-outline" title={strings.today.nothing} />}
      {groups && order.filter((k) => groups[k].length).map((k) => (
        <Group key={k} label={`${strings.today.groups[k]} (${groups[k].length})`}>
          {groups[k].map((b) => (
            <BookingCell key={`${k}-${b.id}`} businessId={businessId} b={b} time={k === 'comingBack' && b.ends_at ? hour(b.ends_at) : hour(b.starts_at)} />
          ))}
        </Group>
      ))}
    </Screen>
  );
}
