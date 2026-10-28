import type { components } from '@lb/api-client';
import { firstOfMonth, formatMXN, hhmm, localToISO, strings, todayISO } from '@lb/shared';
import { useState } from 'react';

import { Button, Field, Notice, Text, radius, useUI } from '../ui';
import { View } from 'react-native';
import { api } from '../lib/api';
import { errorText, fieldErrors } from '../lib/errors';
import { Calendar } from './Calendar';

type Booking = components['schemas']['Booking'];
const t = strings.bookings;

/** New return date for an active rental; the API keeps the same units and recalculates the total. */
export function ExtendRental({ businessId, booking, onDone }: { businessId: string; booking: Booking; onDone: (b: Booking) => void }) {
  const currentEnd = todayISO(new Date(booking.ends_at!));
  const ui = useUI();
  const [open, setOpen] = useState(false);
  const [month, setMonth] = useState(firstOfMonth(currentEnd));
  const [date, setDate] = useState<string | null>(null);
  const [time, setTime] = useState(hhmm(booking.ends_at!));
  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  if (!open) return <Button icon="calendar" onPress={() => setOpen(true)}>{t.extend}</Button>;

  async function save() {
    const endsAt = date ? localToISO(date, time) : null;
    if (!endsAt) return setMessage(t.pickReturn);
    setBusy(true);
    setMessage(null);
    const { data, error, response } = await api.POST('/v1/b/{businessId}/bookings/{bookingId}/extend', {
      params: { path: { businessId, bookingId: booking.id } }, body: { ends_at: endsAt },
    });
    setBusy(false);
    if (data) { setOpen(false); setDate(null); return onDone(data); }
    setMessage(fieldErrors(error).ends_at ?? errorText(error, response.status));
  }

  return (
    <View style={{ gap: 12, backgroundColor: ui.fill, borderRadius: radius.sm, padding: 14 }}>
      <Text variant="headline">{t.extend}</Text>
      <Text variant="footnote" color="labelSecondary">{t.extendHint(formatMXN(booking.total))}</Text>
      <Calendar month={month} onMonthChange={setMonth} minMonth={firstOfMonth(currentEnd)} from={date} onSelect={setDate} label={t.extend}
        dayInfo={(d) => ({ disabled: d < currentEnd })} />
      <Field label={t.backTime} value={time} onChangeText={setTime} />
      {message && <Notice tone="warning">{message}</Notice>}
      <Button variant="primary" busy={busy} disabled={!date} onPress={save}>{t.doExtend}</Button>
      <Button variant="plain" onPress={() => setOpen(false)}>{strings.common.cancel}</Button>
    </View>
  );
}
