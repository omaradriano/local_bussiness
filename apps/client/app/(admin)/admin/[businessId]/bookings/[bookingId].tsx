import type { components } from '@lb/api-client';
import { engineFor, formatMXN, formatPhone, hhmm, strings, term, WALK_IN_PHONE } from '@lb/shared';
import { router, Stack, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator, Linking, View } from 'react-native';
import { BookingDocuments } from '../../../../../components/BookingDocuments';
import { ConfirmDialog } from '../../../../../components/ConfirmDialog';
import { ExtendRental } from '../../../../../components/ExtendRental';
import { RescheduleOrder } from '../../../../../components/RescheduleOrder';
import { DiscountForm } from '../../../../../components/DiscountForm';
import { PaymentForm, PaymentList, paymentTypesFor, type PaymentType } from '../../../../../components/Payments';
import { StatusBadge } from '../../../../../components/StatusBadge';
import { StepCard } from '../../../../../components/StepCard';
import { api } from '../../../../../lib/api';
import { errorText } from '../../../../../lib/errors';
import { useBusiness } from '../../../../../lib/useMe';
import { useTexts } from '../../../../../lib/useTexts';
import { Button, Cell, Field, Group, Notice, Row, Screen, Step, SubStep, Text, useToast, useUI, type StepState } from '../../../../../ui';

type Booking = components['schemas']['Booking'];
type BookingStep = Booking['steps'][number];
const t = strings.bookings;
const qs = strings.quickSale;

const when = (iso: string) =>
  new Date(iso).toLocaleString('es-MX', { weekday: 'short', day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });

/** The most likely payment now: missing advance, deposit before check-out, late fee after a late return, else the balance. */
function suggestedPayment(b: Booking, security: number): { type: PaymentType; cents: number } {
  const lateFee = b.status === 'returned' ? b.rental?.late_fee_suggested ?? 0 : 0;
  if (b.status === 'pending_deposit' && b.balance.paid < b.deposit_required) return { type: 'advance', cents: b.deposit_required - b.balance.paid };
  if (engineFor(b.kind).custody && b.status === 'confirmed' && b.balance.deposit_held < security) return { type: 'security_deposit', cents: security - b.balance.deposit_held };
  if (lateFee > 0 && b.balance.extra_charges < lateFee) return { type: 'late_fee', cents: lateFee - b.balance.extra_charges };
  if (b.balance.due > 0) return { type: 'balance', cents: b.balance.due };
  if (b.balance.deposit_held > 0 && b.status === 'returned') return { type: 'refund', cents: b.balance.deposit_held };
  return { type: 'balance', cents: 0 };
}

export default function BookingDetail() {
  const { businessId, bookingId } = useLocalSearchParams<{ businessId: string; bookingId: string }>();
  const { business } = useBusiness(businessId);
  const [b, setB] = useState<Booking | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [confirmCancel, setConfirmCancel] = useState(false);
  const [ask, setAsk] = useState<{ to: Booking['status']; paid?: 'refund' | 'keep' } | null>(null);
  const [payOpen, setPayOpen] = useState(false);
  const [condition, setCondition] = useState('');
  const path = { businessId, bookingId };
  const ui = useUI();
  const toast = useToast();
  const { texts, has } = useTexts();

  const show = useCallback((data: Booking) => { setB(data); setPayOpen(false); }, []);

  // On focus: coming back from adding products (quick sale) shows them.
  useFocusEffect(useCallback(() => {
    api.GET('/v1/b/{businessId}/bookings/{bookingId}', { params: { path } })
      .then(({ data, error, response }) => (data ? show(data) : setLoadError(errorText(error, response.status))));
  }, [businessId, bookingId])); // eslint-disable-line react-hooks/exhaustive-deps

  // Moving money or confirming short of the advance asks first.
  function request(to: Booking['status'], paid?: 'refund' | 'keep') {
    if (paid || (to === 'confirmed' && b!.balance.paid < b!.deposit_required)) setAsk({ to, paid });
    else move(to);
  }

  async function move(to: Booking['status'], paid?: 'refund' | 'keep') {
    setAsk(null);
    setBusy(true);
    const { data, error, response } = await api.POST('/v1/b/{businessId}/bookings/{bookingId}/transition', { params: { path }, body: { to, paid } });
    setBusy(false);
    setConfirmCancel(false);
    if (data) { show(data); toast(t.statusChanged(texts.bookings.status[data.status])); return; }
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

  if (!b || !business) return loadError ? <Text style={{ padding: 16 }}>{loadError}</Text> : <ActivityIndicator style={{ marginTop: 24 }} />;
  // Tolerate an older API without the operation fields instead of crashing the screen.
  b.next ??= [];
  b.payments ??= [];
  b.events ??= [];
  b.steps ??= [];
  b.balance ??= { paid: 0, due: b.total, deposit_held: 0, extra_charges: 0 };
  const security = Number(b.items[0]?.options?.security_deposit ?? 0);
  const bookingTerm = term(business.vertical, 'booking') ?? t.detail;
  const e = engineFor(b.kind);
  // Check-out/return (custody) have their own flow; the rest are plain status moves.
  // "Iniciar" only where the business tracks the start (feature startService) and it can be skipped; an order always goes through it.
  const actions = b.next.filter((s) => !(e.custody && (s === 'checked_out' || s === 'returned')) && !(s === 'in_progress' && !has('startService') && b.next.includes('completed')));
  const hasPhone = b.customer.phone !== WALK_IN_PHONE;
  const waText = t.whatsappText(b.customer.name, bookingTerm, when(b.starts_at), texts.bookings.status[b.status]);
  const editable = !['cancelled', 'expired', 'no_show', 'completed'].includes(b.status);
  const f = t.detailFlow;
  const closedNote = ['cancelled', 'expired', 'no_show'].includes(b.status);

  // Steps grouped by moment: customer documents and out-photos before check-out; return photos after.
  // Additional documents (e.g. the promissory note) are steps marked config.section = 'additional'.
  const isAdditional = (st: BookingStep) => (st.config as { section?: string }).section === 'additional';
  const docs = b.steps.filter((st) => st.customer_doc !== '');
  const additional = b.steps.filter((st) => st.customer_doc === '' && isAdditional(st));
  const outPhotos = b.steps.filter((st) => st.customer_doc === '' && !isAdditional(st) && (st.to_status === 'checked_out' || !st.to_status));
  const backPhotos = b.steps.filter((st) => st.to_status === 'returned' || st.to_status === 'completed');
  const allDone = (list: BookingStep[]) => list.every((st) => st.status === 'done' || st.on_file || !st.required);
  const cards = (list: BookingStep[], canChange: boolean) => list.map((st) => (
    <StepCard key={st.id} businessId={businessId} bookingId={bookingId} step={st} onDone={show} editable={editable} canChange={canChange} />
  ));

  const balance = (
    <View style={{ gap: 6 }}>
      {b.discount > 0 && <Row label={t.discount.row(b.discount_type, b.discount_value)} value={`− ${formatMXN(b.discount)}`} />}
      <Row label={t.total} value={formatMXN(b.total)} />
      <Row label={t.paid} value={formatMXN(b.balance.paid)} />
      {b.deposit_required > 0 && <Row label={t.advance} value={formatMXN(b.deposit_required)} />}
      {(security > 0 || b.balance.deposit_held > 0) && <Row label={t.depositHeld} value={`${formatMXN(b.balance.deposit_held)} / ${formatMXN(security)}`} />}
      {b.balance.extra_charges > 0 && <Row label={t.extraCharges} value={formatMXN(b.balance.extra_charges)} />}
      <Row label={t.due} value={formatMXN(b.balance.due)} strong />
    </View>
  );
  // The payment form opens on demand, with only the payment types that apply at this moment; the
  // discount sits next to it (the API refuses it once the booking is finished).
  const payments = editable && (
    <View style={{ gap: 12 }}>
      <DiscountForm key={`d-${b.total}-${b.discount}`} businessId={businessId} booking={b} onDone={show} />
      {payOpen ? (
        <PaymentForm key={`${b.status}-${b.balance.paid}-${b.balance.extra_charges}-${b.balance.deposit_held}`} businessId={businessId} booking={b}
          types={paymentTypesFor(b, security)} suggested={suggestedPayment(b, security)} onDone={show} />
      ) : <Button icon="cash" onPress={() => setPayOpen(true)}>{t.showPaymentForm}</Button>}
    </View>
  );
  const settled = b.balance.due === 0 && (security === 0 || b.balance.deposit_held >= security);

  const rentalStep = { pending_deposit: 0, confirmed: 1, checked_out: 2, overdue: 2, returned: 3, completed: 4 }[b.status as string] ?? -1;
  const stepState = (i: number): StepState => (rentalStep > i ? 'done' : rentalStep === i ? 'current' : 'upcoming');
  const isOther = (st: Booking['status']) => st === 'cancelled' || st === 'no_show';
  const otherActions = actions.filter(isOther);
  const extrasTotal = b.extras.reduce((n, x) => n + x.total, 0);
  const extrasPaid = b.extras.reduce((n, x) => n + (x.method === 'on_account' ? 0 : x.total), 0); // on account: owed, not paid
  const paid = b.balance.paid + extrasPaid; // cancelling gives back (or keeps) the extras too

  return (
    <Screen>
      <Stack.Screen options={{ title: bookingTerm }} />
      <View style={{ gap: 16 }}>
        <View style={{ gap: 4 }}>
          {b.units.length > 0 && <Text variant="footnote" color="labelSecondary">{b.units.map((u) => u.name).join(', ')}</Text>}
          <Text variant="largeTitle" accessibilityRole="header">{b.items.map((i) => `${i.quantity > 1 ? `${i.quantity} × ` : ''}${i.name}`).join(', ')}</Text>
          {b.items.map((i) => (typeof i.options.description === 'string' && i.options.description
            ? <Text key={i.item_id} color="labelSecondary">{i.options.description}</Text> : null))}
          <StatusBadge status={b.status} size="md" />
          {b.status === 'overdue' && <Notice tone="warning">{f.overdue}</Notice>}
          {closedNote && <Notice tone="warning">{texts.bookings.inactive}</Notice>}
        </View>

        <Group separatorInset={58}>
          {b.customer.walk_in ? (
            // A walk-in may have left a name and a phone on this booking only.
            <Cell icon="walk" tint={ui.indigo} label={b.customer.name}
              sublabel={hasPhone ? formatPhone(b.customer.phone) : strings.customers.walkInHint} />
          ) : (
            <Cell icon="person" tint={ui.indigo} label={b.customer.name} sublabel={formatPhone(b.customer.phone)}
              onPress={() => router.push({ pathname: '/admin/[businessId]/customers/[customerId]', params: { businessId, customerId: b.customer.id } })} />
          )}
          {hasPhone ? (
            <Cell icon="logo-whatsapp" tint={ui.success} label={t.whatsapp}
              onPress={() => Linking.openURL(`https://wa.me/${b.customer.phone.replace(/\D/g, '')}?text=${encodeURIComponent(waText)}`)} />
          ) : null}
          <Cell icon="arrow-up-circle" tint={ui.accent} label={term(business.vertical, 'starts_at') ?? ''}
            value={e.timing === 'day' && b.ends_at ? `${when(b.starts_at)} ${t.windowUntil(hhmm(b.ends_at))}` : when(b.starts_at)} />
          {b.recipient_name || b.recipient_phone ? (
            <Cell icon="gift" tint={ui.purple} label={`${t.gift.recipient}: ${b.recipient_name ?? ''}`.trim()} sublabel={b.recipient_phone ? formatPhone(b.recipient_phone) : undefined}
              onPress={b.recipient_phone ? () => Linking.openURL(`tel:${b.recipient_phone}`) : undefined} />
          ) : null}
          {b.card_message ? <Cell icon="mail" tint={ui.purple} label={t.gift.cardLabel} sublabel={b.card_message} /> : null}
          {e.timing === 'range' && b.ends_at ? <Cell icon="arrow-down-circle" tint={ui.warning} label={term(business.vertical, 'ends_at') ?? ''} value={when(b.ends_at)} /> : null}
          {b.staff.length > 0 ? <Cell icon="person-circle" tint={ui.gray} label={t.attendedBy} value={b.staff.map((x) => x.name).join(', ')} /> : null}
          {b.fulfillment === 'delivery' ? <Cell icon={e.fulfillment === 'atPlaceOrHome' ? 'home' : 'car'} tint={ui.gray} label={e.fulfillment === 'atPlaceOrHome' ? t.homeService : t.delivery} sublabel={b.delivery_address ?? ''} /> : null}
          {b.hold_expires_at ? <Cell icon="hourglass" tint={ui.warning} label={t.holdUntil} value={when(b.hold_expires_at)} /> : null}
          {b.notes ? <Cell icon="document-text" tint={ui.gray} label={b.notes} /> : null}
        </Group>
        {e.timing === 'day' && ['pending_deposit', 'confirmed', 'in_progress', 'ready'].includes(b.status) && (
          <RescheduleOrder businessId={businessId} booking={b} onDone={show} />
        )}

        {e.custody && !closedNote ? (
          <>
            <Step n={1} title={f.confirm.title} description={f.confirm.description} state={stepState(0)}>
              {rentalStep === 0 && (
                <>
                  <SubStep title={f.sub.payment} description={f.sub.advanceHint}>{balance}{payments}</SubStep>
                  {actions.includes('confirmed') && <Button variant="primary" busy={busy} onPress={() => request('confirmed')}>{t.actions.confirmed ?? texts.bookings.status.confirmed}</Button>}
                </>
              )}
            </Step>

            <Step n={2} title={f.checkout.title} description={f.checkout.description} state={stepState(1)}>
              {rentalStep === 1 && (
                <>
                  {docs.length > 0 && <SubStep title={f.sub.documents} description={f.sub.documentsHint} done={allDone(docs)}>{cards(docs, true)}</SubStep>}
                  {(additional.length > 0 || has('bookingDocuments')) && (
                    <SubStep title={t.docs.section} description={t.docs.sectionHint} done={additional.length ? allDone(additional) : undefined}>
                      {cards(additional, true)}
                      {has('bookingDocuments') && <BookingDocuments businessId={businessId} booking={b} editable={editable} onDone={show} />}
                    </SubStep>
                  )}
                  {outPhotos.length > 0 && <SubStep title={f.sub.outPhotos} description={f.sub.outPhotosHint} done={allDone(outPhotos)}>{cards(outPhotos, true)}</SubStep>}
                  <SubStep title={f.sub.payment} description={f.sub.checkoutPaymentHint} done={settled}>{balance}{payments}</SubStep>
                  <SubStep title={f.sub.doCheckout} description={f.sub.doCheckoutHint}>
                    <Field label={t.checkoutCondition} value={condition} onChangeText={setCondition} />
                    <Button variant="primary" icon="arrow-up-circle" busy={busy} onPress={() => rentalMove('checkout')}>{t.doCheckout}</Button>
                  </SubStep>
                </>
              )}
              {rentalStep > 1 && b.rental?.checked_out_at && (
                <SubStep title={`${t.outAt}: ${when(b.rental.checked_out_at)}`} description={b.rental.out_condition ?? undefined}>{cards(outPhotos, false)}</SubStep>
              )}
            </Step>

            <Step n={3} title={f.return.title} description={f.return.description} state={stepState(2)}>
              {rentalStep === 2 && (
                <>
                  {b.ends_at && <SubStep title={f.sub.extend} description={f.sub.extendHint}>
                    <ExtendRental businessId={businessId} booking={b} onDone={(nb) => { show(nb); toast(t.extended(when(nb.ends_at!), formatMXN(nb.total))); }} />
                  </SubStep>}
                  {backPhotos.length > 0 && <SubStep title={f.sub.backPhotos} description={f.sub.backPhotosHint} done={allDone(backPhotos)}>{cards(backPhotos, true)}</SubStep>}
                  <SubStep title={f.sub.doReturn} description={f.sub.doReturnHint}>
                    <Field label={t.returnCondition} value={condition} onChangeText={setCondition} />
                    <Button variant="primary" icon="arrow-down-circle" busy={busy} onPress={() => rentalMove('return')}>{t.doReturn}</Button>
                  </SubStep>
                </>
              )}
              {rentalStep > 2 && b.rental?.returned_at && (
                <SubStep title={`${t.backAt}: ${when(b.rental.returned_at)}`} description={b.rental.return_condition ?? undefined} />
              )}
            </Step>

            <Step n={4} title={f.close.title} description={f.close.description} state={stepState(3)}>
              {rentalStep === 3 && (
                <>
                  {backPhotos.some((st) => st.status !== 'done') && (
                    <SubStep title={f.sub.backPhotos} description={f.sub.backPhotosHint} done={false}>{cards(backPhotos, true)}</SubStep>
                  )}
                  <SubStep title={f.sub.closePayment} description={f.sub.closePaymentHint} done={settled && b.balance.deposit_held === 0}>
                    {b.rental && b.rental.late_days > 0 && (
                      <Notice tone="warning">{`${t.lateDays(b.rental.late_days)} · ${t.lateFeeHint}: ${formatMXN(b.rental.late_fee_suggested)}`}</Notice>
                    )}
                    {balance}
                    {payments}
                  </SubStep>
                  {actions.includes('completed') && <Button variant="primary" icon="checkmark-circle" busy={busy} onPress={() => move('completed')}>{t.actions.completed ?? texts.bookings.status.completed}</Button>}
                </>
              )}
            </Step>
          </>
        ) : (
          <>
            <Group label={t.total} padded><View style={{ gap: 12 }}>{balance}{payments}</View></Group>
            {(b.extras.length > 0 || (has('quickSale') && editable)) && (
              <Group label={qs.extras} footer={b.extras.length ? undefined : qs.addToOrderHint} padded>
                <View style={{ gap: 8 }}>
                  {b.extras.map((x) => <Row key={x.id} label={`${x.items} · ${t.methods[x.method]}`} value={formatMXN(x.total)} />)}
                  {b.extras.length > 0 && <Row label={qs.totalWithExtras} value={formatMXN(b.total + extrasTotal)} strong />}
                  {has('quickSale') && editable && (
                    <Button icon="add-circle" onPress={() => router.push({ pathname: '/admin/[businessId]/sales/new', params: { businessId, booking: b.id, who: b.customer.name } })}>
                      {qs.addToOrder}
                    </Button>
                  )}
                </View>
              </Group>
            )}
            {editable && actions.filter((st) => !isOther(st)).length > 0 && (
              <Group label={t.actionsTitle} padded>
                <View style={{ gap: 8 }}>
                  {actions.filter((st) => !isOther(st)).map((st) => (
                    <Button key={st} busy={busy} onPress={() => request(st)}>{t.actions[st] ?? texts.bookings.status[st]}</Button>
                  ))}
                </View>
              </Group>
            )}
            {b.steps.length > 0 && <Group label={t.steps} padded><View style={{ gap: 10 }}>{cards(b.steps, true)}</View></Group>}
          </>
        )}

        {/* After the check-out (and for other kinds) the documents stay at hand in their own section. */}
        {(!e.custody || rentalStep > 1 || closedNote) && (additional.length > 0 || has('bookingDocuments')) && (
          <View>
            <Text variant="footnote" color="labelSecondary" style={{ marginHorizontal: 16, marginBottom: 6 }}>{(texts.bookings.documents?.section ?? t.docs.section).toUpperCase()}</Text>
            <View style={{ gap: 10 }}>
              {cards(additional, false)}
              {has('bookingDocuments') && <BookingDocuments businessId={businessId} booking={b} editable={editable} onDone={show} />}
            </View>
          </View>
        )}

        {b.payments.length > 0 && (
          <View>
            <Text variant="footnote" color="labelSecondary" style={{ marginHorizontal: 16, marginBottom: 6 }}>{t.payments.toUpperCase()}</Text>
            <PaymentList businessId={businessId} booking={b} />
          </View>
        )}

        <Group label={t.history}>
          {b.events.map((e, i) => (
            <Cell key={i} label={`${e.from_status ? `${texts.bookings.status[e.from_status as Booking['status']] ?? e.from_status} → ` : `${t.created}: `}${texts.bookings.status[e.to_status as Booking['status']] ?? e.to_status}`}
              sublabel={`${when(e.at)}${e.note ? ` — ${e.note}` : ''}`} />
          ))}
        </Group>

        {otherActions.length > 0 && (
          <Group label={t.otherActions} footer={confirmCancel ? (paid > 0 ? t.confirmCancelPaid(formatMXN(paid)) + (extrasPaid ? t.cancelExtras(formatMXN(extrasPaid)) : '') : t.confirmCancel) : undefined}>
            {otherActions.map((st) => (st === 'cancelled' && confirmCancel && paid > 0 ? (
              <View key={st}>
                <Cell label={t.cancelRefund(formatMXN(paid))} destructive onPress={() => request(st, 'refund')} />
                <Cell label={t.cancelKeep(formatMXN(paid))} destructive onPress={() => request(st, 'keep')} />
              </View>
            ) : (
              // A no-show keeps what was paid (the usual rule for a missed appointment).
              <Cell key={st} label={st === 'cancelled' && confirmCancel ? t.yesCancel : t.actions[st] ?? texts.bookings.status[st]} destructive
                onPress={() => (st === 'cancelled' && !confirmCancel ? setConfirmCancel(true) : move(st, paid > 0 ? 'keep' : undefined))} />
            )))}
          </Group>
        )}
      </View>
      {ask && (
        <ConfirmDialog open busy={busy} primary={!ask.paid} onClose={() => setAsk(null)} onConfirm={() => move(ask.to, ask.paid)}
          title={ask.paid === 'refund' ? t.askRefund(formatMXN(paid)) : ask.paid === 'keep' ? t.askKeep(formatMXN(paid)) : t.askConfirmShort(formatMXN(b.deposit_required - b.balance.paid))}
          body={ask.paid === 'refund' ? t.askRefundBody : ask.paid === 'keep' ? t.askKeepBody : t.askConfirmShortBody}
          confirmLabel={ask.paid === 'refund' ? t.cancelRefund(formatMXN(paid)) : ask.paid === 'keep' ? t.cancelKeep(formatMXN(paid)) : t.confirmAnyway} />
      )}
    </Screen>
  );
}
