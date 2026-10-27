import type { components } from '@lb/api-client';
import { formatMXN, hhmm, localToISO, strings, term, todayISO } from '@lb/shared';
import { router, Stack, useLocalSearchParams } from 'expo-router';
import { useEffect, useMemo, useState } from 'react';
import { ActivityIndicator, Button, Pressable, ScrollView, Text, TextInput, View } from 'react-native';
import { api } from '../../../../../lib/api';
import { errorText, fieldErrors } from '../../../../../lib/errors';
import { useBusiness } from '../../../../../lib/useMe';

type Item = components['schemas']['CatalogItem'];
type Availability = components['schemas']['Availability'];
type Slot = NonNullable<Availability['slots']>[number];
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
  const [items, setItems] = useState<Item[] | null>(null);
  const [staffNames, setStaffNames] = useState<Record<string, string>>({});
  const [itemId, setItemId] = useState<string | null>(null);
  const item = items?.find((i) => i.id === itemId) ?? null;

  // Form state (kept as typed; converted on save).
  const [date, setDate] = useState(todayISO());
  const [time, setTime] = useState('12:00');
  const [backDate, setBackDate] = useState(todayISO());
  const [backTime, setBackTime] = useState('12:00');
  const [quantity, setQuantity] = useState('1');
  const [slot, setSlot] = useState<Slot | null>(null);
  const [staffId, setStaffId] = useState<string | null>(null);
  const [fulfillment, setFulfillment] = useState<'pickup' | 'delivery'>('pickup');
  const [address, setAddress] = useState('');
  const [zone, setZone] = useState('');
  const [phone, setPhone] = useState('');
  const [name, setName] = useState('');
  const [notes, setNotes] = useState('');

  const [avail, setAvail] = useState<Availability | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  // One key per form: a double tap or retry never creates two bookings.
  const idempotencyKey = useMemo(() => `adm-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`, []);

  useEffect(() => {
    const path = { businessId };
    api.GET('/v1/b/{businessId}/catalog', { params: { path } }).then(({ data }) => setItems(data?.items.filter((i) => i.active) ?? []));
    api.GET('/v1/b/{businessId}/staff', { params: { path } }).then(({ data }) =>
      setStaffNames(Object.fromEntries((data?.staff ?? []).map((s) => [s.id, s.name]))));
  }, [businessId]);

  // Any change to what's being asked invalidates the previous availability answer.
  useEffect(() => { setAvail(null); setSlot(null); setStaffId(null); }, [itemId, date, time, backDate, backTime, quantity]);

  async function check() {
    if (!item) return;
    setMessage(null);
    const qty = Number(quantity) || 1;
    let query: Record<string, string | number> = { item_id: item.id, quantity: qty };
    if (item.kind === 'service') query = { ...query, date };
    else if (item.kind === 'product') query = { ...query, from: date, to: date };
    else {
      const from = localToISO(date, time);
      const to = localToISO(backDate, backTime);
      if (!from || !to) return setMessage(t.invalidDate);
      query = { ...query, from, to };
    }
    const { data, error, response } = await api.GET('/v1/b/{businessId}/availability', {
      params: { path: { businessId }, query: query as never },
    });
    if (data) setAvail(data);
    else setMessage(errorText(error, response.status));
  }

  async function save() {
    if (!item) return;
    const startsAt = item.kind === 'service' ? slot?.starts_at : localToISO(date, time);
    if (!startsAt) return setMessage(item.kind === 'service' ? t.seeSlots : t.invalidDate);
    const endsAt = item.kind === 'rental' ? localToISO(backDate, backTime) : null;
    if (item.kind === 'rental' && !endsAt) return setMessage(t.invalidDate);
    setBusy(true);
    setErrors({});
    setMessage(null);
    const res = await api.POST('/v1/b/{businessId}/bookings', {
      params: { path: { businessId }, header: { 'Idempotency-Key': idempotencyKey } },
      body: {
        customer: { name, phone },
        item_id: item.id,
        quantity: Number(quantity) || 1,
        starts_at: startsAt,
        ...(endsAt ? { ends_at: endsAt } : {}),
        ...(staffId ? { staff_id: staffId } : {}),
        ...(item.kind === 'product' ? { fulfillment, delivery_address: address, delivery_zone: zone } : {}),
        notes,
      },
    });
    setBusy(false);
    if (res.data) {
      return router.replace({ pathname: '/admin/[businessId]/bookings/[bookingId]', params: { businessId, bookingId: res.data.id } });
    }
    if (res.response.status === 409) { setAvail(null); setSlot(null); return setMessage(t.taken); }
    setErrors(fieldErrors(res.error));
    setMessage(errorText(res.error, res.response.status));
  }

  if (!business || !items) return <ActivityIndicator style={{ marginTop: 24 }} />;
  const bookingTerm = term(business.vertical, 'booking') ?? '';

  return (
    <ScrollView contentContainerStyle={{ padding: 16, gap: 14, maxWidth: 560, width: '100%', alignSelf: 'center' }}>
      <Stack.Screen options={{ title: t.new(bookingTerm) }} />

      <Text style={{ fontWeight: '600' }}>{term(business.vertical, 'catalog_item')}</Text>
      <View accessibilityRole="radiogroup" style={{ gap: 6 }}>
        {items.map((i) => (
          <Choice key={i.id} selected={i.id === itemId} label={`${i.name} · ${formatMXN(i.price)}`} onPress={() => setItemId(i.id)} />
        ))}
      </View>

      {item?.kind === 'service' && (
        <>
          <Field label={t.date} value={date} onChange={setDate} />
          <Button title={t.seeSlots} onPress={check} />
          {avail && !avail.slots?.length && <Text>{t.noSlots}</Text>}
          {!!avail?.slots?.length && (
            <View accessibilityRole="radiogroup" style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 6 }}>
              {avail.slots.map((s) => (
                <Choice key={s.starts_at} selected={slot?.starts_at === s.starts_at} label={hhmm(s.starts_at)}
                  onPress={() => { setSlot(s); setStaffId(null); }} />
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

      {item?.kind === 'product' && (
        <>
          <Field label={t.deliveryDate} value={date} onChange={setDate} error={errors.starts_at} />
          <Field label={t.deliveryTime} value={time} onChange={setTime} />
          <Field label={t.quantity} value={quantity} onChange={setQuantity} keyboard="numeric" error={errors.quantity} />
          <Button title={t.check} onPress={check} />
          {avail?.days?.[0] && <Text>{avail.days[0].available ? t.capacity(avail.days[0].remaining) : t.noCapacity}</Text>}
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

      {item?.kind === 'rental' && (
        <>
          <Field label={t.outDate} value={date} onChange={setDate} />
          <Field label={t.outTime} value={time} onChange={setTime} />
          <Field label={t.backDate} value={backDate} onChange={setBackDate} error={errors.ends_at} />
          <Field label={t.backTime} value={backTime} onChange={setBackTime} />
          <Field label={t.quantity} value={quantity} onChange={setQuantity} keyboard="numeric" error={errors.quantity} />
          <Button title={t.check} onPress={check} />
          {avail && <Text>{avail.available ? t.free(avail.free_units?.length ?? 0) : t.notEnough}</Text>}
        </>
      )}

      {item && (
        <>
          <Text style={{ fontWeight: '600' }}>{t.customer}</Text>
          <Field label={t.phone} value={phone} onChange={setPhone} keyboard="phone-pad" error={errors['customer.phone']} />
          <Field label={t.name} value={name} onChange={setName} error={errors['customer.name']} />
          <Field label={t.notes} value={notes} onChange={setNotes} />
          {Object.entries(errors).filter(([k]) => k.startsWith('options.')).map(([k, v]) => <Text key={k} accessibilityRole="alert">{v}</Text>)}
          {message && <Text accessibilityRole="alert">{message}</Text>}
          <Button title={busy ? '…' : t.save} onPress={save} disabled={busy} />
        </>
      )}
    </ScrollView>
  );
}
