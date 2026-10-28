import { strings } from '@lb/shared';
import { Stack, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, Button, ScrollView, Switch, Text, TextInput, View } from 'react-native';
import { api } from '../../../../lib/api';
import { errorText, fieldErrors } from '../../../../lib/errors';
import { useBusiness } from '../../../../lib/useMe';
import { useSettings, type Settings } from '../../../../lib/useSettings';

const t = strings.settings;
type Rentals = Settings['rentals'];

function Toggle({ label, hint, value, onChange, disabled }: { label: string; hint?: string; value: boolean; onChange: (v: boolean) => void; disabled?: boolean }) {
  return (
    <View style={{ gap: 2, opacity: disabled ? 0.5 : 1 }}>
      <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 12 }}>
        <Text style={{ flexShrink: 1 }}>{label}</Text>
        <Switch accessibilityLabel={label} value={value} onValueChange={onChange} disabled={disabled} />
      </View>
      {hint && <Text style={{ opacity: 0.7, fontSize: 12 }}>{hint}</Text>}
    </View>
  );
}

export default function SettingsScreen() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const { business } = useBusiness(businessId);
  const { settings, setSettings } = useSettings(businessId);
  const [r, setR] = useState<Rentals | null>(null);
  const [multiplier, setMultiplier] = useState('1');
  const [inspection, setInspection] = useState('0');
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!settings) return;
    setR(settings.rentals);
    setMultiplier(String(settings.rentals.late_fee_multiplier));
    setInspection(String(settings.rentals.inspection_minutes));
  }, [settings]);

  if (!business || !r) return <ActivityIndicator style={{ marginTop: 24 }} />;
  const isOwner = business.role === 'owner';
  const set = (patch: Partial<Rentals>) => setR({ ...r, ...patch });

  async function save() {
    setBusy(true);
    setErrors({});
    setMessage(null);
    const { data, error, response } = await api.PATCH('/v1/b/{businessId}/settings', {
      params: { path: { businessId } },
      body: { rentals: { ...r!, late_fee_multiplier: Number(multiplier.replace(',', '.')), inspection_minutes: Number(inspection) || 0 } },
    });
    setBusy(false);
    if (data) { setSettings(data); return setMessage(t.saved); }
    setErrors(fieldErrors(error));
    setMessage(errorText(error, response.status));
  }

  return (
    <ScrollView contentContainerStyle={{ padding: 16, gap: 16, maxWidth: 560, width: '100%', alignSelf: 'center' }}>
      <Stack.Screen options={{ title: t.title }} />
      {!isOwner && <Text>{t.onlyOwner}</Text>}
      <Text style={{ fontWeight: '600' }}>{t.rentals}</Text>
      <Toggle label={t.securityDeposit} hint={t.securityDepositHint} value={r.security_deposit_enabled} disabled={!isOwner}
        onChange={(v) => set({ security_deposit_enabled: v })} />
      <View style={{ gap: 4 }}>
        <Text>{t.lateMultiplier}</Text>
        <TextInput accessibilityLabel={t.lateMultiplier} value={multiplier} onChangeText={setMultiplier} keyboardType="decimal-pad" editable={isOwner}
          style={{ borderWidth: 1, borderRadius: 6, padding: 10, width: 120 }} />
        <Text style={{ opacity: 0.7, fontSize: 12 }}>{t.lateMultiplierHint}</Text>
        {errors.late_fee_multiplier && <Text accessibilityRole="alert">{errors.late_fee_multiplier}</Text>}
      </View>
      <Toggle label={t.spanClosed} value={r.rentals_span_closed_days} disabled={!isOwner} onChange={(v) => set({ rentals_span_closed_days: v })} />
      <Toggle label={t.chargeClosed} value={r.charge_closed_days} disabled={!isOwner || !r.rentals_span_closed_days}
        onChange={(v) => set({ charge_closed_days: v })} />
      <View style={{ gap: 4 }}>
        <Text>{t.inspection}</Text>
        <TextInput accessibilityLabel={t.inspection} value={inspection} onChangeText={setInspection} keyboardType="numeric" editable={isOwner}
          style={{ borderWidth: 1, borderRadius: 6, padding: 10, width: 120 }} />
        {errors.inspection_minutes && <Text accessibilityRole="alert">{errors.inspection_minutes}</Text>}
      </View>
      {message && <Text accessibilityRole="alert">{message}</Text>}
      {isOwner && <Button title={busy ? '…' : t.save} onPress={save} disabled={busy} />}
    </ScrollView>
  );
}
