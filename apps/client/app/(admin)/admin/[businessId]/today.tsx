import type { components } from '@lb/api-client';
import { addDays, agendaGroups, type AgendaGroup, formatMXN, strings, todayISO } from '@lb/shared';
import { Link, Stack, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator, Button, ScrollView, Text, View } from 'react-native';
import { api } from '../../../../lib/api';
import { errorText } from '../../../../lib/errors';
import { useBusiness } from '../../../../lib/useMe';

type Summary = components['schemas']['BookingSummary'];
const t = strings.bookings;
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
    <ScrollView contentContainerStyle={{ padding: 16, gap: 8, maxWidth: 600, width: '100%', alignSelf: 'center' }}>
      <Stack.Screen options={{ title: strings.business.sections.today }} />
      <Text style={{ fontWeight: '600' }}>{new Date().toLocaleDateString('es-MX', { weekday: 'long', day: 'numeric', month: 'long' })}</Text>
      {error && (<><Text accessibilityRole="alert">{error}</Text><Button title={strings.common.retry} onPress={load} /></>)}
      {!rows && !error && <ActivityIndicator />}
      {empty && <Text>{strings.today.nothing}</Text>}
      {groups && order.filter((k) => groups[k].length).map((k) => (
        <View key={k} style={{ gap: 6, marginTop: 8 }}>
          <Text style={{ fontWeight: '600' }}>{strings.today.groups[k]} ({groups[k].length})</Text>
          {groups[k].map((b) => (
            <Link key={`${k}-${b.id}`} href={{ pathname: '/admin/[businessId]/bookings/[bookingId]', params: { businessId, bookingId: b.id } }}>
              <View style={{ borderWidth: 1, borderRadius: 8, padding: 10, gap: 2 }}>
                <Text style={{ fontWeight: '600' }}>
                  {k === 'comingBack' && b.ends_at ? hour(b.ends_at) : hour(b.starts_at)} · {b.items}
                  {b.units.length ? ` · ${b.units.join(', ')}` : ''}{b.staff.length ? ` · ${b.staff.join(', ')}` : ''}
                </Text>
                <Text>{b.customer_name} · {t.status[b.status]} · {formatMXN(b.total)}</Text>
              </View>
            </Link>
          ))}
        </View>
      ))}
    </ScrollView>
  );
}
