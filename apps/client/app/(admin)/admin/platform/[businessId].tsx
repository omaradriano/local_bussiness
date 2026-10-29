import type { components } from '@lb/api-client';
import { featureInfo, strings, verticalMap } from '@lb/shared';
import { router, Stack, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, StyleSheet, View } from 'react-native';
import { DeleteBusiness } from '../../../../components/DeleteBusiness';
import { api } from '../../../../lib/api';
import { errorText } from '../../../../lib/errors';
import { Button, Field, Group, Notice, Screen, Segmented, Text, radius, useToast, useUI } from '../../../../ui';

type AdminBusiness = components['schemas']['AdminBusiness'];
type FeatureState = AdminBusiness['features'][number];
type Choice = 'giro' | 'give' | 'remove';
const t = strings.platform;

/**
 * Platform admin: what each feature of one business does. "Como el giro" follows its giro; "Dar" and
 * "Quitar" override it for this business only. The owner then turns available ones on or off.
 */
export default function PlatformBusiness() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const ui = useUI();
  const toast = useToast();
  const [b, setB] = useState<AdminBusiness | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.GET('/v1/admin/businesses').then(({ data, error: err, response }) => {
      const found = data?.businesses.find((x) => x.business_id === businessId);
      if (found) setB(found);
      else setError(data ? strings.common.notFound : errorText(err, response.status));
    });
  }, [businessId]);

  async function set(f: FeatureState, choice: Choice) {
    const grant = choice === 'giro' ? null : choice === 'give';
    const { data, error: err, response } = await api.PUT('/v1/admin/businesses/{businessId}/features/{feature}', {
      params: { path: { businessId, feature: f.key } }, body: { grant },
    });
    if (data) { setB(data); return toast(t.saved); }
    toast(errorText(err, response.status), 'error');
  }

  if (error) return <Screen><Notice tone="warning">{error}</Notice></Screen>;
  if (!b) return <ActivityIndicator style={{ marginTop: 24 }} />;
  const status = (f: FeatureState) => (!f.available ? t.state.off : f.enabled ? t.state.on : t.state.ownerOff);

  return (
    <Screen>
      <Stack.Screen options={{ title: b.name }} />
      <Text variant="footnote" color="labelSecondary">{verticalMap(b.vertical).label} · /{b.slug}</Text>
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 8 }}>{b.name}</Text>
      <AccountPlan businessId={businessId} account={b.account} onSaved={(account) => setB({ ...b, account })} />
      <Text variant="subhead" color="labelSecondary" style={{ marginBottom: 20 }}>{t.featuresHint}</Text>
      <View style={{ backgroundColor: ui.surface, borderRadius: radius.sm, overflow: 'hidden' }}>
        {b.features.map((f, i) => (
          <View key={f.key} style={{ padding: 16, gap: 10, borderTopWidth: i ? StyleSheet.hairlineWidth : 0, borderColor: ui.separator }}>
            <View style={{ flexDirection: 'row', alignItems: 'baseline', gap: 8 }}>
              <Text style={{ flex: 1, fontWeight: '600' }}>{featureInfo(f.key).label}</Text>
              <Text variant="footnote" style={{ color: f.active ? ui.success : ui.labelSecondary }}>{status(f)}</Text>
            </View>
            <Text variant="footnote" color="labelSecondary">{featureInfo(f.key).hint}</Text>
            <Segmented
              options={[
                { value: 'giro', label: f.from_giro ? t.choice.giroOn : t.choice.giroOff },
                { value: 'give', label: t.choice.give },
                { value: 'remove', label: t.choice.remove },
              ]}
              value={f.grant === null ? 'giro' : f.grant ? 'give' : 'remove'}
              onChange={(c: Choice) => set(f, c)}
            />
          </View>
        ))}
      </View>
      <View style={{ marginTop: 28 }}>
        <DeleteBusiness businessId={businessId} name={b.name} asAdmin onDeleted={() => router.replace('/admin/platform')} />
      </View>
    </Screen>
  );
}

type Account = AdminBusiness['account'];
const PLANS: Account['plan'][] = ['trial', 'lite', 'pro', 'business'];
const pa = strings.plans.admin;
// ends_at is the instant after the last day: show that last day.
const dayOf = (iso: string) => new Date(new Date(iso).getTime() - 1).toLocaleDateString('es-MX', { timeZone: 'UTC', day: 'numeric', month: 'long', year: 'numeric' });

/** The plan of the owner's account (all their businesses), assigned by hand while there's no billing. */
function AccountPlan({ businessId, account, onSaved }: { businessId: string; account: Account; onSaved: (a: Account) => void }) {
  const toast = useToast();
  const ui = useUI();
  const [plan, setPlan] = useState(account.plan);
  // ends_at is the instant after the last day: show that last day.
  const [endsAt, setEndsAt] = useState(account.ends_at ? new Date(new Date(account.ends_at).getTime() - 1).toISOString().slice(0, 10) : '');
  const [note, setNote] = useState('');
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);

  async function save() {
    setBusy(true);
    const { data, error, response } = await api.PUT('/v1/admin/businesses/{businessId}/plan', {
      params: { path: { businessId } }, body: { plan, note, ...(endsAt ? { ends_at: endsAt } : {}) },
    });
    setBusy(false);
    if (!data) { setErrors((error?.details as Record<string, string>) ?? {}); return toast(errorText(error, response.status), 'error'); }
    setErrors({});
    onSaved(data);
    toast(pa.saved);
  }

  const current = `${strings.plans.names[account.plan]} · ${pa.status[account.status]} · ${account.ends_at ? pa.until(dayOf(account.ends_at)) : pa.noEnd}`;
  return (
    <Group label={pa.title} padded footer={pa.hint}>
      <Text variant="subhead" style={{ color: account.status === 'expired' ? ui.warning : ui.labelSecondary, marginBottom: 12 }}>{current}</Text>
      <View style={{ marginBottom: 16 }}>
        <Segmented options={PLANS.map((p) => ({ value: p, label: strings.plans.names[p] }))} value={plan} onChange={setPlan} />
      </View>
      <Field label={pa.endsAt} value={endsAt} onChangeText={setEndsAt} error={errors.ends_at} />
      <Field label={pa.note} value={note} onChangeText={setNote} error={errors.note} />
      <Button variant="primary" busy={busy} onPress={save}>{pa.save}</Button>
    </Group>
  );
}
