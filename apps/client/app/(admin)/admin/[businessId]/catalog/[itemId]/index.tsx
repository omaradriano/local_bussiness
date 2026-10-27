import type { components } from '@lb/api-client';
import { centavosToInput, kindFor, parsePesos, strings, term, type ItemKind } from '@lb/shared';
import { router, Stack, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Button, Pressable, ScrollView, Text, TextInput, View } from 'react-native';
import { ItemPhotos } from '../../../../../../components/ItemPhotos';
import { api } from '../../../../../../lib/api';
import { uploadPhotos, type PickedPhoto } from '../../../../../../lib/photos';
import { errorText, fieldErrors } from '../../../../../../lib/errors';
import { useBusiness } from '../../../../../../lib/useMe';

type Item = components['schemas']['CatalogItem'];
type Fields = components['schemas']['CatalogItemFields'];
type PriceUnit = components['schemas']['PriceUnit'];
const t = strings.catalog;

// Form keeps what the person typed; it becomes numbers (centavos) only on save.
type Form = {
  name: string; description: string; price: string;
  duration: string; buffer: string; prepHours: string; dailyCapacity: string;
  priceUnit: PriceUnit; deposit: string; lateFee: string;
};

const empty: Form = { name: '', description: '', price: '', duration: '60', buffer: '0', prepHours: '24', dailyCapacity: '', priceUnit: 'day', deposit: '', lateFee: '' };

function fromItem(it: Item): Form {
  return {
    name: it.name, description: it.description, price: centavosToInput(it.price),
    duration: String(it.duration_minutes ?? ''), buffer: String(it.buffer_minutes ?? 0),
    prepHours: String(it.prep_hours ?? ''), dailyCapacity: it.daily_capacity ? String(it.daily_capacity) : '',
    priceUnit: it.price_unit ?? 'day',
    deposit: it.security_deposit != null ? centavosToInput(it.security_deposit) : '',
    lateFee: it.late_fee_per_day != null ? centavosToInput(it.late_fee_per_day) : '',
  };
}

/** Form -> API fields for the kind, or field errors for what can't be parsed. */
function toFields(f: Form, kind: ItemKind): { fields?: Fields; errors?: Record<string, string> } {
  const errors: Record<string, string> = {};
  const money = (key: string, v: string) => { const c = parsePesos(v); if (c === null) errors[key] = t.invalidMoney; return c ?? 0; };
  const int = (key: string, v: string) => { if (!/^\d+$/.test(v.trim())) errors[key] = t.invalidNumber; return Number(v); };

  const fields: Fields = { name: f.name, description: f.description, price: money('price', f.price) };
  if (kind === 'service') {
    fields.duration_minutes = int('duration_minutes', f.duration);
    fields.buffer_minutes = int('buffer_minutes', f.buffer || '0');
  } else if (kind === 'product') {
    fields.prep_hours = int('prep_hours', f.prepHours);
    fields.daily_capacity = f.dailyCapacity.trim() ? int('daily_capacity', f.dailyCapacity) : null;
  } else {
    fields.price_unit = f.priceUnit;
    fields.security_deposit = money('security_deposit', f.deposit);
    fields.late_fee_per_day = money('late_fee_per_day', f.lateFee);
  }
  return Object.keys(errors).length ? { errors } : { fields };
}

export default function CatalogItemForm() {
  const { businessId, itemId, error: carriedError } = useLocalSearchParams<{ businessId: string; itemId: string; error?: string }>();
  const { business } = useBusiness(businessId);
  const isNew = itemId === 'new';
  const [item, setItem] = useState<Item | null>(null);
  const [form, setForm] = useState<Form | null>(isNew ? empty : null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [message, setMessage] = useState<string | null>(carriedError ?? null);
  const [busy, setBusy] = useState(false);
  const [pending, setPending] = useState<PickedPhoto[]>([]); // photos chosen before the item exists

  useEffect(() => {
    if (isNew) return;
    api.GET('/v1/b/{businessId}/catalog', { params: { path: { businessId } } }).then(({ data, error, response }) => {
      const found = data?.items.find((x) => x.id === itemId);
      if (found) { setItem(found); setForm(fromItem(found)); }
      else setMessage(data ? strings.common.notFound : errorText(error, response.status));
    });
  }, [businessId, itemId, isNew]);

  // Refresh only the item (its photos), never the form, so typing isn't lost while photos process.
  const reloadItem = useCallback(async () => {
    const { data } = await api.GET('/v1/b/{businessId}/catalog', { params: { path: { businessId } } });
    const found = data?.items.find((x) => x.id === itemId);
    if (found) setItem(found);
  }, [businessId, itemId]);

  if (!business || !form) {
    return message ? <Text style={{ padding: 16 }}>{message}</Text> : <ActivityIndicator style={{ marginTop: 24 }} />;
  }
  const kind: ItemKind = item?.kind ?? kindFor(business.vertical);
  const set = (k: keyof Form) => (v: string) => setForm({ ...form, [k]: v });

  async function save() {
    const { fields, errors: local } = toFields(form!, kind);
    if (!fields) return setErrors(local!);
    setBusy(true);
    setErrors({});
    setMessage(null);
    const res = isNew
      ? await api.POST('/v1/b/{businessId}/catalog', { params: { path: { businessId } }, body: { ...fields, kind, name: fields.name!, price: fields.price! } })
      : await api.PATCH('/v1/b/{businessId}/catalog/{itemId}', { params: { path: { businessId, itemId } }, body: fields });
    if (!res.data) {
      setBusy(false);
      setErrors(fieldErrors(res.error));
      return setMessage(errorText(res.error, res.response.status));
    }
    if (isNew && pending.length) {
      setMessage(strings.photos.uploading);
      const err = await uploadPhotos(businessId, res.data.id, pending);
      if (err) {
        // The item exists now: continue on its edit screen so the photo can be retried.
        setBusy(false);
        return router.replace({ pathname: '/admin/[businessId]/catalog/[itemId]', params: { businessId, itemId: res.data.id, error: err } });
      }
    }
    setBusy(false);
    router.back();
  }

  const field = (key: keyof Form, apiKey: string, label: string, numeric = false) => (
    <View style={{ gap: 4 }}>
      <Text>{label}</Text>
      <TextInput
        accessibilityLabel={label}
        value={form[key] as string}
        onChangeText={set(key)}
        keyboardType={numeric ? 'decimal-pad' : 'default'}
        style={{ borderWidth: 1, borderRadius: 6, padding: 10 }}
      />
      {errors[apiKey] && <Text accessibilityRole="alert">{errors[apiKey]}</Text>}
    </View>
  );

  return (
    <ScrollView contentContainerStyle={{ padding: 16, gap: 12, maxWidth: 480, width: '100%', alignSelf: 'center' }}>
      <Stack.Screen options={{ title: isNew ? `${t.add} ${term(business.vertical, 'catalog_item')?.toLowerCase()}` : form.name }} />
      {field('name', 'name', t.name)}
      {field('description', 'description', t.description)}
      {field('price', 'price', t.price, true)}
      {kind === 'service' && (<>{field('duration', 'duration_minutes', t.duration, true)}{field('buffer', 'buffer_minutes', t.buffer, true)}</>)}
      {kind === 'product' && (<>{field('prepHours', 'prep_hours', t.prepHours, true)}{field('dailyCapacity', 'daily_capacity', t.dailyCapacity, true)}</>)}
      {kind === 'rental' && (
        <>
          <Text>{t.priceUnit}</Text>
          <View accessibilityRole="radiogroup" style={{ flexDirection: 'row', gap: 8 }}>
            {(['day', 'weekend', 'week'] as const).map((u) => (
              <Pressable key={u} accessibilityRole="radio" accessibilityState={{ checked: form.priceUnit === u }}
                onPress={() => setForm({ ...form, priceUnit: u })}
                style={{ borderWidth: form.priceUnit === u ? 2 : 1, borderRadius: 6, padding: 8 }}>
                <Text>{t.priceUnits[u]}</Text>
              </Pressable>
            ))}
          </View>
          {field('deposit', 'security_deposit', t.deposit, true)}
          {field('lateFee', 'late_fee_per_day', t.lateFee, true)}
        </>
      )}
      {item ? (
        <ItemPhotos businessId={businessId} itemId={item.id} images={item.images ?? []} reload={reloadItem} editable={business.role === 'owner'} />
      ) : (
        <ItemPhotos businessId={businessId} pending={pending} onPendingChange={setPending} editable={business.role === 'owner'} />
      )}
      {message && <Text accessibilityRole="alert">{message}</Text>}
      <Button title={t.save} onPress={save} disabled={busy} />
    </ScrollView>
  );
}
