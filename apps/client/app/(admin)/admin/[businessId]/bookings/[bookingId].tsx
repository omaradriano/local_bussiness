import type { components } from '@lb/api-client';
import { formatMXN, strings, term } from '@lb/shared';
import { Stack, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, ScrollView, Text } from 'react-native';
import { api } from '../../../../../lib/api';
import { errorText } from '../../../../../lib/errors';
import { useBusiness } from '../../../../../lib/useMe';

type Booking = components['schemas']['Booking'];
const t = strings.bookings;

const when = (iso: string) =>
  new Date(iso).toLocaleString('es-MX', { weekday: 'short', day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });

export default function BookingDetail() {
  const { businessId, bookingId } = useLocalSearchParams<{ businessId: string; bookingId: string }>();
  const { business } = useBusiness(businessId);
  const [b, setB] = useState<Booking | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  useEffect(() => {
    api.GET('/v1/b/{businessId}/bookings/{bookingId}', { params: { path: { businessId, bookingId } } })
      .then(({ data, error, response }) => (data ? setB(data) : setMessage(errorText(error, response.status))));
  }, [businessId, bookingId]);

  if (!b || !business) return message ? <Text style={{ padding: 16 }}>{message}</Text> : <ActivityIndicator style={{ marginTop: 24 }} />;
  const security = Number(b.items[0]?.options?.security_deposit ?? 0);

  return (
    <ScrollView contentContainerStyle={{ padding: 16, gap: 8, maxWidth: 560, width: '100%', alignSelf: 'center' }}>
      <Stack.Screen options={{ title: term(business.vertical, 'booking') ?? t.detail }} />
      <Text accessibilityRole="header" style={{ fontSize: 20, fontWeight: '600' }}>{t.status[b.status]}</Text>
      <Text>{b.items.map((i) => `${i.quantity} × ${i.name}`).join(', ')}</Text>
      <Text>{term(business.vertical, 'starts_at')}: {when(b.starts_at)}</Text>
      {b.kind === 'rental' && b.ends_at && <Text>{term(business.vertical, 'ends_at')}: {when(b.ends_at)}</Text>}
      {b.staff.length > 0 && <Text>{t.attendedBy}: {b.staff.map((s) => s.name).join(', ')}</Text>}
      {b.units.length > 0 && <Text>{t.units}: {b.units.map((u) => u.name).join(', ')}</Text>}
      {b.fulfillment === 'delivery' && <Text>{t.delivery}: {b.delivery_address}</Text>}
      <Text>{t.customer}: {b.customer.name} · {b.customer.phone}</Text>
      <Text style={{ marginTop: 8, fontWeight: '600' }}>{t.total}: {formatMXN(b.total)}</Text>
      {b.deposit_required > 0 && <Text>{t.advance}: {formatMXN(b.deposit_required)}</Text>}
      {security > 0 && <Text>{t.securityDeposit}: {formatMXN(security)}</Text>}
      {b.hold_expires_at && <Text>{t.holdUntil}: {when(b.hold_expires_at)}</Text>}
      {b.notes && <Text>{b.notes}</Text>}
    </ScrollView>
  );
}
