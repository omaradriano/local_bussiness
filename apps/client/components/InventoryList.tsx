import type { components } from '@lb/api-client';
import { strings } from '@lb/shared';
import { Link, useFocusEffect } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator, Pressable, Text, View } from 'react-native';
import { api } from '../lib/api';
import { errorText } from '../lib/errors';

type Row = components['schemas']['InventoryRow'];
type State = Row['state'];
const t = strings.inventory;
const STATES: State[] = ['available', 'rented', 'scheduled', 'inactive'];
const when = (iso: string) => new Date(iso).toLocaleString('es-MX', { weekday: 'short', day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });

/** Every rental unit: free, out (with its return date), booked to go out, or inactive; sorted by date by the API. */
export function InventoryList({ businessId }: { businessId: string }) {
  const [rows, setRows] = useState<Row[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [shown, setShown] = useState<Record<State, boolean>>({ available: true, rented: true, scheduled: true, inactive: true });

  useFocusEffect(useCallback(() => {
    api.GET('/v1/b/{businessId}/inventory', { params: { path: { businessId } } }).then(({ data, error, response }) => {
      if (data) setRows(data.rows);
      else setError(errorText(error, response.status));
    });
  }, [businessId]));

  if (error) return <Text accessibilityRole="alert">{error}</Text>;
  if (!rows) return <ActivityIndicator />;
  const count = (s: State) => rows.filter((r) => r.state === s).length;
  const visible = rows.filter((r) => shown[r.state]);

  return (
    <View style={{ gap: 10 }}>
      <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
        {STATES.map((s) => (
          <Pressable key={s} accessibilityRole="checkbox" accessibilityState={{ checked: shown[s] }} onPress={() => setShown({ ...shown, [s]: !shown[s] })}
            style={{ borderWidth: shown[s] ? 2 : 1, borderRadius: 16, paddingVertical: 6, paddingHorizontal: 12, opacity: shown[s] ? 1 : 0.6 }}>
            <Text>{shown[s] ? '✓ ' : ''}{t.states[s]} ({count(s)})</Text>
          </Pressable>
        ))}
      </View>
      {rows.length === 0 && <Text>{t.noUnits}</Text>}
      {rows.length > 0 && visible.length === 0 && <Text>{t.empty}</Text>}
      {visible.map((r, i) => {
        const body = (
          <View style={{ borderWidth: 1, borderRadius: 8, padding: 10, gap: 2 }}>
            <Text style={{ fontWeight: '600' }}>{t.states[r.state]} · {r.item_name} {r.unit_code}</Text>
            {r.state === 'rented' && r.date && <Text>{t.returns}: {when(r.date)}{r.booking_status === 'overdue' ? ` · ${strings.bookings.status.overdue}` : ''}</Text>}
            {r.state === 'scheduled' && r.starts_at && r.ends_at && <Text>{t.goesOut}: {when(r.starts_at)} → {t.returns}: {when(r.ends_at)}</Text>}
            {r.state === 'inactive' && <Text>{strings.units.status[r.unit_status]}</Text>}
            {r.customer_name && <Text>{r.customer_name}</Text>}
          </View>
        );
        return r.booking_id ? (
          <Link key={`${r.unit_id}-${r.booking_id}`} href={{ pathname: '/admin/[businessId]/bookings/[bookingId]', params: { businessId, bookingId: r.booking_id } }}>{body}</Link>
        ) : (
          <Link key={`${r.unit_id}-${i}`} href={{ pathname: '/admin/[businessId]/catalog/[itemId]', params: { businessId, itemId: r.item_id } }}>{body}</Link>
        );
      })}
    </View>
  );
}
