import type { components } from '@lb/api-client';
import { formatMXN, parsePesos, strings } from '@lb/shared';
import { useState } from 'react';
import { ConfirmDialog } from './ConfirmDialog';
import { api } from '../lib/api';
import { errorText, fieldErrors } from '../lib/errors';
import { Button, Field, Segmented, useToast } from '../ui';

type Booking = components['schemas']['Booking'];
type Method = 'cash' | 'transfer' | 'card';
const t = strings.bookings.partialRefund;

/** Give back part of what was paid (damaged flowers) without cancelling: owner only; the total goes down the same. */
export function PartialRefund({ businessId, booking, onDone }: { businessId: string; booking: Booking; onDone: (b: Booking) => void }) {
  const toast = useToast();
  const [open, setOpen] = useState(false);
  const [amount, setAmount] = useState('');
  // Money goes back the way it came in (the API checks each method): start on the method that took the most.
  const paidIn = (m: Method) => booking.payments.reduce((n, p) => n + (p.method !== m ? 0 : p.type === 'payment_refund' ? -p.amount : p.type === 'advance' || p.type === 'balance' ? p.amount : 0), 0);
  const [method, setMethod] = useState<Method>(() => (['cash', 'transfer', 'card'] as const).reduce((a, m) => (paidIn(m) > paidIn(a) ? m : a)));
  const max = Math.min(booking.total, booking.balance.paid); // the arrangement only: extras go back by cancelling their sale
  const [reason, setReason] = useState('');
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const cents = parsePesos(amount);

  function close() { setOpen(false); setAmount(''); setReason(''); setErrors({}); }

  async function save() {
    if (!cents || cents <= 0) return setErrors({ amount: t.amountMissing });
    if (!reason.trim()) return setErrors({ reason: t.reasonMissing });
    setBusy(true);
    const { data, error, response } = await api.POST('/v1/b/{businessId}/bookings/{bookingId}/refund', {
      params: { path: { businessId, bookingId: booking.id } }, body: { amount: cents, method, reason: reason.trim() },
    });
    setBusy(false);
    if (!data) { setErrors(fieldErrors(error)); return toast(errorText(error, response.status), 'error'); }
    toast(t.done);
    close();
    onDone(data);
  }

  return (
    <>
      <Button icon="arrow-undo" onPress={() => setOpen(true)}>{t.action}</Button>
      <ConfirmDialog open={open} primary busy={busy} title={t.title} body={t.body(formatMXN(max))}
        confirmLabel={cents ? t.confirm(formatMXN(cents)) : t.action} onConfirm={save} onClose={close}>
        <Field label={t.amount} value={amount} onChangeText={(v) => { setAmount(v); setErrors({}); }} keyboardType="decimal-pad" error={errors.amount} />
        <Segmented options={(['cash', 'transfer', 'card'] as const).map((m) => ({ value: m, label: strings.bookings.methods[m] }))} value={method} onChange={setMethod} />
        <Field label={t.reason} placeholder={t.reasonPlaceholder} value={reason} onChangeText={(v) => { setReason(v); setErrors({}); }} maxLength={200} error={errors.reason} />
      </ConfirmDialog>
    </>
  );
}
