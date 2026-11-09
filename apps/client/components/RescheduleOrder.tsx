import type { components } from '@lb/api-client';
import { firstOfMonth, hhmm, localToISO, strings, todayISO } from '@lb/shared';
import { useState } from 'react';

import { Button, Field, Text, radius, useToast, useUI } from '../ui';
import { View } from 'react-native';
import { api } from '../lib/api';
import { errorText, fieldErrors } from '../lib/errors';
import { Calendar } from './Calendar';

type Booking = components['schemas']['Booking'];
const t = strings.bookings;

/** New date (and optional same-day window) for an order; the API re-checks the day's capacity. */
export function RescheduleOrder({ businessId, booking, onDone }: { businessId: string; booking: Booking; onDone: (b: Booking) => void }) {
  const today = todayISO();
  const ui = useUI();
  const toast = useToast();
  const [open, setOpen] = useState(false);
  const [month, setMonth] = useState(firstOfMonth(todayISO(new Date(booking.starts_at))));
  const [date, setDate] = useState<string | null>(null);
  const [time, setTime] = useState(hhmm(booking.starts_at));
  const [until, setUntil] = useState(booking.ends_at ? hhmm(booking.ends_at) : '');
  const [busy, setBusy] = useState(false);

  if (!open) return <Button icon="calendar" onPress={() => setOpen(true)}>{t.reschedule}</Button>;

  async function save() {
    if (!date) return toast(t.pickDate, 'error');
    const startsAt = localToISO(date, time);
    const endsAt = until.trim() ? localToISO(date, until) : null;
    if (!startsAt || (until.trim() && !endsAt)) return toast(t.badTime, 'error');
    setBusy(true);
    const { data, error, response } = await api.PUT('/v1/b/{businessId}/bookings/{bookingId}/schedule', {
      params: { path: { businessId, bookingId: booking.id } },
      body: { starts_at: startsAt, ends_at: endsAt },
    });
    setBusy(false);
    if (data) { setOpen(false); setDate(null); toast(t.rescheduled); return onDone(data); }
    const f = fieldErrors(error);
    toast(f.starts_at ?? f.ends_at ?? errorText(error, response.status), 'error');
  }

  return (
    <View style={{ gap: 12, backgroundColor: ui.fill, borderRadius: radius.sm, padding: 14 }}>
      <Text variant="headline">{t.reschedule}</Text>
      <Text variant="footnote" color="labelSecondary">{t.rescheduleHint}</Text>
      <Calendar month={month} onMonthChange={setMonth} minMonth={firstOfMonth(today)} from={date} onSelect={setDate} label={t.reschedule}
        dayInfo={(d) => ({ disabled: d < today })} />
      <View style={{ flexDirection: 'row', gap: 12 }}>
        <View style={{ flex: 1 }}><Field label={t.deliveryTime} value={time} onChangeText={setTime} /></View>
        <View style={{ flex: 1 }}><Field label={t.windowEnd} value={until} onChangeText={setUntil} /></View>
      </View>
      <Button variant="primary" busy={busy} disabled={!date} onPress={save}>{t.doExtend}</Button>
      <Button variant="plain" onPress={() => setOpen(false)}>{strings.common.cancel}</Button>
    </View>
  );
}
