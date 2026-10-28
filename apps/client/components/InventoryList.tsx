import type { components } from '@lb/api-client';
import { strings } from '@lb/shared';
import { router, useFocusEffect } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator, View } from 'react-native';
import { api } from '../lib/api';
import { errorText } from '../lib/errors';
import { Cell, Chip, Chips, EmptyState, Group, Notice, useUI } from '../ui';

type Row = components['schemas']['InventoryRow'];
type State = Row['state'];
const t = strings.inventory;
const STATES: State[] = ['available', 'rented', 'scheduled', 'inactive'];
const when = (iso: string) => new Date(iso).toLocaleString('es-MX', { weekday: 'short', day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });

/** Every rental unit: free, out (with its return date), booked to go out, or inactive; sorted by date by the API. */
export function InventoryList({ businessId }: { businessId: string }) {
  const ui = useUI();
  const [rows, setRows] = useState<Row[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [shown, setShown] = useState<Record<State, boolean>>({ available: true, rented: true, scheduled: true, inactive: true });

  useFocusEffect(useCallback(() => {
    api.GET('/v1/b/{businessId}/inventory', { params: { path: { businessId } } }).then(({ data, error, response }) => {
      if (data) setRows(data.rows);
      else setError(errorText(error, response.status));
    });
  }, [businessId]));

  if (error) return <Notice tone="warning">{error}</Notice>;
  if (!rows) return <ActivityIndicator />;
  const count = (st: State) => rows.filter((r) => r.state === st).length;
  const visible = rows.filter((r) => shown[r.state]);
  const tint = { available: ui.success, rented: ui.accent, scheduled: ui.warning, inactive: ui.gray } as const;
  const icon = { available: 'checkmark-circle', rented: 'arrow-up-circle', scheduled: 'calendar', inactive: 'construct' } as const;

  return (
    <View>
      <View style={{ marginBottom: 16 }}>
        <Chips>
          {STATES.map((st) => (
            <Chip key={st} selected={shown[st]} onPress={() => setShown({ ...shown, [st]: !shown[st] })}>{`${t.states[st]} ${count(st)}`}</Chip>
          ))}
        </Chips>
      </View>
      {rows.length === 0 && <EmptyState icon="construct-outline" title={t.noUnits} />}
      {rows.length > 0 && visible.length === 0 && <EmptyState icon="filter-outline" title={t.empty} />}
      {visible.length > 0 && (
        <Group separatorInset={58}>
          {visible.map((r, i) => {
            const sub = r.state === 'rented' && r.date ? `${t.returns}: ${when(r.date)}${r.booking_status === 'overdue' ? ` · ${strings.bookings.status.overdue}` : ''}`
              : r.state === 'scheduled' && r.starts_at && r.ends_at ? `${t.goesOut}: ${when(r.starts_at)} → ${when(r.ends_at)}`
                : r.state === 'inactive' ? strings.units.status[r.unit_status] : t.states.available;
            return (
              <Cell key={`${r.unit_id}-${r.booking_id ?? i}`} icon={icon[r.state]} tint={r.booking_status === 'overdue' ? ui.danger : tint[r.state]}
                label={`${r.item_name} ${r.unit_code}`} sublabel={r.customer_name ? `${sub} · ${r.customer_name}` : sub}
                onPress={() => (r.booking_id
                  ? router.push({ pathname: '/admin/[businessId]/bookings/[bookingId]', params: { businessId, bookingId: r.booking_id } })
                  : router.push({ pathname: '/admin/[businessId]/catalog/[itemId]', params: { businessId, itemId: r.item_id } }))} />
            );
          })}
        </Group>
      )}
    </View>
  );
}
