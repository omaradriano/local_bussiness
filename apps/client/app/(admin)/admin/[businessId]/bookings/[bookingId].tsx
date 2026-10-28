import type { components } from '@lb/api-client';
import { centavosToInput, formatMXN, parsePesos, strings, term, formatPhone } from '@lb/shared';
import { router, Stack, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Linking, View } from 'react-native';
import { api } from '../../../../../lib/api';
import { errorText, fieldErrors } from '../../../../../lib/errors';
import { ExtendRental } from '../../../../../components/ExtendRental';
import { StepCard } from '../../../../../components/StepCard';
import { useBusiness } from '../../../../../lib/useMe';
import { Button, Cell, Field, Group, Notice, Row, Screen, Segmented, Step, Text, useToast, useUI, type StepState } from '../../../../../ui';

type Booking = components['schemas']['Booking'];
type PaymentType = components['schemas']['PaymentType'];
type Method = 'cash' | 'transfer' | 'card';
const t = strings.bookings;

const when = (iso: string) =>
  new Date(iso).toLocaleString('es-MX', { weekday: 'short', day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });

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
  const ui = useUI();
  const toast = useToast();

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
    const { data, error, response } = await api.POST('/v1/b/{businessId}/bookings/{bookingId}/transition', { params: { path }, body: { to, note } });
    setBusy(false);
    setConfirmCancel(false);
    if (data) { show(data); setNote(''); toast(t.statusChanged(t.status[data.status])); return; }
    const pending = (error?.details as { steps?: string[] } | undefined)?.steps;
    toast(pending ? error!.message : errorText(error, response.status), 'error');
  }

  async function rentalMove(kind: 'checkout' | 'return') {
    setBusy(true);
    const res = kind === 'checkout'
      ? await api.POST('/v1/b/{businessId}/bookings/{bookingId}/checkout', { params: { path }, body: { condition } })
      : await api.POST('/v1/b/{businessId}/bookings/{bookingId}/return', { params: { path }, body: { condition } });
    setBusy(false);
    if (res.data) { setCondition(''); toast(kind === 'checkout' ? t.checkedOutToast : t.returnedToast); return show(res.data); }
    toast(errorText(res.error, res.response.status), 'error');
  }

  async function pay() {
    const cents = parsePesos(amount);
    if (cents === null || cents <= 0) return setPayErrors({ amount: strings.catalog.invalidMoney });
    setBusy(true);
    setPayErrors({});
    const { data, error, response } = await api.POST('/v1/b/{businessId}/bookings/{bookingId}/payments', {
      params: { path }, body: { type: payType, method, amount: cents },
    });
    setBusy(false);
    if (data) { toast(t.paymentSaved); return show(data); }
    setPayErrors(fieldErrors(error));
    toast(errorText(error, response.status), 'error');
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
  const payTypes: PaymentType[] = b.kind !== 'rental' ? ['advance', 'balance']
    : (['advance', 'balance', 'security_deposit', 'refund', 'late_fee', 'damage_fee'] as PaymentType[])
      .filter((p) => p !== 'security_deposit' || security > 0)
      .filter((p) => p !== 'refund' || b.balance.deposit_held > 0);
  // Rental check-out/return have their own flow (next part); the rest are plain status moves.
  const actions = b.next.filter((s) => !(b.kind === 'rental' && (s === 'checked_out' || s === 'returned')));
  const waText = t.whatsappText(b.customer.name, bookingTerm, when(b.starts_at), t.status[b.status]);

  const editable = !['cancelled', 'expired', 'no_show', 'completed'].includes(b.status);
  const stepCards = (to: string | null) => b.steps.filter((st) => (to === 'checked_out' ? st.to_status !== 'returned' : st.to_status === to)).map((st) => (
    <StepCard key={st.id} businessId={businessId} bookingId={bookingId} step={st} onDone={show} editable={editable} />
  ));
  const actionButtons = (only?: Booking['status'][]) => {
    const list = actions.filter((st) => !only || only.includes(st));
    if (!list.length) return null;
    return (
      <View style={{ gap: 8 }}>
        {list.map((st) => (
          <Button key={st} variant={st === 'cancelled' || st === 'no_show' ? 'destructive' : 'secondary'} disabled={busy}
            onPress={() => (st === 'cancelled' ? setConfirmCancel(true) : move(st))}>{t.actions[st] ?? t.status[st]}</Button>
        ))}
        {confirmCancel && (
          <View style={{ gap: 8 }}>
            <Notice tone="warning">{t.confirmCancel}</Notice>
            <Button variant="destructive" onPress={() => move('cancelled')} disabled={busy}>{t.yesCancel}</Button>
          </View>
        )}
      </View>
    );
  };
  const paymentForm = (
    <View style={{ gap: 10 }}>
      <Text variant="subhead" color="labelSecondary">{t.addPayment}</Text>
      <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
        {payTypes.map((pt) => (
          <Button key={pt} size="sm" variant={payType === pt ? 'primary' : 'secondary'} onPress={() => setPayType(pt)}>{t.paymentTypes[pt]}</Button>
        ))}
      </View>
      <Segmented options={(['cash', 'transfer', 'card'] as Method[]).map((m) => ({ value: m, label: t.methods[m] }))} value={method} onChange={setMethod} />
      <Field label={t.amount} value={amount} onChangeText={setAmount} keyboardType="decimal-pad" error={payErrors.amount} />
      <Button variant="primary" busy={busy} onPress={pay}>{t.addPayment}</Button>
    </View>
  );
  const balance = (
    <View style={{ gap: 6 }}>
      <Row label={t.total} value={formatMXN(b.total)} />
      <Row label={t.paid} value={formatMXN(b.balance.paid)} />
      {b.deposit_required > 0 && <Row label={t.advance} value={formatMXN(b.deposit_required)} />}
      {(security > 0 || b.balance.deposit_held > 0) && <Row label={t.depositHeld} value={`${formatMXN(b.balance.deposit_held)} / ${formatMXN(security)}`} />}
      {b.balance.extra_charges > 0 && <Row label={t.extraCharges} value={formatMXN(b.balance.extra_charges)} />}
      <Row label={t.due} value={formatMXN(b.balance.due)} strong />
    </View>
  );

  // Rentals go through numbered steps; the current one is open and the rest stay visible so the path is predictable.
  const rentalStep = { pending_deposit: 0, confirmed: 1, checked_out: 2, overdue: 2, returned: 3, completed: 4 }[b.status as string] ?? -1;
  const stepState = (i: number): StepState => (rentalStep > i ? 'done' : rentalStep === i ? 'current' : 'upcoming');
  const f = t.detailFlow;
  const closedNote = ['cancelled', 'expired', 'no_show'].includes(b.status);

  return (
    <Screen>
      <Stack.Screen options={{ title: bookingTerm }} />
      <View style={{ gap: 16 }}>
        <View style={{ gap: 4 }}>
          <Text variant="footnote" color="labelSecondary">{b.items.map((i) => `${i.quantity} × ${i.name}`).join(', ')}{b.units.length ? ` · ${b.units.map((u) => u.name).join(', ')}` : ''}</Text>
          <Text variant="largeTitle" accessibilityRole="header">{t.status[b.status]}</Text>
          {b.status === 'overdue' && <Notice tone="warning">{f.overdue}</Notice>}
          {closedNote && <Notice tone="warning">{f.closed}</Notice>}
        </View>

        <Group separatorInset={58}>
          <Cell icon="person" tint={ui.indigo} label={b.customer.name} sublabel={formatPhone(b.customer.phone)}
            onPress={() => router.push({ pathname: '/admin/[businessId]/customers/[customerId]', params: { businessId, customerId: b.customer.id } })} />
          <Cell icon="logo-whatsapp" tint={ui.success} label={t.whatsapp}
            onPress={() => Linking.openURL(`https://wa.me/${b.customer.phone.replace(/\D/g, '')}?text=${encodeURIComponent(waText)}`)} />
          <Cell icon="arrow-up-circle" tint={ui.accent} label={term(business.vertical, 'starts_at') ?? ''} value={when(b.starts_at)} />
          {b.kind === 'rental' && b.ends_at ? <Cell icon="arrow-down-circle" tint={ui.warning} label={term(business.vertical, 'ends_at') ?? ''} value={when(b.ends_at)} /> : null}
          {b.staff.length > 0 ? <Cell icon="person-circle" tint={ui.gray} label={t.attendedBy} value={b.staff.map((x) => x.name).join(', ')} /> : null}
          {b.fulfillment === 'delivery' ? <Cell icon="car" tint={ui.gray} label={t.delivery} sublabel={b.delivery_address ?? ''} /> : null}
          {b.hold_expires_at ? <Cell icon="hourglass" tint={ui.warning} label={t.holdUntil} value={when(b.hold_expires_at)} /> : null}
          {b.notes ? <Cell icon="document-text" tint={ui.gray} label={b.notes} /> : null}
        </Group>

        {b.kind === 'rental' && !closedNote ? (
          <>
            <Step n={1} title={f.confirm.title} description={f.confirm.description} state={stepState(0)}>
              {rentalStep === 0 && (<>{balance}{paymentForm}{actionButtons(['confirmed', 'cancelled'])}</>)}
            </Step>

            <Step n={2} title={f.checkout.title} description={f.checkout.description} state={stepState(1)}>
              {rentalStep === 1 && (
                <>
                  {stepCards('checked_out')}
                  {balance}
                  {paymentForm}
                  <Field label={t.checkoutCondition} value={condition} onChangeText={setCondition} />
                  <Button variant="primary" icon="arrow-up-circle" busy={busy} onPress={() => rentalMove('checkout')}>{t.doCheckout}</Button>
                  {actionButtons(['cancelled', 'no_show'])}
                </>
              )}
              {rentalStep > 1 && b.rental?.checked_out_at && (
                <Text variant="subhead" color="labelSecondary">{t.outAt}: {when(b.rental.checked_out_at)}{b.rental.out_condition ? ` — ${b.rental.out_condition}` : ''}</Text>
              )}
            </Step>

            <Step n={3} title={f.return.title} description={f.return.description} state={stepState(2)}>
              {rentalStep === 2 && (
                <>
                  {stepCards('returned')}
                  <Field label={t.returnCondition} value={condition} onChangeText={setCondition} />
                  <Button variant="primary" icon="arrow-down-circle" busy={busy} onPress={() => rentalMove('return')}>{t.doReturn}</Button>
                </>
              )}
              {rentalStep <= 2 && b.ends_at && (
                <ExtendRental businessId={businessId} booking={b} onDone={(nb) => { show(nb); toast(t.extended(when(nb.ends_at!), formatMXN(nb.total))); }} />
              )}
              {rentalStep > 2 && b.rental?.returned_at && (
                <Text variant="subhead" color="labelSecondary">{t.backAt}: {when(b.rental.returned_at)}{b.rental.return_condition ? ` — ${b.rental.return_condition}` : ''}</Text>
              )}
            </Step>

            <Step n={4} title={f.close.title} description={f.close.description} state={stepState(3)}>
              {rentalStep === 3 && (
                <>
                  {b.rental && b.rental.late_days > 0 && (
                    <Notice tone="warning">{`${t.lateDays(b.rental.late_days)} · ${t.lateFeeHint}: ${formatMXN(b.rental.late_fee_suggested)}`}</Notice>
                  )}
                  {balance}
                  {paymentForm}
                  {actionButtons(['completed'])}
                </>
              )}
            </Step>
          </>
        ) : (
          <>
            <Group label={t.total} padded>{balance}</Group>
            {editable && <Group label={t.actionsTitle} padded>{actionButtons() ?? <Text color="labelSecondary">—</Text>}</Group>}
            {editable && <Group label={t.payments} padded>{paymentForm}</Group>}
            {b.steps.length > 0 && <Group label={t.steps} padded><View style={{ gap: 10 }}>{stepCards('checked_out')}{stepCards('returned')}</View></Group>}
          </>
        )}

        {b.payments.length > 0 && (
          <Group label={t.payments}>
            {b.payments.map((p) => (
              <Cell key={p.id} label={`${t.paymentTypes[p.type]} · ${t.methods[p.method]}`} sublabel={when(p.paid_at)} value={formatMXN(p.amount)} />
            ))}
          </Group>
        )}

        <Group label={t.history}>
          {b.events.map((e, i) => (
            <Cell key={i} label={`${e.from_status ? `${t.status[e.from_status as Booking['status']] ?? e.from_status} → ` : `${t.created}: `}${t.status[e.to_status as Booking['status']] ?? e.to_status}`}
              sublabel={`${when(e.at)}${e.note ? ` — ${e.note}` : ''}`} />
          ))}
        </Group>
      </View>
    </Screen>
  );
}
