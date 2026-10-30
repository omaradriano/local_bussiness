import type { components } from '@lb/api-client';
import { addDays, engineFor, firstOfMonth, futureTime, nextHour, parsePesos, formatMXN, hhmm, lastOfMonth, localToISO, strings, term, todayISO } from '@lb/shared';
import { router, Stack, useLocalSearchParams } from 'expo-router';
import { useEffect, useMemo, useState } from 'react';
import { ActivityIndicator, Switch, View } from 'react-native';
import { Calendar, type DayInfo } from '../../../../../components/Calendar';
import { CustomerPicker, type PickedCustomer } from '../../../../../components/CustomerPicker';
import { api } from '../../../../../lib/api';
import { errorText, fieldErrors } from '../../../../../lib/errors';
import { useBusiness } from '../../../../../lib/useMe';
import { useSettings } from '../../../../../lib/useSettings';
import { useTexts } from '../../../../../lib/useTexts';
import { Button, Chip, Chips, Field as UIField, Notice, Row, Screen, Segmented, Step, Text, useToast, type StepState } from '../../../../../ui';

type Item = components['schemas']['CatalogItem'];
type Availability = components['schemas']['Availability'];
type Slot = NonNullable<Availability['slots']>[number];
type Day = NonNullable<Availability['days']>[number];
const t = strings.bookings;

function Field({ label, value, onChange, error, keyboard, hint, onBlur }: {
  label: string; value: string; onChange: (v: string) => void; error?: string; keyboard?: 'phone-pad' | 'numeric'; hint?: string; onBlur?: () => void;
}) {
  return <UIField label={label} value={value} onChangeText={onChange} keyboardType={keyboard} error={error} hint={hint} onBlur={onBlur} />;
}

type Quote = { total: number; security_deposit: number; periods: number };

export default function NewBooking() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const { business } = useBusiness(businessId);
  const { settings } = useSettings(businessId);
  const { texts, has } = useTexts();
  const toast = useToast();
  const today = todayISO();
  const [items, setItems] = useState<Item[] | null>(null);
  const [staff, setStaff] = useState<{ id: string; name: string; service_ids: string[]; active: boolean }[]>([]);
  const [itemId, setItemId] = useState<string | null>(null);
  const item = items?.find((i) => i.id === itemId) ?? null;
  // The form follows the item's engine modules, not its name (see engines.json).
  const e = item ? engineFor(item.kind) : null;
  const isSlot = e?.timing === 'slot'; // pick a time slot (and who attends)
  const isDay = e?.timing === 'day'; // pick a day and a time
  const isRange = e?.timing === 'range'; // pick from–to and check the exact range

  // What's being booked (kept as typed; converted on save).
  const [month, setMonth] = useState(firstOfMonth(today));
  const [from, setFrom] = useState<string | null>(null); // day (service/product) or pick-up day (rental)
  const [to, setTo] = useState<string | null>(null); // return day (rental)
  // By default an hour from now: a start time in the past would read as "no units free".
  const [time, setTime] = useState(() => nextHour());
  const [backTime, setBackTime] = useState(() => nextHour());
  const [quantity, setQuantity] = useState('1');
  const [slot, setSlot] = useState<Slot | null>(null);
  const [staffId, setStaffId] = useState<string | null>(null);
  const [fulfillment, setFulfillment] = useState<'pickup' | 'delivery'>('pickup');
  const [address, setAddress] = useState('');
  const [atHome, setAtHome] = useState(false); // service at the customer's address (feature homeService)
  const [zone, setZone] = useState('');
  const [customer, setCustomer] = useState<PickedCustomer | null>(null);
  const [notes, setNotes] = useState('');
  const [payMode, setPayMode] = useState<'none' | 'total' | 'partial'>('none'); // the real payment is usually at check-out
  const [payMethod, setPayMethod] = useState<'cash' | 'transfer' | 'card'>('cash');
  const [payAmount, setPayAmount] = useState('');

  const [days, setDays] = useState<Record<string, Day>>({}); // calendar availability of the visible month
  const [daysState, setDaysState] = useState<'loading' | 'ready' | string>('loading'); // string = error
  const [slots, setSlots] = useState<Slot[] | null>(null);
  const [exact, setExact] = useState<Availability | null>(null); // rental: the exact range with times
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [refresh, setRefresh] = useState(0); // bump to re-ask availability (after a 409)
  // One key per form: a double tap or retry never creates two bookings.
  const idempotencyKey = useMemo(() => `adm-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`, []);
  const qty = Math.max(1, Number(quantity) || 1);

  useEffect(() => {
    const path = { businessId };
    api.GET('/v1/b/{businessId}/catalog', { params: { path } }).then(({ data }) => setItems(data?.items.filter((i) => i.active) ?? []));
    api.GET('/v1/b/{businessId}/staff', { params: { path } }).then(({ data }) => setStaff(data?.staff ?? []));
  }, [businessId]);

  // Picking today with a time already past: show the corrected time right away.
  useEffect(() => { if (from) setTime((tm) => futureTime(from, tm)); }, [from]);

  // New item: start over.
  useEffect(() => { setFrom(null); setTo(null); setSlot(null); setStaffId(null); setMessage(null); }, [itemId]);

  // Products and rentals: paint the visible month (closed / full / no free units are disabled).
  useEffect(() => {
    setDays({});
    setDaysState('loading');
    if (!item || isSlot) return;
    const start = month < today ? today : month;
    const end = lastOfMonth(month);
    if (start > end) return setDaysState('ready');
    api.GET('/v1/b/{businessId}/availability', {
      params: { path: { businessId }, query: { item_id: item.id, from: start, to: end, quantity: qty } },
    }).then(({ data, error, response }) => {
      if (!data) return setDaysState(`${t.availabilityError} ${errorText(error, response.status)}`);
      setDays(Object.fromEntries((data.days ?? []).map((d) => [d.date, d])));
      setDaysState('ready');
    });
  }, [businessId, item, month, qty, today, refresh]);

  // Who can do this service. With more than one person the form asks who attends first, and then
  // shows only that person's free times ("Cualquiera" = anyone free; the API assigns one).
  const attendants = staff.filter((p) => p.active && !!item && p.service_ids.includes(item.id));
  useEffect(() => { setStaffId(null); }, [itemId]);

  // Services: slots of the chosen day (for the chosen person, if any).
  useEffect(() => {
    setSlots(null); setSlot(null);
    if (!item || !isSlot || !from) return;
    api.GET('/v1/b/{businessId}/availability', { params: { path: { businessId }, query: { item_id: item.id, date: from, ...(staffId ? { staff_id: staffId } : {}) } } })
      .then(({ data, error, response }) => (data ? setSlots(data.slots ?? []) : setMessage(errorText(error, response.status))));
  }, [businessId, item, from, staffId, refresh]);

  // Rentals: check the exact range (days + times) as soon as it is complete.
  // A past time today counts as the next full hour (the field shows it on blur).
  const outISO = from ? localToISO(from, futureTime(from, time)) : null;
  const backISO = to ? localToISO(to, backTime) : null;
  useEffect(() => {
    setExact(null);
    if (!item || !isRange || !outISO || !backISO) return;
    if (backISO <= outISO) return setMessage(t.sameInstant);
    setMessage(null);
    api.GET('/v1/b/{businessId}/availability', { params: { path: { businessId }, query: { item_id: item.id, from: outISO, to: backISO, quantity: qty } } })
      .then(({ data, error, response }) => (data ? setExact(data) : setMessage(errorText(error, response.status))));
  }, [businessId, item, outISO, backISO, qty]);

  // Price as soon as the dates (or the slot) are known: the API applies the same rules as when creating.
  const [quote, setQuote] = useState<Quote | null>(null);
  const quoteStart = isSlot ? slot?.starts_at : outISO;
  const quoteReady = !!item && !!quoteStart && (!isRange || !!exact?.available);
  useEffect(() => {
    setQuote(null);
    if (!item || !quoteReady || !quoteStart) return;
    api.GET('/v1/b/{businessId}/quote', {
      params: { path: { businessId }, query: { item_id: item.id, quantity: qty, starts_at: quoteStart, ...(isRange && backISO ? { ends_at: backISO } : {}) } },
    }).then(({ data }) => setQuote(data ?? null));
  }, [businessId, item, qty, quoteStart, backISO, quoteReady]);

  function dayInfo(date: string): DayInfo {
    if (date < today) return { disabled: true };
    if (!item || isSlot) return {};
    const d = days[date];
    if (!d) return { disabled: true }; // not loaded yet or outside the range
    if (d.closed) return { disabled: true, badge: t.closedBadge }; // no pick-up/return, but a rental can span it
    if (!d.available) return { disabled: true };
    if (e?.resource === 'units') return { badge: t.freeBadge(d.remaining) };
    return d.remaining >= 0 ? { badge: t.countBadge(d.remaining) } : {};
  }

  function pick(date: string) {
    setMessage(null);
    if (!isRange) return setFrom(date);
    if (!from || to || date < from) { setFrom(date); setTo(null); return; }
    // A rental range can't cross a day without free units; a closed day in between is fine (the tool
    // stays with the customer). Only the visible month is loaded; the exact check of the whole range runs right after.
    const spanClosed = settings?.rentals.rentals_span_closed_days ?? true;
    for (let d = from; d <= date; d = addDays(d, 1)) {
      if (days[d] && days[d].remaining < qty) { toast(t.rangeBlocked, 'error'); return; }
      if (!spanClosed && days[d]?.closed) { toast(t.closedNotAllowed, 'error'); return; }
    }
    setTo(date);
  }

  async function save() {
    if (!item) return;
    const startsAt = isSlot ? slot?.starts_at : outISO;
    if (!startsAt) return toast(isRange ? t.pickRange : t.pickDay, 'error');
    if (isRange && !backISO) return toast(t.pickReturn, 'error');
    if (!customer) return toast(strings.customers.pick, 'error');
    const partial = payMode === 'partial' ? parsePesos(payAmount) : null;
    if (payMode === 'partial' && !partial) return setErrors({ amount: t.invalidAmount });
    setBusy(true);
    setErrors({});
    setMessage(null);
    const res = await api.POST('/v1/b/{businessId}/bookings', {
      params: { path: { businessId }, header: { 'Idempotency-Key': idempotencyKey } },
      body: {
        customer: customer === 'walk-in' ? { walk_in: true } : { id: customer.id },
        item_id: item.id,
        quantity: qty,
        starts_at: startsAt,
        ...(isRange && backISO ? { ends_at: backISO } : {}),
        ...(staffId ? { staff_id: staffId } : {}),
        ...(e?.fulfillment === 'pickupOrDelivery' ? { fulfillment, delivery_address: address, delivery_zone: zone } : {}),
        ...(e?.fulfillment === 'atPlaceOrHome' && atHome ? { fulfillment: 'delivery' as const, delivery_address: address } : {}),
        notes,
        ...(payMode !== 'none' ? { initial_payment: { method: payMethod, ...(partial ? { amount: partial } : {}) } } : {}),
      },
    });
    setBusy(false);
    if (res.data) {
      return router.replace({ pathname: '/admin/[businessId]/bookings/[bookingId]', params: { businessId, bookingId: res.data.id } });
    }
    if (res.response.status === 409) { setSlot(null); setTo(null); setRefresh((n) => n + 1); return toast(t.taken, 'error'); }
    setErrors(fieldErrors(res.error));
    toast(errorText(res.error, res.response.status), 'error');
  }

  if (!business || !items) return <ActivityIndicator style={{ marginTop: 24 }} />;
  const bookingTerm = term(business.vertical, 'booking') ?? '';
  const canSave = !!item && (isSlot ? !!slot : isDay ? !!from && !!outISO : !!exact?.available);
  const paidNow = payMode === 'total' ? quote?.total ?? 0 : payMode === 'partial' ? parsePesos(payAmount) ?? 0 : 0;
  const remaining = quote ? Math.max(quote.total - paidNow, 0) : null;
  // Each step is done once its data is there; the first one not done is the current one.
  const doneFlags = [!!item, canSave, !!customer, true];
  const current = doneFlags.indexOf(false);
  const state = (i: number): StepState => (doneFlags[i] && (current === -1 || i < current) ? 'done' : i === current || current === -1 ? 'current' : 'upcoming');
  const s = t.flow;

  return (
    <Screen>
      <Stack.Screen options={{ title: t.new(bookingTerm) }} />
      <View style={{ gap: 16 }}>
        <Step inline n={1} title={s.item.title(term(business.vertical, 'catalog_item') ?? '')} description={texts.newBooking.itemHint} state={state(0)}>
          <Chips>
            {items.map((i) => (
              <Chip key={i.id} selected={i.id === itemId} onPress={() => setItemId(i.id)}>{`${i.name} · ${formatMXN(i.price)}`}</Chip>
            ))}
          </Chips>
          {item && !isSlot && (
            <Field label={t.quantity} value={quantity} onChange={setQuantity} keyboard="numeric" error={errors.quantity} />
          )}
        </Step>

        <Step inline n={2} title={isRange ? s.dates.title : s.when.title}
          description={isRange ? s.dates.description : s.when.description} state={state(1)}>
          {item && (
            <>
              <Text variant="subhead" color="labelSecondary">
                {isRange ? (from && !to ? t.pickReturn : t.pickRange) : t.pickDay}
              </Text>
              <Calendar month={month} onMonthChange={setMonth} minMonth={firstOfMonth(today)} from={from} to={isRange ? to : null}
                onSelect={pick} dayInfo={dayInfo} />
              {isRange && (settings?.rentals.rentals_span_closed_days ?? true) && <Text variant="footnote" color="labelSecondary">{t.closedDayHint}</Text>}
              {daysState !== 'loading' && daysState !== 'ready' && <Notice tone="warning">{daysState}</Notice>}
              {e?.resource === 'units' && daysState === 'ready' && Object.values(days).every((d) => d.remaining === 0) && (
                <View style={{ gap: 8 }}>
                  <Notice tone="warning">{t.noUnits}</Notice>
                  <Button size="sm" onPress={() => router.push({ pathname: '/admin/[businessId]/catalog/[itemId]', params: { businessId, itemId: item.id } })}>{t.goToUnits}</Button>
                </View>
              )}
            </>
          )}

          {e?.resource === 'staff' && attendants.length > 1 && (
            <View style={{ gap: 8 }}>
              <Text variant="subhead" color="labelSecondary">{t.whoAttends}</Text>
              <Chips>
                <Chip selected={staffId === null} onPress={() => setStaffId(null)}>{t.anyone}</Chip>
                {attendants.map((p) => <Chip key={p.id} selected={staffId === p.id} onPress={() => setStaffId(p.id)}>{p.name}</Chip>)}
              </Chips>
            </View>
          )}
          {isSlot && from && (
            <>
              {e?.fulfillment === 'atPlaceOrHome' && has('homeService') && (
                <View style={{ gap: 8 }}>
                  <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12 }}>
                    <Text style={{ flex: 1 }}>{t.homeService}</Text>
                    <Switch accessibilityLabel={t.homeService} value={atHome} onValueChange={(on) => {
                      setAtHome(on);
                      // Start from the customer's address when there is one.
                      if (on && !address && customer && customer !== 'walk-in' && customer.address) setAddress(customer.address);
                    }} />
                  </View>
                  {atHome && <Field label={t.homeAddress} value={address} onChange={setAddress} error={errors.delivery_address} />}
                </View>
              )}
              {slots === null && <ActivityIndicator />}
              {slots?.length === 0 && <Text>{t.noSlots}</Text>}
              {!!slots?.length && (
                <Chips>
                  {slots.map((sl) => (
                    <Chip key={sl.starts_at} selected={slot?.starts_at === sl.starts_at} onPress={() => setSlot(sl)}>{hhmm(sl.starts_at)}</Chip>
                  ))}
                </Chips>
              )}
            </>
          )}

          {isDay && from && (
            <>
              <Field label={t.deliveryTime} value={time} onChange={setTime} error={errors.starts_at} onBlur={() => from && setTime(futureTime(from, time))} />
              {e?.fulfillment === 'pickupOrDelivery' && <Segmented options={[{ value: 'pickup', label: t.pickup }, { value: 'delivery', label: t.delivery }]} value={fulfillment} onChange={setFulfillment} />}
              {e?.fulfillment === 'pickupOrDelivery' && fulfillment === 'delivery' && (
                <>
                  <Field label={t.address} value={address} onChange={setAddress} error={errors.delivery_address} />
                  <Field label={t.zone} value={zone} onChange={setZone} error={errors.delivery_zone} />
                </>
              )}
            </>
          )}

          {isRange && from && (
            <View style={{ flexDirection: 'row', gap: 12 }}>
              <View style={{ flex: 1 }}><Field label={t.outTime} value={time} onChange={setTime} hint={s.dates.timeHint} onBlur={() => from && setTime(futureTime(from, time))} /></View>
              {to && <View style={{ flex: 1 }}><Field label={t.backTime} value={backTime} onChange={setBackTime} error={errors.ends_at} /></View>}
            </View>
          )}
          {isRange && to && !exact && !message && <Text color="labelSecondary">{t.checking}</Text>}
          {isRange && exact && (
            <Notice tone={exact.available ? 'success' : 'warning'}>{exact.available ? t.free(exact.free_units?.length ?? 0) : t.notEnough}</Notice>
          )}
          {quote && (
            <View style={{ gap: 6 }}>
              {e?.pricing === 'perPeriod' && <Row label={s.dates.days} value={String(quote.periods)} />}
              <Row label={t.total} value={formatMXN(quote.total)} strong />
              {quote.security_deposit > 0 && <Row label={s.dates.deposit} value={formatMXN(quote.security_deposit)} />}
            </View>
          )}
        </Step>

        <Step inline n={3} title={s.customer.title} description={texts.newBooking.customerHint} state={state(2)}>
          {item && (
            <CustomerPicker businessId={businessId} isOwner={business.role === 'owner'} value={customer} onChange={setCustomer}
              error={errors['customer.id'] ?? errors['customer.name']} />
          )}
        </Step>

        <Step inline n={4} title={s.payment.title} description={texts.newBooking.paymentHint} state={item ? state(3) : 'upcoming'}>
          {item && (
            <>
              <Segmented options={(['none', 'total', 'partial'] as const).map((m) => ({ value: m, label: t.payModes[m] }))} value={payMode} onChange={setPayMode} />
              {payMode !== 'none' && (
                <Segmented options={(['cash', 'transfer', 'card'] as const).map((m) => ({ value: m, label: t.methods[m] }))} value={payMethod} onChange={setPayMethod} />
              )}
              {payMode === 'partial' && <Field label={t.amount} value={payAmount} onChange={setPayAmount} keyboard="numeric" error={errors.amount} />}
              {quote && (
                <View style={{ gap: 6 }}>
                  <Row label={t.total} value={formatMXN(quote.total)} />
                  <Row label={s.payment.now} value={formatMXN(paidNow)} />
                  <Row label={s.payment.remaining} value={formatMXN(remaining ?? 0)} strong />
                </View>
              )}
              <Field label={t.notes} value={notes} onChange={setNotes} />
              {Object.entries(errors).filter(([k]) => k.startsWith('options.')).map(([k, v]) => <Notice key={k} tone="warning">{v}</Notice>)}
              {message && <Notice tone="warning">{message}</Notice>}
              <Button variant="primary" busy={busy} onPress={save} disabled={!canSave || !customer}>{t.save}</Button>
            </>
          )}
        </Step>
      </View>
    </Screen>
  );
}
