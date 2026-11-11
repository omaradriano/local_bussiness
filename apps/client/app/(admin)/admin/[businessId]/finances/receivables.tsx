import type { components } from '@lb/api-client';
import { formatMXN, formatPhone, strings } from '@lb/shared';
import { router, Stack, useFocusEffect, useLocalSearchParams, type Href } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator, View } from 'react-native';
import { api } from '../../../../../lib/api';
import { errorText } from '../../../../../lib/errors';
import { useTexts } from '../../../../../lib/useTexts';
import { Button, Cell, Group, Notice, Row, Screen, useUI } from '../../../../../ui';

type Data = components['schemas']['Receivables'];
type Item = components['schemas']['Receivable'];
const t = strings.finances.receivables;

const day = (iso: string) => new Date(iso).toLocaleDateString('es-MX', { day: 'numeric', month: 'short' });

/** Por cobrar: who owes (on credit or not handed over yet) and the advances taken for what's still to deliver. */
export default function Receivables() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const { texts } = useTexts();
  const ui = useUI();
  const [data, setData] = useState<Data | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    setError(null);
    api.GET('/v1/b/{businessId}/finances/receivables', { params: { path: { businessId } } })
      .then(({ data: d, error: e, response }) => (d ? setData(d) : setError(errorText(e, response.status))));
  }, [businessId]);
  useFocusEffect(load);

  const row = (x: Item, tint: string) => (
    <Cell key={x.booking_id} icon="person" tint={tint} label={x.customer}
      sublabel={`${day(x.starts_at)} · ${texts.bookings.status[x.status]} · ${formatPhone(x.phone)}`}
      value={formatMXN(x.due)} onPress={() => router.push(`/admin/${businessId}/bookings/${x.booking_id}` as Href)} />
  );

  return (
    <Screen>
      <Stack.Screen options={{ title: t.title }} />
      {error && (<Group padded><Notice tone="warning">{error}</Notice><Button onPress={load} style={{ marginTop: 12 }}>{strings.common.retry}</Button></Group>)}
      {!data && !error && <ActivityIndicator />}
      {data && (
        <View style={{ gap: 16 }}>
          <Group padded>
            <View style={{ gap: 6 }}>
              <Row label={t.totalDue} value={formatMXN(data.total_due)} strong />
              <Row label={t.advances(data.advance_count)} value={formatMXN(data.advances)} />
            </View>
          </Group>
          <Group label={t.credit} footer={t.creditHint}>
            {data.credit.length === 0 ? <Cell label={t.none} /> : data.credit.map((x) => row(x, ui.danger))}
          </Group>
          <Group label={t.active} footer={t.activeHint}>
            {data.active.length === 0 ? <Cell label={t.none} /> : data.active.map((x) => row(x, ui.warning))}
          </Group>
        </View>
      )}
    </Screen>
  );
}
