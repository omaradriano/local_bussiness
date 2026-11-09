import type { components } from '@lb/api-client';
import { addDays, dayStartISO, displayPhone, formatMXN, hhmm, productionSheet, routeSheet, strings, todayISO } from '@lb/shared';
import { Stack, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Platform, StyleSheet, View } from 'react-native';
import { api } from '../../../../lib/api';
import { errorText } from '../../../../lib/errors';
import { useBusiness } from '../../../../lib/useMe';
import { DayNav } from '../../../../components/DayNav';
import { Button, EmptyState, Notice, Screen, Text, radius, useUI } from '../../../../ui';

type Summary = components['schemas']['BookingSummary'];
const t = strings.production;

// On paper only the sheet: no sidebar (role menu), no buttons, and the scroll areas grow to fit every page.
const PRINT_CSS = '@media print { [role="menu"], [role="button"] { display: none !important; } * { overflow: visible !important; } html, body, #root { height: auto !important; } }';

/** Production sheet: what to make on a day (added up) and each order with its card, ready to print. */
export default function Production() {
  const { businessId, day: dayParam } = useLocalSearchParams<{ businessId: string; day?: string }>();
  const { business } = useBusiness(businessId);
  const ui = useUI();
  const [day, setDay] = useState(dayParam ?? todayISO());
  const [rows, setRows] = useState<Summary[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (Platform.OS !== 'web') return;
    const style = document.createElement('style');
    style.textContent = PRINT_CSS;
    document.head.appendChild(style);
    return () => style.remove();
  }, []);

  const tz = business?.timezone;
  useFocusEffect(useCallback(() => {
    if (!tz) return; // the day starts at midnight in the business zone
    setRows(null);
    setError(null);
    const from = dayStartISO(day, tz);
    const to = dayStartISO(addDays(day, 1), tz);
    api.GET('/v1/b/{businessId}/bookings', { params: { path: { businessId }, query: { from, to, status: 'all' } } })
      .then(({ data, error, response }) => (data ? setRows(data.bookings) : setError(errorText(error, response.status))));
  }, [businessId, day, tz]));

  if (!business) return <ActivityIndicator style={{ marginTop: 24 }} />;
  const sheet = rows ? productionSheet(rows) : null;
  const route = rows ? routeSheet(rows) : null;
  const empty = sheet && route && sheet.toMake.length === 0 && route.zones.length === 0 && route.pickups.length === 0;
  const line = { borderTopWidth: StyleSheet.hairlineWidth, borderColor: ui.separator };
  const card = { backgroundColor: ui.surface, borderRadius: radius.md, padding: 16 };

  return (
    <Screen>
      <Stack.Screen options={{ title: t.title }} />
      <Text variant="footnote" color="labelSecondary">{business.name}</Text>
      <Text variant="largeTitle" accessibilityRole="header">{t.title}</Text>
      <DayNav day={day} onChange={setDay} />
      {Platform.OS === 'web' && sheet && !empty && (
        <Button icon="print" onPress={() => window.print()} style={{ marginBottom: 16 }}>{t.print}</Button>
      )}

      {error && <Notice tone="warning">{error}</Notice>}
      {!rows && !error && <ActivityIndicator />}
      {empty && <EmptyState icon="flower-outline" title={t.none} />}

      {sheet && !empty && (
        <View style={{ gap: 20 }}>
          {sheet.toMake.length > 0 && (<>
          <View style={card}>
            <Text variant="footnote" color="labelSecondary" style={{ marginBottom: 6 }}>{t.totals.toUpperCase()}</Text>
            {sheet.totals.map((x, i) => (
              <View key={x.name} style={[{ flexDirection: 'row', paddingVertical: 8, gap: 12 }, i ? line : null]}>
                <Text variant="headline" style={{ minWidth: 36, fontVariant: ['tabular-nums'] }}>{x.quantity}</Text>
                <Text style={{ flex: 1 }}>{x.name}</Text>
              </View>
            ))}
          </View>

          <View style={card}>
            <Text variant="footnote" color="labelSecondary" style={{ marginBottom: 6 }}>{t.orders.toUpperCase()}</Text>
            {sheet.toMake.map((b, i) => (
              <View key={b.id} style={[{ flexDirection: 'row', paddingVertical: 10, gap: 12 }, i ? line : null]}>
                {/* A box to tick on paper. */}
                <View accessibilityElementsHidden importantForAccessibility="no"
                  style={{ width: 18, height: 18, borderWidth: 1.5, borderColor: ui.labelSecondary, borderRadius: 4, marginTop: 2 }} />
                <View style={{ flex: 1, gap: 2 }}>
                  <Text variant="headline" style={{ fontVariant: ['tabular-nums'] }}>
                    {`${hhmm(b.starts_at)}${b.ends_at ? `–${hhmm(b.ends_at)}` : ''} · ${b.items}`}
                  </Text>
                  {!!b.details && <Text>{b.details}</Text>}
                  {!!b.extras && <Text>{t.extras(b.extras)}</Text>}
                  <Text variant="footnote" color="labelSecondary">
                    {[b.customer_name, b.recipient_name ? t.for(b.recipient_name) : null,
                      b.fulfillment === 'delivery' ? strings.board.delivery : strings.board.pickup,
                      b.status === 'pending_deposit' ? t.pending : null].filter(Boolean).join(' · ')}
                  </Text>
                  {!!b.card_message && <Text style={{ fontStyle: 'italic' }}>{`${t.card}: «${b.card_message}»`}</Text>}
                </View>
              </View>
            ))}
          </View>

          </>)}

          {route && route.zones.map((z) => (
            <View key={z.zone ?? ''} style={card}>
              <Text variant="footnote" color="labelSecondary" style={{ marginBottom: 6 }}>{`${t.route} · ${z.zone ?? t.noZone}`.toUpperCase()}</Text>
              {z.stops.map((b, i) => <Stop key={b.id} b={b} first={!i} />)}
            </View>
          ))}
          {route && route.pickups.length > 0 && (
            <View style={card}>
              <Text variant="footnote" color="labelSecondary" style={{ marginBottom: 6 }}>{t.pickups.toUpperCase()}</Text>
              {route.pickups.map((b, i) => <Stop key={b.id} b={b} first={!i} />)}
            </View>
          )}

          {sheet.done.length > 0 && (
            <Text variant="footnote" color="labelSecondary">{`${t.done}: ${sheet.doneTotals.map((x) => `${x.quantity} × ${x.name}`).join(', ')}`}</Text>
          )}
        </View>
      )}
    </Screen>
  );
}

/** One stop of the route: when, where, who receives and what to collect. */
function Stop({ b, first }: { b: Summary; first: boolean }) {
  const ui = useUI();
  return (
    <View style={[{ paddingVertical: 10, gap: 2 }, first ? null : { borderTopWidth: StyleSheet.hairlineWidth, borderColor: ui.separator }]}>
      <View style={{ flexDirection: 'row', gap: 12 }}>
        <Text variant="headline" style={{ flex: 1, fontVariant: ['tabular-nums'] }}>{`${hhmm(b.starts_at)}${b.ends_at ? `–${hhmm(b.ends_at)}` : ''} · ${b.items}`}</Text>
        <Text variant="headline" style={{ fontVariant: ['tabular-nums'] }} color={b.due > 0 ? 'label' : 'labelSecondary'}>
          {b.due > 0 ? t.collect(formatMXN(b.due)) : t.paid}
        </Text>
      </View>
      {!!b.delivery_address && <Text>{b.delivery_address}</Text>}
      {!!b.extras && <Text style={{ fontWeight: '600' }}>{t.extras(b.extras)}</Text>}
      {!!b.recipient_name && <Text variant="footnote" color="labelSecondary">{t.receives(b.recipient_name, displayPhone(b.recipient_phone))}</Text>}
      <Text variant="footnote" color="labelSecondary">{t.customer(b.customer_name, displayPhone(b.customer_phone))}</Text>
    </View>
  );
}
