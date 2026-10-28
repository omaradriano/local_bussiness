import { strings } from '@lb/shared';
import { Stack, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, Switch, View } from 'react-native';
import { api } from '../../../../lib/api';
import { errorText, fieldErrors } from '../../../../lib/errors';
import { useBusiness } from '../../../../lib/useMe';
import { useSettings, type Settings } from '../../../../lib/useSettings';
import { Button, Cell, Field, Group, Notice, Screen, Text } from '../../../../ui';

const t = strings.settings;
type Rentals = Settings['rentals'];

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
    <Screen>
      <Stack.Screen options={{ title: t.title }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 20 }}>{t.title}</Text>
      {!isOwner && <View style={{ marginBottom: 16 }}><Notice>{t.onlyOwner}</Notice></View>}

      <Group label={t.rentals} footer={t.securityDepositHint}>
        <Cell label={t.securityDeposit} accessory={
          <Switch accessibilityLabel={t.securityDeposit} value={r.security_deposit_enabled} disabled={!isOwner}
            onValueChange={(v) => set({ security_deposit_enabled: v })} />} />
      </Group>

      <Group label={t.closedDays} footer={t.closedDaysHint}>
        <Cell label={t.spanClosed} accessory={
          <Switch accessibilityLabel={t.spanClosed} value={r.rentals_span_closed_days} disabled={!isOwner}
            onValueChange={(v) => set({ rentals_span_closed_days: v })} />} />
        <Cell label={t.chargeClosed} accessory={
          <Switch accessibilityLabel={t.chargeClosed} value={r.charge_closed_days} disabled={!isOwner || !r.rentals_span_closed_days}
            onValueChange={(v) => set({ charge_closed_days: v })} />} />
      </Group>

      <Group label={t.lateAndInspection} padded>
        <Field label={t.lateMultiplier} value={multiplier} onChangeText={setMultiplier} keyboardType="decimal-pad" editable={isOwner}
          hint={t.lateMultiplierHint} error={errors.late_fee_multiplier} />
        <Field label={t.inspection} value={inspection} onChangeText={setInspection} keyboardType="numeric" editable={isOwner}
          hint={t.inspectionHint} error={errors.inspection_minutes} />
      </Group>

      {message && <View style={{ marginBottom: 16 }}><Notice tone={message === t.saved ? 'success' : 'warning'}>{message}</Notice></View>}
      {isOwner && <Button variant="primary" busy={busy} onPress={save}>{t.save}</Button>}
    </Screen>
  );
}
