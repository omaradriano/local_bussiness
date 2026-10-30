import type { components } from '@lb/api-client';
import { centavosToInput, discountInput, strings, type DiscountMode } from '@lb/shared';
import { useState } from 'react';
import { View } from 'react-native';
import { api } from '../lib/api';
import { errorText } from '../lib/errors';
import { Button, Field, Segmented, useToast } from '../ui';

type Booking = components['schemas']['Booking'];
const t = strings.bookings.discount;
const modes: DiscountMode[] = ['none', 'amount', 'percent'];

/** Discount type and value; the parent keeps the state (new booking) or saves it (DiscountForm). */
export function DiscountFields({ mode, text, onMode, onText, error }: {
  mode: DiscountMode; text: string; onMode: (m: DiscountMode) => void; onText: (s: string) => void; error?: string;
}) {
  return (
    <View style={{ gap: 8 }}>
      <Segmented options={modes.map((m) => ({ value: m, label: t.modes[m] }))} value={mode} onChange={onMode} />
      {mode !== 'none' && (
        <Field label={mode === 'amount' ? t.amount : t.percent} value={text} onChangeText={onText} keyboardType="decimal-pad" error={error} />
      )}
    </View>
  );
}

/** Put, change or remove the discount of a booking; the API recalculates the total and the advance. */
export function DiscountForm({ businessId, booking, onDone }: { businessId: string; booking: Booking; onDone: (b: Booking) => void }) {
  const toast = useToast();
  const [open, setOpen] = useState(false);
  const [mode, setMode] = useState<DiscountMode>(booking.discount_type ?? 'none');
  const [text, setText] = useState(booking.discount_type === 'amount' ? centavosToInput(booking.discount_value) : booking.discount_type === 'percent' ? String(booking.discount_value) : '');
  const [error, setError] = useState<string | undefined>();
  const [busy, setBusy] = useState(false);

  if (!open) return <Button icon="pricetag" onPress={() => setOpen(true)}>{booking.discount > 0 ? t.change : t.open}</Button>;

  async function save() {
    const d = discountInput(mode, text);
    if (d === undefined) return setError(t.invalid);
    setBusy(true);
    setError(undefined);
    const { data, error: err, response } = await api.PUT('/v1/b/{businessId}/bookings/{bookingId}/discount', {
      params: { path: { businessId, bookingId: booking.id } },
      body: d ?? { type: 'amount', value: 0 },
    });
    setBusy(false);
    if (data) { toast(d ? t.saved : t.removed); setOpen(false); return onDone(data); }
    toast(errorText(err, response.status), 'error');
  }

  return (
    <View style={{ gap: 10 }}>
      <DiscountFields mode={mode} text={text} onMode={setMode} onText={setText} error={error} />
      <Button variant="primary" busy={busy} onPress={save}>{t.save}</Button>
    </View>
  );
}
