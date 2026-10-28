import { Ionicons } from '@expo/vector-icons';
import type { components } from '@lb/api-client';
import { splitE164, strings, toE164, type CountryCode } from '@lb/shared';
import { ZoomableImage } from './ZoomableImage';
import { useEffect, useState } from 'react';
import { Switch, View } from 'react-native';
import { Button, Cell, Field, Group, Text, useToast, useUI } from '../ui';
import { api } from '../lib/api';
import { errorText, fieldErrors } from '../lib/errors';
import { pickPhotos, uploadCustomerDoc } from '../lib/photos';
import { useTexts } from '../lib/useTexts';
import { PhoneInput } from './PhoneInput';

export type Customer = components['schemas']['Customer'];
type DocType = components['schemas']['DocType'];
const t = strings.customers;

/**
 * Create or edit a customer profile. Rental businesses require address and guarantor (the API checks it too).
 * Documents can be added once the customer exists.
 */
export function CustomerForm({ businessId, customer, isOwner, onSaved, onExisting }: {
  businessId: string; customer?: Customer; isOwner: boolean;
  onSaved: (c: Customer) => void; onExisting?: (customerId: string) => void;
}) {
  // Guarantor, documents and "trusted" are rental things (the API requires them only there).
  const { isRental } = useTexts();
  const p = splitE164(customer?.phone);
  const g = splitE164(customer?.guarantor_phone);
  const [f, setF] = useState({
    name: customer?.name ?? '', code: p.code, national: p.national, email: customer?.email ?? '', address: customer?.address ?? '',
    guarantorName: customer?.guarantor_name ?? '', gCode: g.code, gNational: g.national, trusted: customer?.trusted ?? false, notes: customer?.notes ?? '',
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const toast = useToast();
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
    setExisting(null);
    const body = {
      name: f.name, phone: phone!, email: f.email, address: f.address, guarantor_name: f.guarantorName, guarantor_phone: gPhone ?? '', notes: f.notes,
      ...(isOwner && isRental ? { trusted: f.trusted } : {}),
    };
    const res = customer
      ? await api.PATCH('/v1/b/{businessId}/customers/{customerId}/', { params: { path: { businessId, customerId: customer.id } }, body })
      : await api.POST('/v1/b/{businessId}/customers', { params: { path: { businessId } }, body });
    setBusy(false);
    if (res.data) { toast(t.saved); return onSaved(res.data); }
    const fe = fieldErrors(res.error);
    setErrors(fe);
    if (res.response.status === 409) setExisting(fe.customer_id ?? null);
    toast(errorText(res.error, res.response.status), 'error');
  }

  return (
    <View>
      <Group label={t.contact} padded>
        <Field label={t.name} value={f.name} onChangeText={(name) => set({ name })} error={errors.name} autoComplete="name" />
        <PhoneInput label={t.phone} code={f.code} national={f.national} onChange={(code, national) => set({ code, national })} error={errors.phone} />
        <Field label={t.address} value={f.address} onChangeText={(address) => set({ address })} error={errors.address} multiline />
        <Field label={t.email} value={f.email} onChangeText={(email) => set({ email })} error={errors.email} autoCapitalize="none" keyboardType="email-address" />
      </Group>
      {isRental && <Group label={t.guarantor} padded footer={t.guarantorHint}>
        <Field label={t.guarantorName} value={f.guarantorName} onChangeText={(guarantorName) => set({ guarantorName })} error={errors.guarantor_name} />
        <PhoneInput label={t.guarantorPhone} code={f.gCode} national={f.gNational} onChange={(gCode: CountryCode, gNational) => set({ gCode, gNational })} error={errors.guarantor_phone} />
      </Group>}
      <Group padded>
        <Field label={t.notes} value={f.notes} onChangeText={(notes) => set({ notes })} multiline />
      </Group>
      {isOwner && isRental && (
        <Group footer={t.trustedHint}>
          <Cell label={t.trustedShort} accessory={<Switch accessibilityLabel={t.trusted} value={f.trusted} onValueChange={(trusted) => set({ trusted })} />} />
        </Group>
      )}
      {existing && onExisting && <Button onPress={() => onExisting(existing)} style={{ marginBottom: 8 }}>{t.openExisting}</Button>}
      <Button variant="primary" busy={busy} onPress={save}>{t.save}</Button>
    </View>
  );
}

/** The customer's documents on file: ID, proof of address, guarantor ID. */
export function CustomerDocuments({ businessId, customer, isOwner, onChange }: {
  businessId: string; customer: Customer; isOwner: boolean; onChange: (c: Customer) => void;
}) {
  const ui = useUI();
  const toast = useToast();
  const [busy, setBusy] = useState<DocType | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [urls, setUrls] = useState<Record<string, string>>({});
  const path = { businessId, customerId: customer.id };

  async function add(docType: DocType) {
    setBusy(docType);
    setMessage(null);
    const picked = await pickPhotos(4);
    if (picked === 'cancelled') return setBusy(null);
    if (typeof picked === 'string') { setBusy(null); setMessage(null); return toast(picked, 'error'); }
    const keys: string[] = [];
    for (const [i, photo] of picked.photos.entries()) {
      setMessage(strings.photos.progress(i + 1, picked.photos.length));
      const up = await uploadCustomerDoc(businessId, customer.id, docType, photo);
      if ('error' in up) { setBusy(null); setMessage(null); return toast(up.error, 'error'); }
      keys.push(up.key);
    }
    const { data, error, response } = await api.POST('/v1/b/{businessId}/customers/{customerId}/documents', { params: { path }, body: { doc_type: docType, keys } });
    setBusy(null);
    setMessage(null);
    if (data) { toast(t.docSaved); onChange(data); } else toast(errorText(error, response.status), 'error');
  }

  // Load the thumbnails right away (short-lived signed URLs from the private bucket).
  useEffect(() => {
    for (const d of customer.documents) {
      if (urls[d.key]) continue;
      api.GET('/v1/b/{businessId}/customers/{customerId}/files', { params: { path: { businessId, customerId: customer.id }, query: { key: d.key } } })
        .then(({ data }) => data && setUrls((u) => ({ ...u, [d.key]: data.url })));
    }
  }, [businessId, customer]); // eslint-disable-line react-hooks/exhaustive-deps

  async function remove(docId: string) {
    const { response } = await api.DELETE('/v1/b/{businessId}/customers/{customerId}/documents/{docId}', { params: { path: { ...path, docId } } });
    if (response.ok) onChange({ ...customer, documents: customer.documents.filter((d) => d.id !== docId) });
  }

  return (
    <Group label={t.documents} footer={message ?? t.documentsHint}>
      {(['ine', 'proof_of_address', 'guarantor_id'] as const).map((type) => {
        const docs = customer.documents.filter((d) => d.doc_type === type);
        return (
          <View key={type} style={{ padding: 16, gap: 10 }}>
            <View style={{ flexDirection: 'row', alignItems: 'center', gap: 8 }}>
              <Ionicons name={docs.length ? 'checkmark-circle' : 'ellipse-outline'} size={22} color={docs.length ? ui.success : ui.labelSecondary} />
              <Text style={{ flex: 1 }}>{t.docTypes[type]}</Text>
              <Text variant="footnote" color="labelSecondary">{docs.length ? t.hasDoc(docs.length) : t.noDoc}</Text>
            </View>
            {docs.length > 0 && (
              <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
                {docs.map((d) => (
                  <View key={d.id} style={{ gap: 4, alignItems: 'center' }}>
                    {urls[d.key]
                      ? <ZoomableImage source={urls[d.key]} style={{ width: 88, height: 88, borderRadius: 10 }} contentFit="cover" accessibilityLabel={t.docTypes[type]} />
                      : <View style={{ width: 88, height: 88, borderRadius: 10, backgroundColor: ui.fill }} />}
                    {isOwner && <Button size="sm" variant="plain" onPress={() => remove(d.id)}>{t.removeDoc}</Button>}
                  </View>
                ))}
              </View>
            )}
            <Button size="sm" icon="camera" busy={busy === type} disabled={busy !== null} onPress={() => add(type)}>{t.addDoc}</Button>
          </View>
        );
      })}
    </Group>
  );
}
