import { strings } from '@lb/shared';
import { Stack, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, Switch, View } from 'react-native';
import { api } from '../../../../lib/api';
import { errorText, fieldErrors } from '../../../../lib/errors';
import { useBusiness } from '../../../../lib/useMe';
import { DocumentTypesEditor } from '../../../../components/DocumentTypesEditor';
import { useSettings, type Settings } from '../../../../lib/useSettings';
import { Button, Cell, Field, Group, Notice, Screen, Text, useToast } from '../../../../ui';

const t = strings.settings;
type Rentals = Settings['rentals'];

export default function SettingsScreen() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const { business } = useBusiness(businessId);
  const { settings, setSettings } = useSettings(businessId);
  const toast = useToast();
  const [r, setR] = useState<Rentals | null>(null);
  const [multiplier, setMultiplier] = useState('1');
  const [inspectionOn, setInspectionOn] = useState(false);
  const [hours, setHours] = useState('1');
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!settings) return;
    setR(settings.rentals);
    setMultiplier(String(settings.rentals.late_fee_multiplier));
    // Stored in minutes; captured in whole hours.
    setInspectionOn(settings.rentals.inspection_minutes > 0);
    setHours(String(Math.max(1, Math.round(settings.rentals.inspection_minutes / 60))));
  }, [settings]);

  if (!business || !r) return <ActivityIndicator style={{ marginTop: 24 }} />;
  const isOwner = business.role === 'owner';
  const set = (patch: Partial<Rentals>) => setR({ ...r, ...patch });

  async function save() {
    const h = Number(hours);
    if (inspectionOn && (!Number.isInteger(h) || h < 1 || h > 168)) return setErrors({ inspection_minutes: t.inspectionHoursInvalid });
    setBusy(true);
    setErrors({});
    const { data, error, response } = await api.PATCH('/v1/b/{businessId}/settings', {
      params: { path: { businessId } },
      body: { rentals: { ...r!, late_fee_multiplier: Number(multiplier.replace(',', '.')), inspection_minutes: inspectionOn ? h * 60 : 0 } },
    });
    setBusy(false);
    if (data) { setSettings(data); return toast(t.saved); }
    setErrors(fieldErrors(error));
    toast(errorText(error, response.status), 'error');
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

      <Group label={t.late} padded>
        <Field label={t.lateMultiplier} value={multiplier} onChangeText={setMultiplier} keyboardType="decimal-pad" editable={isOwner}
          hint={t.lateMultiplierHint} error={errors.late_fee_multiplier} />
      </Group>

      <Group label={t.inspectionTitle} footer={t.inspectionHint}>
        <Cell label={t.inspectionToggle} accessory={
          <Switch accessibilityLabel={t.inspectionToggle} value={inspectionOn} disabled={!isOwner} onValueChange={setInspectionOn} />} />
        {inspectionOn ? (
          <View style={{ paddingHorizontal: 16, paddingTop: 4 }}>
            <Field label={t.inspectionHours} value={hours} onChangeText={setHours} keyboardType="numeric" editable={isOwner} error={errors.inspection_minutes} />
          </View>
        ) : null}
      </Group>

      {isOwner && <Button variant="primary" busy={busy} onPress={save} style={{ marginBottom: 28 }}>{t.save}</Button>}

      <Text variant="footnote" color="labelSecondary" style={{ marginHorizontal: 16, marginBottom: 6 }}>{t.documents.title.toUpperCase()}</Text>
      <DocumentTypesEditor businessId={businessId} isOwner={isOwner} />
      <Text variant="footnote" color="labelSecondary" style={{ marginHorizontal: 16, marginBottom: 28 }}>{t.documents.hint}</Text>

    </Screen>
  );
}
