import type { components } from '@lb/api-client';
import { centavosToInput, formatMXN, parsePesos, strings, term } from '@lb/shared';
import { Stack, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Button, Linking, Pressable, ScrollView, Text, TextInput, View } from 'react-native';
import { api } from '../../../../../lib/api';
import { errorText, fieldErrors } from '../../../../../lib/errors';
import { StepCard } from '../../../../../components/StepCard';
import { useBusiness } from '../../../../../lib/useMe';

type Booking = components['schemas']['Booking'];
type PaymentType = components['schemas']['PaymentType'];
type Method = 'cash' | 'transfer' | 'card';
const t = strings.bookings;

const when = (iso: string) =>
  new Date(iso).toLocaleString('es-MX', { weekday: 'short', day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });

function Choice({ selected, label, onPress }: { selected: boolean; label: string; onPress: () => void }) {
  return (
    <Pressable accessibilityRole="radio" accessibilityState={{ checked: selected }} onPress={onPress}
      style={{ borderWidth: selected ? 2 : 1, borderRadius: 6, paddingVertical: 6, paddingHorizontal: 10 }}>
      <Text>{label}</Text>
    </Pressable>
  );
}

const Section = ({ title }: { title: string }) => <Text style={{ fontWeight: '600', marginTop: 12 }}>{title}</Text>;

export default function BookingDetail() {
  const { businessId, bookingId } = useLocalSearchParams<{ businessId: string; bookingId: string }>();
  const { business } = useBusiness(businessId);
  const [b, setB] = useState<Booking | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState('');
  const [confirmCancel, setConfirmCancel] = useState(false);
  const [payType, setPayType] = useState<PaymentType>('advance');
  const [method, setMethod] = useState<Method>('cash');
  const [amount, setAmount] = useState('');
  const [payErrors, setPayErrors] = useState<Record<string, string>>({});
  const [condition, setCondition] = useState('');
  const path = { businessId, bookingId };

  const show = useCallback((data: Booking) => {
    setB(data);
    // Suggest the most likely payment: missing advance, security deposit before check-out,
    // late fee after a late return, else the balance.
    const security = Number(data.items[0]?.options?.security_deposit ?? 0);
    const owesAdvance = data.status === 'pending_deposit' && data.balance.paid < data.deposit_required;
    const owesDeposit = data.kind === 'rental' && data.status === 'confirmed' && data.balance.deposit_held < security;
    const lateFee = data.status === 'returned' ? data.rental?.late_fee_suggested ?? 0 : 0;
    const [type, cents]: [PaymentType, number] = owesAdvance ? ['advance', data.deposit_required - data.balance.paid]
      : owesDeposit ? ['security_deposit', security - data.balance.deposit_held]
        : lateFee > 0 && data.balance.extra_charges < lateFee ? ['late_fee', lateFee - data.balance.extra_charges]
          : data.balance.due > 0 ? ['balance', data.balance.due]
            : data.balance.deposit_held > 0 && data.status === 'returned' ? ['refund', data.balance.deposit_held] : ['balance', 0];
    setPayType(type);
    setAmount(cents > 0 ? centavosToInput(cents) : '');
  }, []);

  useEffect(() => {
    api.GET('/v1/b/{businessId}/bookings/{bookingId}', { params: { path } })
      .then(({ data, error, response }) => (data ? show(data) : setMessage(errorText(error, response.status))));
  }, [businessId, bookingId]); // eslint-disable-line react-hooks/exhaustive-deps

  async function move(to: Booking['status']) {
    setBusy(true);
    setMessage(null);
    const { data, error, response } = await api.POST('/v1/b/{businessId}/bookings/{bookingId}/transition', { params: { path }, body: { to, note } });
    setBusy(false);
    setConfirmCancel(false);
    if (data) { show(data); setNote(''); return; }
    const pending = (error?.details as { steps?: string[] } | undefined)?.steps;
    setMessage(pending ? error!.message : errorText(error, response.status));
  }

  async function rentalMove(kind: 'checkout' | 'return') {
    setBusy(true);
    setMessage(null);
    const res = kind === 'checkout'
      ? await api.POST('/v1/b/{businessId}/bookings/{bookingId}/checkout', { params: { path }, body: { condition } })
      : await api.POST('/v1/b/{businessId}/bookings/{bookingId}/return', { params: { path }, body: { condition } });
    setBusy(false);
    if (res.data) { setCondition(''); return show(res.data); }
    setMessage(errorText(res.error, res.response.status));
  }

  async function pay() {
    const cents = parsePesos(amount);
    if (cents === null || cents <= 0) return setPayErrors({ amount: strings.catalog.invalidMoney });
    setBusy(true);
    setPayErrors({});
    setMessage(null);
    const { data, error, response } = await api.POST('/v1/b/{businessId}/bookings/{bookingId}/payments', {
      params: { path }, body: { type: payType, method, amount: cents },
    });
    setBusy(false);
    if (data) return show(data);
    setPayErrors(fieldErrors(error));
    setMessage(errorText(error, response.status));
  }

  if (!b || !business) return message ? <Text style={{ padding: 16 }}>{message}</Text> : <ActivityIndicator style={{ marginTop: 24 }} />;
  // Tolerate an older API without the operation fields instead of crashing the screen.
  b.next ??= [];
  b.payments ??= [];
  b.events ??= [];
  b.steps ??= [];
  b.balance ??= { paid: 0, due: b.total, deposit_held: 0, extra_charges: 0 };
  const security = Number(b.items[0]?.options?.security_deposit ?? 0);
  const bookingTerm = term(business.vertical, 'booking') ?? t.detail;
  const payTypes: PaymentType[] = b.kind === 'rental'
    ? ['advance', 'balance', 'security_deposit', 'refund', 'late_fee', 'damage_fee'] : ['advance', 'balance'];
  // Rental check-out/return have their own flow (next part); the rest are plain status moves.
  const actions = b.next.filter((s) => !(b.kind === 'rental' && (s === 'checked_out' || s === 'returned')));
  const waText = t.whatsappText(b.customer.name, bookingTerm, when(b.starts_at), t.status[b.status]);

  return (
    <ScrollView contentContainerStyle={{ padding: 16, gap: 6, maxWidth: 600, width: '100%', alignSelf: 'center' }}>
      <Stack.Screen options={{ title: bookingTerm }} />
      <Text accessibilityRole="header" style={{ fontSize: 20, fontWeight: '600' }}>{t.status[b.status]}</Text>
      <Text>{b.items.map((i) => `${i.quantity} × ${i.name}`).join(', ')}</Text>
      <Text>{term(business.vertical, 'starts_at')}: {when(b.starts_at)}</Text>
      {b.kind === 'rental' && b.ends_at && <Text>{term(business.vertical, 'ends_at')}: {when(b.ends_at)}</Text>}
      {b.staff.length > 0 && <Text>{t.attendedBy}: {b.staff.map((s) => s.name).join(', ')}</Text>}
      {b.units.length > 0 && <Text>{t.units}: {b.units.map((u) => u.name).join(', ')}</Text>}
      {b.fulfillment === 'delivery' && <Text>{t.delivery}: {b.delivery_address}</Text>}
      <Text>{t.customer}: {b.customer.name} · {b.customer.phone}</Text>
      {b.notes && <Text>{b.notes}</Text>}
      {b.hold_expires_at && <Text>{t.holdUntil}: {when(b.hold_expires_at)}</Text>}
      <Pressable accessibilityRole="link" onPress={() => Linking.openURL(`https://wa.me/${b.customer.phone.replace(/\D/g, '')}?text=${encodeURIComponent(waText)}`)}>
        <Text style={{ textDecorationLine: 'underline' }}>{t.whatsapp}</Text>
      </Pressable>

      <Section title={t.total} />
      <Text>{t.total}: {formatMXN(b.total)} · {t.paid}: {formatMXN(b.balance.paid)} · {t.due}: {formatMXN(b.balance.due)}</Text>
      {b.deposit_required > 0 && <Text>{t.advance}: {formatMXN(b.deposit_required)}</Text>}
      {(security > 0 || b.balance.deposit_held > 0) && (
        <Text>{t.securityDeposit}: {formatMXN(security)} · {t.depositHeld}: {formatMXN(b.balance.deposit_held)}</Text>
      )}
      {b.balance.extra_charges > 0 && <Text>{t.extraCharges}: {formatMXN(b.balance.extra_charges)}</Text>}

      {message && <Text accessibilityRole="alert" style={{ marginTop: 8 }}>{message}</Text>}

      {actions.length > 0 && (
        <>
          <Section title={t.actionsTitle} />
          <TextInput accessibilityLabel={t.noteOptional} placeholder={t.noteOptional} value={note} onChangeText={setNote}
            style={{ borderWidth: 1, borderRadius: 6, padding: 8 }} />
          <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
            {actions.map((s) => (
              <Button key={s} title={t.actions[s] ?? t.status[s]} disabled={busy}
                onPress={() => (s === 'cancelled' ? setConfirmCancel(true) : move(s))} />
            ))}
          </View>
          {confirmCancel && (
            <View style={{ gap: 6 }}>
              <Text>{t.confirmCancel}</Text>
              <Button title={t.yesCancel} onPress={() => move('cancelled')} disabled={busy} />
            </View>
          )}
        </>
      )}
      {b.kind === 'rental' && (b.next.includes('checked_out') || b.next.includes('returned')) && (
        <>
          <Section title={b.next.includes('checked_out') ? t.checkout : t.return} />
          {b.next.includes('checked_out') && <Text style={{ opacity: 0.7 }}>{t.checkoutNeeds}</Text>}
          <TextInput accessibilityLabel={b.next.includes('checked_out') ? t.checkoutCondition : t.returnCondition}
            placeholder={b.next.includes('checked_out') ? t.checkoutCondition : t.returnCondition}
            value={condition} onChangeText={setCondition} style={{ borderWidth: 1, borderRadius: 6, padding: 8 }} />
          <Button title={b.next.includes('checked_out') ? t.doCheckout : t.doReturn} disabled={busy}
            onPress={() => rentalMove(b.next.includes('checked_out') ? 'checkout' : 'return')} />
        </>
      )}
      {b.rental && (b.rental.checked_out_at || b.rental.returned_at) && (
        <>
          {b.rental.checked_out_at && <Text>{t.outAt}: {when(b.rental.checked_out_at)}{b.rental.out_condition ? ` — ${b.rental.out_condition}` : ''}</Text>}
          {b.rental.returned_at && <Text>{t.backAt}: {when(b.rental.returned_at)}{b.rental.return_condition ? ` — ${b.rental.return_condition}` : ''}</Text>}
          {b.rental.late_days > 0 && (
            <Text>{t.lateDays(b.rental.late_days)} · {t.lateFeeHint}: {formatMXN(b.rental.late_fee_suggested)}</Text>
          )}
        </>
      )}

      <Section title={t.payments} />
      {b.payments.map((p) => (
        <Text key={p.id}>{when(p.paid_at)} · {t.paymentTypes[p.type]} · {t.methods[p.method]} · {formatMXN(p.amount)}</Text>
      ))}
      <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 6 }}>
        {payTypes.map((pt) => <Choice key={pt} selected={payType === pt} label={t.paymentTypes[pt]} onPress={() => setPayType(pt)} />)}
      </View>
      <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 6 }}>
        {(['cash', 'transfer', 'card'] as Method[]).map((m) => <Choice key={m} selected={method === m} label={t.methods[m]} onPress={() => setMethod(m)} />)}
      </View>
      <TextInput accessibilityLabel={t.amount} placeholder={t.amount} value={amount} onChangeText={setAmount} keyboardType="decimal-pad"
        style={{ borderWidth: 1, borderRadius: 6, padding: 8 }} />
      {payErrors.amount && <Text accessibilityRole="alert">{payErrors.amount}</Text>}
      <Button title={t.addPayment} onPress={pay} disabled={busy} />

      {b.steps.length > 0 && (
        <>
          <Section title={t.steps} />
          {b.steps.map((s) => (
            <StepCard key={s.id} businessId={businessId} bookingId={bookingId} step={s} onDone={show}
              editable={!['cancelled', 'expired', 'no_show', 'completed'].includes(b.status)} />
          ))}
        </>
      )}

      <Section title={t.history} />
      {b.events.map((e, i) => (
        <Text key={i}>{when(e.at)} · {e.from_status ? `${t.status[e.from_status as Booking['status']] ?? e.from_status} → ` : `${t.created}: `}{t.status[e.to_status as Booking['status']] ?? e.to_status}{e.note ? ` — ${e.note}` : ''}</Text>
      ))}
    </ScrollView>
  );
}
