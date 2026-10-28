import type { components } from '@lb/api-client';
import { centavosToInput, kindFor, parsePesos, strings, term, type ItemKind } from '@lb/shared';
import { router, Stack, useLocalSearchParams, usePathname } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Switch, View } from 'react-native';
import { ItemPhotos } from '../../../../../../components/ItemPhotos';
import { UnitsSection } from '../../../../../../components/UnitsSection';
import { Button, Cell, Field, Group, Notice, Screen, Text } from '../../../../../../ui';
import { api } from '../../../../../../lib/api';
import { goBack } from '../../../../../../lib/nav';
import { uploadPhotos, type PickedPhoto } from '../../../../../../lib/photos';
import { errorText, fieldErrors } from '../../../../../../lib/errors';
import { useBusiness } from '../../../../../../lib/useMe';
import { useSettings } from '../../../../../../lib/useSettings';

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
function toFields(f: Form, kind: ItemKind, depositEnabled: boolean): { fields?: Fields; errors?: Record<string, string> } {
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
    // Rentals are priced per day; the late fee comes from the business multiplier (Ajustes), not per tool.
    fields.price_unit = 'day';
    fields.security_deposit = depositEnabled ? money('security_deposit', f.deposit) : 0;
    fields.late_fee_per_day = 0;
  }
  return Object.keys(errors).length ? { errors } : { fields };
}

export default function CatalogItemForm() {
  const pathname = usePathname();
  const { businessId, itemId, error: carriedError } = useLocalSearchParams<{ businessId: string; itemId: string; error?: string }>();
  const { business } = useBusiness(businessId);
  const { settings } = useSettings(businessId);
  const depositEnabled = settings?.rentals.security_deposit_enabled ?? true;
  const isNew = itemId === 'new';
  const [item, setItem] = useState<Item | null>(null);
  const [form, setForm] = useState<Form | null>(isNew ? empty : null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [message, setMessage] = useState<string | null>(carriedError ?? null);
  const [busy, setBusy] = useState(false);
  const [pending, setPending] = useState<PickedPhoto[]>([]); // photos chosen before the item exists
  const [units, setUnits] = useState('1'); // new rental: pieces to create

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
    const { fields, errors: local } = toFields(form!, kind, depositEnabled);
    if (!fields) return setErrors(local!);
    setBusy(true);
    setErrors({});
    setMessage(null);
    const res = isNew
      ? await api.POST('/v1/b/{businessId}/catalog', { params: { path: { businessId } }, body: { ...fields, kind, name: fields.name!, price: fields.price!, ...(kind === 'rental' ? { units: Number(units) || 0 } : {}) } })
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
    goBack(pathname);
  }

  const field = (key: keyof Form, apiKey: string, label: string, numeric = false, hint?: string) => (
    <Field label={label} value={form[key] as string} onChangeText={set(key)} keyboardType={numeric ? 'decimal-pad' : 'default'}
      error={errors[apiKey]} hint={hint} multiline={key === 'description'} />
  );
  const isOwner = business.role === 'owner';

  async function toggleActive(active: boolean) {
    const { data, error, response } = await api.PATCH('/v1/b/{businessId}/catalog/{itemId}', { params: { path: { businessId, itemId } }, body: { active } });
    if (data) setItem(data);
    else setMessage(errorText(error, response.status));
  }

  const title = isNew ? `${t.add} ${term(business.vertical, 'catalog_item')?.toLowerCase()}` : form.name;
  return (
    <Screen>
      <Stack.Screen options={{ title }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 20 }}>{title}</Text>
      <Group label={t.general} padded>
        {field('name', 'name', t.name)}
        {field('description', 'description', t.description)}
        {field('price', 'price', kind === 'rental' ? t.pricePerDay : t.price, true, kind === 'rental' ? t.perDayNote(settings?.rentals.late_fee_multiplier ?? 1) : undefined)}
        {kind === 'service' && (<>{field('duration', 'duration_minutes', t.duration, true)}{field('buffer', 'buffer_minutes', t.buffer, true)}</>)}
        {kind === 'product' && (<>{field('prepHours', 'prep_hours', t.prepHours, true)}{field('dailyCapacity', 'daily_capacity', t.dailyCapacity, true)}</>)}
        {kind === 'rental' && depositEnabled && field('deposit', 'security_deposit', t.deposit, true, t.depositHint)}
        {kind === 'rental' && isNew && (
          <Field label={t.unitsToCreate} value={units} onChangeText={setUnits} keyboardType="numeric" error={errors.units} hint={t.unitsToCreateHint} />
        )}
      </Group>
      <Group label={strings.photos.title} padded footer={t.photosHint}>
        {item ? (
          <ItemPhotos businessId={businessId} itemId={item.id} images={item.images ?? []} reload={reloadItem} editable={isOwner} />
        ) : (
          <ItemPhotos businessId={businessId} pending={pending} onPendingChange={setPending} editable={isOwner} />
        )}
      </Group>
      {item && isOwner && (
        <Group footer={t.activeHint}>
          <Cell label={t.activeLabel} accessory={<Switch accessibilityLabel={t.activeLabel} value={item.active} onValueChange={toggleActive} />} />
        </Group>
      )}
      {message && <View style={{ marginBottom: 12 }}><Notice tone="warning">{message}</Notice></View>}
      {isOwner && <Button variant="primary" busy={busy} onPress={save} style={{ marginBottom: 28 }}>{t.save}</Button>}
      {item && kind === 'rental' && <UnitsSection businessId={businessId} itemId={item.id} isOwner={isOwner} />}
    </Screen>
  );
}
