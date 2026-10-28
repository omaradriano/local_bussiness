import type { components } from '@lb/api-client';
import { strings } from '@lb/shared';
import { useCallback, useEffect, useRef, useState } from 'react';
import { ActivityIndicator, View } from 'react-native';
import { api } from '../lib/api';
import { errorText } from '../lib/errors';
import { useTexts } from '../lib/useTexts';
import { Button, EmptyState, Field, Group, Notice, Segmented } from '../ui';
import { BookingCell } from './BookingCell';

type Summary = components['schemas']['BookingSummary'];
type HistoryGroup = 'all' | 'active' | 'finished' | 'cancelled';
const t = strings.history;
const PAGE = 30;

const day = (iso: string) => new Date(iso).toLocaleDateString('es-MX', { day: 'numeric', month: 'short', year: 'numeric' });
const monthKey = (iso: string) => new Date(iso).toLocaleDateString('es-MX', { month: 'long', year: 'numeric' });

/** Bookings newest first, grouped by month; filter by group and search (or one customer's). Pages of 30. */
export function BookingHistory({ businessId, customerId, compact }: { businessId: string; customerId?: string; compact?: boolean }) {
  const [group, setGroup] = useState<HistoryGroup>('all');
  const { texts } = useTexts();
  const [q, setQ] = useState('');
  const [rows, setRows] = useState<Summary[] | null>(null);
  const [more, setMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const seq = useRef(0); // ignore answers to an older search

  const load = useCallback(async (after?: Summary) => {
    const n = ++seq.current;
    setBusy(true);
    const { data, error: err, response } = await api.GET('/v1/b/{businessId}/bookings/history', {
      params: { path: { businessId }, query: {
        limit: PAGE, ...(group !== 'all' ? { group } : {}), ...(q.trim() ? { q: q.trim() } : {}), ...(customerId ? { customer_id: customerId } : {}),
        ...(after ? { before: after.starts_at, before_id: after.id } : {}),
      } },
    });
    if (n !== seq.current) return;
    setBusy(false);
    if (!data) return setError(errorText(err, response.status));
    setError(null);
    setRows((prev) => (after ? [...(prev ?? []), ...data.bookings] : data.bookings));
    setMore(data.bookings.length === PAGE);
  }, [businessId, customerId, group, q]);

  useEffect(() => {
    const id = setTimeout(() => load(), q ? 250 : 0);
    return () => clearTimeout(id);
  }, [load]); // eslint-disable-line react-hooks/exhaustive-deps

  const months: [string, Summary[]][] = [];
  for (const r of rows ?? []) {
    const k = monthKey(r.starts_at);
    if (months.at(-1)?.[0] !== k) months.push([k, []]);
    months.at(-1)![1].push(r);
  }

  return (
    <View>
      {!compact && <Field label={t.search} placeholder={texts.bookings.searchPlaceholder} value={q} onChangeText={setQ} autoCorrect={false} />}
      <View style={{ marginBottom: 16 }}>
        <Segmented options={(['all', 'active', 'finished', 'cancelled'] as HistoryGroup[]).map((g) => ({ value: g, label: t.groups[g] }))} value={group} onChange={setGroup} />
      </View>
      {error && <Notice tone="warning">{error}</Notice>}
      {rows === null && !error && <ActivityIndicator />}
      {rows?.length === 0 && <EmptyState icon="file-tray-outline" title={texts.bookings.empty} />}
      {months.map(([month, list]) => (
        <Group key={month} label={month}>
          {list.map((b) => (
            <BookingCell key={b.id} businessId={businessId} b={b}
              time={b.kind === 'rental' && b.ends_at ? `${day(b.starts_at)} → ${day(b.ends_at)}` : day(b.starts_at)} />
          ))}
        </Group>
      ))}
      {more && <Button busy={busy} onPress={() => rows && load(rows[rows.length - 1])}>{t.loadMore}</Button>}
    </View>
  );
}
