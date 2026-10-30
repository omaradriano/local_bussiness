import type { components } from '@lb/api-client';
import { centavosToInput, engineFor, formatMXN, parsePesos, strings } from '@lb/shared';
import { useEffect, useState } from 'react';
import { Pressable, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { api } from '../lib/api';
import { errorText, fieldErrors } from '../lib/errors';
import { pickPhotos, uploadPaymentEvidence, type PickedPhoto } from '../lib/photos';
import { Button, Chip, Chips, Field, Segmented, Text, radius, useToast, useUI } from '../ui';
import { ZoomableImage } from './ZoomableImage';

type Booking = components['schemas']['Booking'];
export type PaymentType = components['schemas']['PaymentType'];
type Method = 'cash' | 'transfer' | 'card';
const t = strings.bookings;

/**
 * Payment types that make sense now (the API enforces the same rules): the deposit until the tool
 * comes back, late/damage fees only after it's back, a refund only while a deposit is held.
 */
export function paymentTypesFor(b: Booking, security: number): PaymentType[] {
  if (!engineFor(b.kind).custody) return ['advance', 'balance'];
  const back = b.status === 'returned' || b.status === 'completed';
  const out: PaymentType[] = [];
  if (b.status === 'pending_deposit' || b.status === 'confirmed') out.push('advance');
  out.push('balance');
  if (!back && security > 0) out.push('security_deposit');
  if (back) out.push('late_fee', 'damage_fee');
  if (b.balance.deposit_held > 0 && back) out.push('refund');
  return out;
}

/** Register a payment: type (only the ones that apply now), method, amount and, for transfer/card, a photo of the proof. */
export function PaymentForm({ businessId, booking, types, suggested, onDone }: {
  businessId: string; booking: Booking; types: PaymentType[]; suggested: { type: PaymentType; cents: number }; onDone: (b: Booking) => void;
}) {
  const ui = useUI();
  const toast = useToast();
  const [type, setType] = useState<PaymentType>(types.includes(suggested.type) ? suggested.type : types[0]);
  const [method, setMethod] = useState<Method>('cash');
  const [amount, setAmount] = useState(suggested.cents > 0 ? centavosToInput(suggested.cents) : '');
  const [evidence, setEvidence] = useState<PickedPhoto[]>([]);
  const [error, setError] = useState<string | undefined>();
  const [busy, setBusy] = useState(false);

  async function pickEvidence() {
    const picked = await pickPhotos(4 - evidence.length);
    if (picked === 'cancelled') return;
    if (typeof picked === 'string') return toast(picked, 'error');
    setEvidence([...evidence, ...picked.photos]);
  }

  async function save() {
    const cents = parsePesos(amount);
    if (cents === null || cents <= 0) return setError(strings.catalog.invalidMoney);
    setBusy(true);
    setError(undefined);
    const keys: string[] = [];
    if (method !== 'cash') {
      for (const photo of evidence) {
        const up = await uploadPaymentEvidence(businessId, booking.id, photo);
        if ('error' in up) { setBusy(false); return toast(up.error, 'error'); }
        keys.push(up.key);
      }
    }
    const { data, error: err, response } = await api.POST('/v1/b/{businessId}/bookings/{bookingId}/payments', {
      params: { path: { businessId, bookingId: booking.id } }, body: { type, method, amount: cents, evidence: keys },
    });
    setBusy(false);
    if (data) { toast(t.paymentSaved); setEvidence([]); return onDone(data); }
    setError(fieldErrors(err).amount);
    toast(errorText(err, response.status), 'error');
  }

  return (
    <View style={{ gap: 12 }}>
      <Chips>
        {types.map((pt) => <Chip key={pt} selected={type === pt} onPress={() => setType(pt)}>{t.paymentTypes[pt]}</Chip>)}
      </Chips>
      <Segmented options={(['cash', 'transfer', 'card'] as Method[]).map((m) => ({ value: m, label: t.methods[m] }))} value={method} onChange={setMethod} />
      <Field label={t.amount} value={amount} onChangeText={setAmount} keyboardType="decimal-pad" error={error} />
      {method !== 'cash' && (
        <View style={{ gap: 8 }}>
          <Text variant="footnote" color="labelSecondary">{t.evidenceHint[method]}</Text>
          {evidence.length > 0 && (
            <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
              {evidence.map((p, i) => (
                <View key={p.uri}>
                  <ZoomableImage source={p.uri} style={{ width: 72, height: 72, borderRadius: 10 }} contentFit="cover" accessibilityLabel={t.evidence} />
                  <Pressable accessibilityRole="button" accessibilityLabel={strings.photos.remove} hitSlop={12} onPress={() => setEvidence(evidence.filter((_, j) => j !== i))}
                    style={{ position: 'absolute', top: -6, right: -6, width: 22, height: 22, borderRadius: 11, backgroundColor: ui.label, alignItems: 'center', justifyContent: 'center' }}>
                    <Ionicons name="close" size={13} color={ui.bg} />
                  </Pressable>
                </View>
              ))}
            </View>
          )}
          {evidence.length < 4 && <Button size="sm" icon="receipt" onPress={pickEvidence}>{t.addEvidence}</Button>}
        </View>
      )}
      <Button variant="primary" busy={busy} onPress={save}>{t.addPayment}</Button>
    </View>
  );
}

/** Payments made, with the photos of their proof. */
export function PaymentList({ businessId, booking }: { businessId: string; booking: Booking }) {
  const ui = useUI();
  const [urls, setUrls] = useState<Record<string, string>>({});
  const when = (iso: string) => new Date(iso).toLocaleString('es-MX', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });

  useEffect(() => {
    for (const p of booking.payments) for (const key of p.evidence ?? []) {
      if (urls[key]) continue;
      api.GET('/v1/b/{businessId}/bookings/{bookingId}/files', { params: { path: { businessId, bookingId: booking.id }, query: { bucket: 'private-docs', key } } })
        .then(({ data }) => data && setUrls((u) => ({ ...u, [key]: data.url })));
    }
  }, [businessId, booking]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <View style={{ backgroundColor: ui.surface, borderRadius: radius.sm, overflow: 'hidden' }}>
      {booking.payments.map((p, i) => (
        <View key={p.id} style={{ padding: 14, gap: 8, borderTopWidth: i ? 0.5 : 0, borderColor: ui.separator }}>
          <View style={{ flexDirection: 'row', alignItems: 'baseline', gap: 8 }}>
            <View style={{ flex: 1 }}>
              <Text>{t.paymentTypes[p.type]} · {t.methods[p.method]}</Text>
              <Text variant="footnote" color="labelSecondary">{when(p.paid_at)}</Text>
            </View>
            <Text style={{ fontVariant: ['tabular-nums'], color: p.type === 'refund' ? ui.danger : ui.label }}>
              {p.type === 'refund' ? '−' : ''}{formatMXN(p.amount)}
            </Text>
          </View>
          {(p.evidence ?? []).length > 0 && (
            <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
              {(p.evidence ?? []).map((key) => urls[key]
                ? <ZoomableImage key={key} source={urls[key]} style={{ width: 56, height: 56, borderRadius: 8 }} contentFit="cover" accessibilityLabel={t.evidence} />
                : <View key={key} style={{ width: 56, height: 56, borderRadius: 8, backgroundColor: ui.fill }} />)}
            </View>
          )}
        </View>
      ))}
    </View>
  );
}
