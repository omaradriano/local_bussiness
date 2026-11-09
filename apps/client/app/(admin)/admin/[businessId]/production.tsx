import type { components } from '@lb/api-client';
import { addDays, dayStartISO, hhmm, productionSheet, strings, todayISO } from '@lb/shared';
import { Stack, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator, Platform, StyleSheet, View } from 'react-native';
import { api } from '../../../../lib/api';
import { errorText } from '../../../../lib/errors';
import { useBusiness } from '../../../../lib/useMe';
import { Button, EmptyState, Notice, Screen, Text, radius, useUI } from '../../../../ui';

type Summary = components['schemas']['BookingSummary'];
const t = strings.production;

/** Production sheet: what to make on a day (added up) and each order with its card, ready to print. */
export default function Production() {
  const { businessId, day: dayParam } = useLocalSearchParams<{ businessId: string; day?: string }>();
  const { business } = useBusiness(businessId);
  const ui = useUI();
  const day = dayParam ?? todayISO();
  const [rows, setRows] = useState<Summary[] | null>(null);
  const [error, setError] = useState<string | null>(null);

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
  const date = new Date(`${day}T12:00:00`).toLocaleDateString('es-MX', { weekday: 'long', day: 'numeric', month: 'long' });
  const line = { borderTopWidth: StyleSheet.hairlineWidth, borderColor: ui.separator };

  return (
    <Screen>
      <Stack.Screen options={{ title: t.title }} />
      <Text variant="footnote" color="labelSecondary">{business.name}</Text>
      <Text variant="largeTitle" accessibilityRole="header">{t.title}</Text>
      <Text variant="headline" style={{ textTransform: 'capitalize', marginBottom: 12 }}>{date}</Text>
      {Platform.OS === 'web' && sheet && sheet.toMake.length > 0 && (
        <Button icon="print" onPress={() => window.print()} style={{ marginBottom: 16 }}>{t.print}</Button>
      )}

      {error && <Notice tone="warning">{error}</Notice>}
      {!rows && !error && <ActivityIndicator />}
      {sheet && sheet.toMake.length === 0 && <EmptyState icon="flower-outline" title={t.none} />}

      {sheet && sheet.toMake.length > 0 && (
        <View style={{ gap: 20 }}>
          <View style={{ backgroundColor: ui.surface, borderRadius: radius.md, padding: 16 }}>
            <Text variant="footnote" color="labelSecondary" style={{ marginBottom: 6 }}>{t.totals.toUpperCase()}</Text>
            {sheet.totals.map((x, i) => (
              <View key={x.name} style={[{ flexDirection: 'row', paddingVertical: 8, gap: 12 }, i ? line : null]}>
                <Text variant="headline" style={{ minWidth: 36, fontVariant: ['tabular-nums'] }}>{x.quantity}</Text>
                <Text style={{ flex: 1 }}>{x.name}</Text>
              </View>
            ))}
          </View>

          <View style={{ backgroundColor: ui.surface, borderRadius: radius.md, padding: 16 }}>
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

          {sheet.done.length > 0 && (
            <Text variant="footnote" color="labelSecondary">{`${t.done}: ${sheet.done.map((b) => b.items).join(', ')}`}</Text>
          )}
        </View>
      )}
    </Screen>
  );
}
