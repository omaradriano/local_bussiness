import type { components } from '@lb/api-client';
import { splitE164, strings, toE164, type CountryCode } from '@lb/shared';
import { ZoomableImage } from './ZoomableImage';
import { useState } from 'react';
import { Pressable, Switch, Text, TextInput, View } from 'react-native';
import { Button } from '../ui';
import { api } from '../lib/api';
import { errorText, fieldErrors } from '../lib/errors';
import { pickPhotos, uploadCustomerDoc } from '../lib/photos';
import { PhoneInput } from './PhoneInput';

export type Customer = components['schemas']['Customer'];
type DocType = components['schemas']['DocType'];
const t = strings.customers;

function Field({ label, value, onChange, error, multiline }: { label: string; value: string; onChange: (v: string) => void; error?: string; multiline?: boolean }) {
  return (
    <View style={{ gap: 4 }}>
      <Text>{label}</Text>
      <TextInput accessibilityLabel={label} value={value} onChangeText={onChange} multiline={multiline}
        style={{ borderWidth: 1, borderRadius: 6, padding: 10, minHeight: multiline ? 60 : undefined }} />
      {error && <Text accessibilityRole="alert">{error}</Text>}
    </View>
  );
}

/**
 * Create or edit a customer profile. Rental businesses require address and guarantor (the API checks it too).
 * Documents can be added once the customer exists.
 */
export function CustomerForm({ businessId, customer, isOwner, onSaved, onExisting }: {
  businessId: string; customer?: Customer; isOwner: boolean;
  onSaved: (c: Customer) => void; onExisting?: (customerId: string) => void;
}) {
  const p = splitE164(customer?.phone);
  const g = splitE164(customer?.guarantor_phone);
  const [f, setF] = useState({
    name: customer?.name ?? '', code: p.code, national: p.national, email: customer?.email ?? '', address: customer?.address ?? '',
    guarantorName: customer?.guarantor_name ?? '', gCode: g.code, gNational: g.national, trusted: customer?.trusted ?? false, notes: customer?.notes ?? '',
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [message, setMessage] = useState<string | null>(null);
  const [existing, setExisting] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const set = (patch: Partial<typeof f>) => setF({ ...f, ...patch });

  async function save() {
    const phone = toE164(f.code, f.national);
    const gPhone = f.gNational ? toE164(f.gCode, f.gNational) : '';
    const local: Record<string, string> = {};
    if (!phone) local.phone = t.invalidPhone;
    if (gPhone === null) local.guarantor_phone = t.invalidPhone;
    if (Object.keys(local).length) return setErrors(local);
    setBusy(true);
    setErrors({});
    setMessage(null);
    setExisting(null);
    const body = {
      name: f.name, phone: phone!, email: f.email, address: f.address, guarantor_name: f.guarantorName, guarantor_phone: gPhone ?? '', notes: f.notes,
      ...(isOwner ? { trusted: f.trusted } : {}),
    };
    const res = customer
      ? await api.PATCH('/v1/b/{businessId}/customers/{customerId}/', { params: { path: { businessId, customerId: customer.id } }, body })
      : await api.POST('/v1/b/{businessId}/customers', { params: { path: { businessId } }, body });
    setBusy(false);
    if (res.data) { setMessage(t.saved); return onSaved(res.data); }
    const fe = fieldErrors(res.error);
    setErrors(fe);
    if (res.response.status === 409) setExisting(fe.customer_id ?? null);
    setMessage(errorText(res.error, res.response.status));
  }

  return (
    <View style={{ gap: 12 }}>
      <Field label={t.name} value={f.name} onChange={(name) => set({ name })} error={errors.name} />
      <PhoneInput label={t.phone} code={f.code} national={f.national} onChange={(code, national) => set({ code, national })} error={errors.phone} />
      <Field label={t.address} value={f.address} onChange={(address) => set({ address })} error={errors.address} multiline />
      <Text style={{ fontWeight: '600' }}>{t.guarantor}</Text>
      <Field label={t.guarantorName} value={f.guarantorName} onChange={(guarantorName) => set({ guarantorName })} error={errors.guarantor_name} />
      <PhoneInput label={t.guarantorPhone} code={f.gCode} national={f.gNational} onChange={(gCode: CountryCode, gNational) => set({ gCode, gNational })} error={errors.guarantor_phone} />
      <Field label={t.email} value={f.email} onChange={(email) => set({ email })} error={errors.email} />
      <Field label={t.notes} value={f.notes} onChange={(notes) => set({ notes })} multiline />
      {isOwner && (
        <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 12 }}>
          <Text style={{ flexShrink: 1 }}>{t.trusted}</Text>
          <Switch accessibilityLabel={t.trusted} value={f.trusted} onValueChange={(trusted) => set({ trusted })} />
        </View>
      )}
      {message && <Text accessibilityRole="alert">{message}</Text>}
      {existing && onExisting && <Button onPress={() => onExisting(existing)}>{t.openExisting}</Button>}
      <Button onPress={save} disabled={busy}>{busy ? '…' : t.save}</Button>
    </View>
  );
}

/** The customer's documents on file: ID, proof of address, guarantor ID. */
export function CustomerDocuments({ businessId, customer, isOwner, onChange }: {
  businessId: string; customer: Customer; isOwner: boolean; onChange: (c: Customer) => void;
}) {
  const [busy, setBusy] = useState<DocType | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [urls, setUrls] = useState<Record<string, string>>({});
  const path = { businessId, customerId: customer.id };

  async function add(docType: DocType) {
    setBusy(docType);
    setMessage(null);
    const picked = await pickPhotos(4);
    if (picked === 'cancelled') return setBusy(null);
    if (typeof picked === 'string') { setBusy(null); return setMessage(picked); }
    const keys: string[] = [];
    for (const [i, photo] of picked.photos.entries()) {
      setMessage(strings.photos.progress(i + 1, picked.photos.length));
      const up = await uploadCustomerDoc(businessId, customer.id, docType, photo);
      if ('error' in up) { setBusy(null); return setMessage(up.error); }
      keys.push(up.key);
    }
    const { data, error, response } = await api.POST('/v1/b/{businessId}/customers/{customerId}/documents', { params: { path }, body: { doc_type: docType, keys } });
    setBusy(null);
    setMessage(data ? null : errorText(error, response.status));
    if (data) onChange(data);
  }

  async function show(key: string) {
    const { data } = await api.GET('/v1/b/{businessId}/customers/{customerId}/files', { params: { path, query: { key } } });
    if (data) setUrls((u) => ({ ...u, [key]: data.url }));
  }

  async function remove(docId: string) {
    const { response } = await api.DELETE('/v1/b/{businessId}/customers/{customerId}/documents/{docId}', { params: { path: { ...path, docId } } });
    if (response.ok) onChange({ ...customer, documents: customer.documents.filter((d) => d.id !== docId) });
  }

  return (
    <View style={{ gap: 10 }}>
      <Text style={{ fontWeight: '600' }}>{t.documents}</Text>
      {(['ine', 'proof_of_address', 'guarantor_id'] as const).map((type) => {
        const docs = customer.documents.filter((d) => d.doc_type === type);
        return (
          <View key={type} style={{ borderRadius: 12, padding: 12, backgroundColor: 'rgba(118,118,128,0.12)', gap: 6 }}>
            <Text>{docs.length ? '✓' : '○'} {t.docTypes[type]} · {docs.length ? t.hasDoc(docs.length) : t.noDoc}</Text>
            <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
              {docs.map((d) => (
                <View key={d.id} style={{ gap: 4, alignItems: 'center' }}>
                  {urls[d.key] ? (
                    <ZoomableImage source={urls[d.key]} style={{ width: 96, height: 96, borderRadius: 6 }} contentFit="cover" accessibilityLabel={t.docTypes[type]} />
                  ) : (
                    <Pressable accessibilityRole="button" onPress={() => show(d.key)}><Text style={{ textDecorationLine: 'underline' }}>{t.seeDoc}</Text></Pressable>
                  )}
                  {isOwner && <Pressable accessibilityRole="button" onPress={() => remove(d.id)}><Text style={{ textDecorationLine: 'underline' }}>{t.removeDoc}</Text></Pressable>}
                </View>
              ))}
            </View>
            <Button onPress={() => add(type)} disabled={busy !== null}>{busy === type ? '…' : t.addDoc}</Button>
          </View>
        );
      })}
      {message && <Text accessibilityRole="alert">{message}</Text>}
    </View>
  );
}
