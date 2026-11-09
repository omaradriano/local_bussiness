import type { components } from '@lb/api-client';
import { addDays, boardColumns, dayStartISO, forwardMove, hhmm, strings, todayISO, type BoardStatus } from '@lb/shared';
import { Ionicons } from '@expo/vector-icons';
import { router, Stack, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator, useWindowDimensions, View } from 'react-native';
import { api } from '../../../../lib/api';
import { errorText } from '../../../../lib/errors';
import { useBusiness } from '../../../../lib/useMe';
import { useTexts } from '../../../../lib/useTexts';
import { DayNav } from '../../../../components/DayNav';
import { Button, Chip, Chips, EmptyState, Notice, PressableScale, Screen, Text, radius, useToast, useUI } from '../../../../ui';

type Summary = components['schemas']['BookingSummary'];
const t = strings.board;
const WIDE = 900; // columns side by side from here; one tab per column below

/** Day board (feature dayBoard): the day's orders by status, each moved ahead with one tap. */
export default function Board() {
  const { businessId, day: dayParam } = useLocalSearchParams<{ businessId: string; day?: string }>();
  const { business } = useBusiness(businessId);
  const { texts } = useTexts();
  const toast = useToast();
  const { width } = useWindowDimensions();
  const [day, setDay] = useState(dayParam ?? todayISO());
  const [rows, setRows] = useState<Summary[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [tab, setTab] = useState<BoardStatus>('confirmed');
  const [busyId, setBusyId] = useState<string | null>(null);

  const tz = business?.timezone;
  const load = useCallback(async () => {
    if (!tz) return; // the day starts at midnight in the business zone
    setError(null);
    const from = dayStartISO(day, tz);
    const to = dayStartISO(addDays(day, 1), tz);
    const { data, error, response } = await api.GET('/v1/b/{businessId}/bookings', { params: { path: { businessId }, query: { from, to, status: 'all' } } });
    if (data) setRows(data.bookings);
    else setError(errorText(error, response.status));
  }, [businessId, day, tz]);

  useFocusEffect(useCallback(() => { setRows(null); load(); }, [load]));

  async function advance(b: Summary, to: string) {
    setBusyId(b.id);
    const { data, error, response } = await api.POST('/v1/b/{businessId}/bookings/{bookingId}/transition', {
      params: { path: { businessId, bookingId: b.id } }, body: { to: to as Summary['status'] },
    });
    setBusyId(null);
    if (!data) return toast(error?.message ?? errorText(error, response.status), 'error');
    toast(t.moved(t.columns[to] ?? to));
    load();
  }

  if (!business) return <ActivityIndicator style={{ marginTop: 24 }} />;
  const wide = width >= WIDE;
  const columns = rows ? boardColumns(rows) : [];
  const shown = wide ? columns : columns.filter((c) => c.status === tab);

  return (
    <Screen refreshing={false} onRefresh={load}
      contentContainerStyle={wide ? { padding: 16, paddingBottom: 40, width: '100%', maxWidth: 1400, alignSelf: 'center' } : undefined}>
      <Stack.Screen options={{ title: t.title }} />
      <Text variant="largeTitle" accessibilityRole="header">{t.title}</Text>
      <Text variant="footnote" color="labelSecondary" style={{ marginBottom: 12 }}>{t.hint}</Text>

      <DayNav day={day} onChange={setDay} />
      <Button size="sm" icon="document-text" style={{ marginBottom: 12 }}
        onPress={() => router.push({ pathname: '/admin/[businessId]/production', params: { businessId, day } })}>{strings.production.open}</Button>

      {error && <Notice tone="warning">{error}</Notice>}
      {!rows && !error && <ActivityIndicator />}
      {rows && columns.every((c) => c.rows.length === 0) && <EmptyState icon="flower-outline" title={t.none} />}

      {rows && columns.some((c) => c.rows.length > 0) && (
        <>
          {!wide && (
            <View style={{ marginBottom: 12 }}>
              <Chips>
                {columns.map((c) => <Chip key={c.status} selected={c.status === tab} onPress={() => setTab(c.status)}>{`${t.columns[c.status]} · ${c.rows.length}`}</Chip>)}
              </Chips>
            </View>
          )}
          <View style={{ flexDirection: wide ? 'row' : 'column', gap: 12, alignItems: 'flex-start' }}>
            {shown.map((c) => (
              <View key={c.status} style={{ flex: wide ? 1 : undefined, width: wide ? undefined : '100%', minWidth: 0, gap: 8 }}>
                {wide && <Text variant="footnote" color="labelSecondary">{`${t.columns[c.status].toUpperCase()} · ${c.rows.length}`}</Text>}
                {c.rows.length === 0 && <Text variant="footnote" color="labelSecondary">{t.empty}</Text>}
                {c.rows.map((b) => (
                  <OrderCard key={b.id} businessId={businessId} b={b} busy={busyId === b.id} statusText={texts.bookings.status[b.status]} onAdvance={advance} />
                ))}
              </View>
            ))}
          </View>
        </>
      )}
    </Screen>
  );
}

function OrderCard({ businessId, b, busy, statusText, onAdvance }: {
  businessId: string; b: Summary; busy: boolean; statusText: string; onAdvance: (b: Summary, to: string) => void;
}) {
  const ui = useUI();
  const to = forwardMove(b.next, b.fulfillment);
  const time = `${hhmm(b.starts_at)}${b.ends_at ? `–${hhmm(b.ends_at)}` : ''}`;
  return (
    <View style={{ backgroundColor: ui.surface, borderRadius: radius.lg, padding: 12, gap: 6 }}>
      <PressableScale accessibilityRole="button" accessibilityLabel={`${b.items}, ${b.customer_name}, ${statusText}`}
        onPress={() => router.push({ pathname: '/admin/[businessId]/bookings/[bookingId]', params: { businessId, bookingId: b.id } })}>
        <View style={{ gap: 4 }}>
          <View style={{ flexDirection: 'row', alignItems: 'center', gap: 6 }}>
            <Text variant="headline" style={{ fontVariant: ['tabular-nums'] }}>{time}</Text>
            <Ionicons name={b.fulfillment === 'delivery' ? 'car' : 'storefront'} size={14} color={ui.labelSecondary} />
            <Text variant="footnote" color="labelSecondary">{b.fulfillment === 'delivery' ? t.delivery : t.pickup}</Text>
          </View>
          <Text>{b.items}</Text>
          {!!b.details && <Text variant="footnote" color="labelSecondary">{b.details}</Text>}
          <Text variant="footnote" color="labelSecondary">{b.recipient_name ? `${b.customer_name} · ${t.for(b.recipient_name)}` : b.customer_name}</Text>
          {!!b.card_message && <Text variant="footnote" numberOfLines={2} style={{ fontStyle: 'italic' }}>«{b.card_message}»</Text>}
        </View>
      </PressableScale>
      {to && (
        <Button size="sm" variant={to === 'completed' ? 'primary' : undefined} busy={busy} onPress={() => onAdvance(b, to)}>
          {t.actions[to] ?? strings.bookings.actions[to] ?? to}
        </Button>
      )}
    </View>
  );
}
