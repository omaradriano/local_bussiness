import type { components } from '@lb/api-client';
import { addDays, firstOfMonth, formatMXN, hhmm, lastOfMonth, localToISO, strings, term, todayISO } from '@lb/shared';
import { Link, router, Stack, useLocalSearchParams } from 'expo-router';
import { useEffect, useMemo, useState } from 'react';
import { ActivityIndicator, Button, Pressable, ScrollView, Text, TextInput, View } from 'react-native';
import { Calendar, type DayInfo } from '../../../../../components/Calendar';
import type { Customer } from '../../../../../components/CustomerForm';
import { CustomerPicker } from '../../../../../components/CustomerPicker';
import { api } from '../../../../../lib/api';
import { errorText, fieldErrors } from '../../../../../lib/errors';
import { useBusiness } from '../../../../../lib/useMe';
import { useSettings } from '../../../../../lib/useSettings';

type Item = components['schemas']['CatalogItem'];
type Availability = components['schemas']['Availability'];
type Slot = NonNullable<Availability['slots']>[number];
type Day = NonNullable<Availability['days']>[number];
const t = strings.bookings;

const input = { borderWidth: 1, borderRadius: 6, padding: 10 } as const;

function Choice({ selected, label, onPress }: { selected: boolean; label: string; onPress: () => void }) {
  return (
    <Pressable accessibilityRole="radio" accessibilityState={{ checked: selected }} onPress={onPress}
      style={{ borderWidth: selected ? 2 : 1, borderRadius: 6, paddingVertical: 8, paddingHorizontal: 10 }}>
      <Text>{label}</Text>
    </Pressable>
  );
}

function Field({ label, value, onChange, error, keyboard }: {
  label: string; value: string; onChange: (v: string) => void; error?: string; keyboard?: 'phone-pad' | 'numeric';
}) {
  return (
    <View style={{ gap: 4 }}>
      <Text>{label}</Text>
      <TextInput accessibilityLabel={label} value={value} onChangeText={onChange} keyboardType={keyboard} style={input} />
      {error && <Text accessibilityRole="alert">{error}</Text>}
    </View>
  );
}

export default function NewBooking() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const { business } = useBusiness(businessId);
  const { settings } = useSettings(businessId);
  const today = todayISO();
  const [items, setItems] = useState<Item[] | null>(null);
  const [staffNames, setStaffNames] = useState<Record<string, string>>({});
  const [itemId, setItemId] = useState<string | null>(null);
  const item = items?.find((i) => i.id === itemId) ?? null;

  // What's being booked (kept as typed; converted on save).
  const [month, setMonth] = useState(firstOfMonth(today));
  const [from, setFrom] = useState<string | null>(null); // day (service/product) or pick-up day (rental)
  const [to, setTo] = useState<string | null>(null); // return day (rental)
  const [time, setTime] = useState('10:00');
  const [backTime, setBackTime] = useState('10:00');
  const [quantity, setQuantity] = useState('1');
  const [slot, setSlot] = useState<Slot | null>(null);
  const [staffId, setStaffId] = useState<string | null>(null);
  const [fulfillment, setFulfillment] = useState<'pickup' | 'delivery'>('pickup');
  const [address, setAddress] = useState('');
  const [zone, setZone] = useState('');
  const [customer, setCustomer] = useState<Customer | null>(null);
  const [notes, setNotes] = useState('');

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
    api.GET('/v1/b/{businessId}/staff', { params: { path } }).then(({ data }) =>
      setStaffNames(Object.fromEntries((data?.staff ?? []).map((s) => [s.id, s.name]))));
  }, [businessId]);

  // New item: start over.
  useEffect(() => { setFrom(null); setTo(null); setSlot(null); setStaffId(null); setMessage(null); }, [itemId]);

  // Products and rentals: paint the visible month (closed / full / no free units are disabled).
  useEffect(() => {
    setDays({});
    setDaysState('loading');
    if (!item || item.kind === 'service') return;
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

  // Services: slots of the chosen day.
  useEffect(() => {
    setSlots(null); setSlot(null); setStaffId(null);
    if (!item || item.kind !== 'service' || !from) return;
    api.GET('/v1/b/{businessId}/availability', { params: { path: { businessId }, query: { item_id: item.id, date: from } } })
      .then(({ data, error, response }) => (data ? setSlots(data.slots ?? []) : setMessage(errorText(error, response.status))));
  }, [businessId, item, from, refresh]);

  // Rentals: check the exact range (days + times) as soon as it is complete.
  const outISO = from ? localToISO(from, time) : null;
  const backISO = to ? localToISO(to, backTime) : null;
  useEffect(() => {
    setExact(null);
    if (!item || item.kind !== 'rental' || !outISO || !backISO) return;
    if (backISO <= outISO) return setMessage(t.sameInstant);
    setMessage(null);
    api.GET('/v1/b/{businessId}/availability', { params: { path: { businessId }, query: { item_id: item.id, from: outISO, to: backISO, quantity: qty } } })
      .then(({ data, error, response }) => (data ? setExact(data) : setMessage(errorText(error, response.status))));
  }, [businessId, item, outISO, backISO, qty]);

  function dayInfo(date: string): DayInfo {
    if (date < today) return { disabled: true };
    if (!item || item.kind === 'service') return {};
    const d = days[date];
    if (!d) return { disabled: true }; // not loaded yet or outside the range
    if (d.closed) return { disabled: true, badge: t.closedBadge }; // no pick-up/return, but a rental can span it
    if (!d.available) return { disabled: true };
    if (item.kind === 'rental') return { badge: t.freeBadge(d.remaining) };
    return d.remaining >= 0 ? { badge: t.countBadge(d.remaining) } : {};
  }

  function pick(date: string) {
    setMessage(null);
    if (item?.kind !== 'rental') return setFrom(date);
    if (!from || to || date < from) { setFrom(date); setTo(null); return; }
    // A rental range can't cross a day without free units; a closed day in between is fine (the tool
    // stays with the customer). Only the visible month is loaded; the exact check of the whole range runs right after.
    const spanClosed = settings?.rentals.rentals_span_closed_days ?? true;
    for (let d = from; d <= date; d = addDays(d, 1)) {
      if (days[d] && days[d].remaining < qty) { setMessage(t.rangeBlocked); return; }
      if (!spanClosed && days[d]?.closed) { setMessage(t.closedNotAllowed); return; }
    }
    setTo(date);
  }

  async function save() {
    if (!item) return;
    const startsAt = item.kind === 'service' ? slot?.starts_at : outISO;
    if (!startsAt) return setMessage(item.kind === 'service' ? t.pickDay : item.kind === 'rental' ? t.pickRange : t.pickDay);
    if (item.kind === 'rental' && !backISO) return setMessage(t.pickReturn);
    if (!customer) return setMessage(strings.customers.pick);
    setBusy(true);
    setErrors({});
    setMessage(null);
    const res = await api.POST('/v1/b/{businessId}/bookings', {
      params: { path: { businessId }, header: { 'Idempotency-Key': idempotencyKey } },
      body: {
        customer: { id: customer?.id },
        item_id: item.id,
        quantity: qty,
        starts_at: startsAt,
        ...(item.kind === 'rental' && backISO ? { ends_at: backISO } : {}),
        ...(staffId ? { staff_id: staffId } : {}),
        ...(item.kind === 'product' ? { fulfillment, delivery_address: address, delivery_zone: zone } : {}),
        notes,
      },
    });
    setBusy(false);
    if (res.data) {
      return router.replace({ pathname: '/admin/[businessId]/bookings/[bookingId]', params: { businessId, bookingId: res.data.id } });
    }
    if (res.response.status === 409) { setSlot(null); setTo(null); setRefresh((n) => n + 1); return setMessage(t.taken); }
    setErrors(fieldErrors(res.error));
    setMessage(errorText(res.error, res.response.status));
  }

  if (!business || !items) return <ActivityIndicator style={{ marginTop: 24 }} />;
  const bookingTerm = term(business.vertical, 'booking') ?? '';
  const canSave = !!item && (item.kind === 'service' ? !!slot : item.kind === 'product' ? !!from && !!outISO : !!exact?.available);

  return (
    <ScrollView contentContainerStyle={{ padding: 16, gap: 14, maxWidth: 560, width: '100%', alignSelf: 'center' }}>
      <Stack.Screen options={{ title: t.new(bookingTerm) }} />

      <Text style={{ fontWeight: '600' }}>{term(business.vertical, 'catalog_item')}</Text>
      <View accessibilityRole="radiogroup" style={{ gap: 6 }}>
        {items.map((i) => (
          <Choice key={i.id} selected={i.id === itemId} label={`${i.name} · ${formatMXN(i.price)}`} onPress={() => setItemId(i.id)} />
        ))}
      </View>

      {item && item.kind !== 'service' && (
        <Field label={t.quantity} value={quantity} onChange={setQuantity} keyboard="numeric" error={errors.quantity} />
      )}

      {item && (
        <>
          <Text>{item.kind === 'rental' ? (from && !to ? t.pickReturn : t.pickRange) : t.pickDay}</Text>
          {item.kind === 'rental' && (settings?.rentals.rentals_span_closed_days ?? true) && <Text style={{ opacity: 0.7 }}>{t.closedDayHint}</Text>}
          <Calendar month={month} onMonthChange={setMonth} minMonth={firstOfMonth(today)} from={from} to={item.kind === 'rental' ? to : null}
            onSelect={pick} dayInfo={dayInfo} />
          {daysState !== 'loading' && daysState !== 'ready' && <Text accessibilityRole="alert">{daysState}</Text>}
          {item.kind === 'rental' && daysState === 'ready' && Object.values(days).every((d) => d.remaining === 0) && (
            <Link href={{ pathname: '/admin/[businessId]/catalog/[itemId]/units', params: { businessId, itemId: item.id } }}>
              <Text accessibilityRole="alert" style={{ textDecorationLine: 'underline' }}>{t.noUnits}</Text>
            </Link>
          )}
        </>
      )}

      {item?.kind === 'service' && from && (
        <>
          {slots === null && <ActivityIndicator />}
          {slots?.length === 0 && <Text>{t.noSlots}</Text>}
          {!!slots?.length && (
            <View accessibilityRole="radiogroup" style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 6 }}>
              {slots.map((s) => (
                <Choice key={s.starts_at} selected={slot?.starts_at === s.starts_at} label={hhmm(s.starts_at)} onPress={() => { setSlot(s); setStaffId(null); }} />
              ))}
            </View>
          )}
          {slot && slot.staff_ids.length > 1 && (
            <>
              <Text>{t.attendedBy}</Text>
              <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 6 }}>
                <Choice selected={staffId === null} label={t.anyone} onPress={() => setStaffId(null)} />
                {slot.staff_ids.map((id) => (
                  <Choice key={id} selected={staffId === id} label={staffNames[id] ?? id.slice(0, 6)} onPress={() => setStaffId(id)} />
                ))}
              </View>
            </>
          )}
        </>
      )}

      {item?.kind === 'product' && from && (
        <>
          <Field label={t.deliveryTime} value={time} onChange={setTime} error={errors.starts_at} />
          <View style={{ flexDirection: 'row', gap: 6 }}>
            <Choice selected={fulfillment === 'pickup'} label={t.pickup} onPress={() => setFulfillment('pickup')} />
            <Choice selected={fulfillment === 'delivery'} label={t.delivery} onPress={() => setFulfillment('delivery')} />
          </View>
          {fulfillment === 'delivery' && (
            <>
              <Field label={t.address} value={address} onChange={setAddress} error={errors.delivery_address} />
              <Field label={t.zone} value={zone} onChange={setZone} error={errors.delivery_zone} />
            </>
          )}
        </>
      )}

      {item?.kind === 'rental' && from && (
        <>
          <Field label={t.outTime} value={time} onChange={setTime} />
          {to && <Field label={t.backTime} value={backTime} onChange={setBackTime} error={errors.ends_at} />}
          {to && !exact && !message && <Text>{t.checking}</Text>}
          {exact && <Text>{exact.available ? t.free(exact.free_units?.length ?? 0) : t.notEnough}</Text>}
        </>
      )}

      {item && (
        <>
          <Text style={{ fontWeight: '600' }}>{t.customer}</Text>
          <CustomerPicker businessId={businessId} isOwner={business.role === 'owner'} value={customer} onChange={setCustomer}
            error={errors['customer.id'] ?? errors['customer.name']} />
          <Field label={t.notes} value={notes} onChange={setNotes} />
          {Object.entries(errors).filter(([k]) => k.startsWith('options.')).map(([k, v]) => <Text key={k} accessibilityRole="alert">{v}</Text>)}
          {message && <Text accessibilityRole="alert">{message}</Text>}
          <Button title={busy ? '…' : t.save} onPress={save} disabled={busy || !canSave} />
        </>
      )}
    </ScrollView>
  );
}
