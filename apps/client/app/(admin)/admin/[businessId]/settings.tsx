import { centavosToInput, parsePesos, settingsSectionsFor, strings, type SettingsSection } from '@lb/shared';
import { Ionicons } from '@expo/vector-icons';
import { Stack, useLocalSearchParams } from 'expo-router';
import { useEffect, useState, type ReactNode } from 'react';
import { ActivityIndicator, Pressable, Switch, View } from 'react-native';
import { DocumentTypesEditor } from '../../../../components/DocumentTypesEditor';
import { api } from '../../../../lib/api';
import { errorText, fieldErrors } from '../../../../lib/errors';
import { useBusiness } from '../../../../lib/useMe';
import { useSettings, type Settings } from '../../../../lib/useSettings';
import { Button, Cell, Field, Group, Notice, Screen, Text, useToast, useUI } from '../../../../ui';

const t = strings.settings;
type Rentals = Settings['rentals'];
type ZoneRow = { name: string; fee: string }; // fee as typed (pesos)

/** Ajustes: the shared section plus the ones of this type of business (see settingsSectionsFor). */
export default function SettingsScreen() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const { business } = useBusiness(businessId);
  const { settings, setSettings } = useSettings(businessId);
  const toast = useToast();
  const ui = useUI();
  const [r, setR] = useState<Rentals | null>(null);
  const [deposit, setDeposit] = useState('0');
  const [hold, setHold] = useState('24');
  const [multiplier, setMultiplier] = useState('1');
  const [inspectionOn, setInspectionOn] = useState(false);
  const [hours, setHours] = useState('1');
  const [zones, setZones] = useState<ZoneRow[]>([]);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!settings) return;
    setR(settings.rentals);
    setDeposit(String(settings.general.deposit_percent));
    setHold(String(settings.general.hold_hours));
    setMultiplier(String(settings.rentals.late_fee_multiplier));
    // Stored in minutes; captured in whole hours.
    setInspectionOn(settings.rentals.inspection_minutes > 0);
    setHours(String(Math.max(1, Math.round(settings.rentals.inspection_minutes / 60))));
    setZones(settings.delivery.delivery_zones.map((z) => ({ name: z.name, fee: centavosToInput(z.fee) })));
  }, [settings]);

  if (!business || !r) return <ActivityIndicator style={{ marginTop: 24 }} />;
  const isOwner = business.role === 'owner';
  const sections = settingsSectionsFor(business.vertical);
  const has = (s: SettingsSection) => sections.includes(s);
  const set = (patch: Partial<Rentals>) => setR({ ...r, ...patch });

  async function save() {
    const local: Record<string, string> = {};
    const h = Number(hours);
    if (has('inspection') && inspectionOn && (!Number.isInteger(h) || h < 1 || h > 168)) local.inspection_minutes = t.inspectionHoursInvalid;
    const fees = zones.map((z) => parsePesos(z.fee || '0'));
    if (fees.some((f) => f === null)) local.delivery_zones = strings.catalog.invalidMoney;
    if (Object.keys(local).length) return setErrors(local);
    setBusy(true);
    setErrors({});
    // Only the sections this business has are sent (the API validates just those).
    const { data, error, response } = await api.PATCH('/v1/b/{businessId}/settings', {
      params: { path: { businessId } },
      body: {
        general: { deposit_percent: Number(deposit) || 0, hold_hours: Number(hold) || 0 },
        ...(has('rentals') ? {
          rentals: { ...r!, late_fee_multiplier: Number(multiplier.replace(',', '.')), inspection_minutes: inspectionOn ? h * 60 : 0 },
        } : {}),
        ...(has('delivery') ? { delivery: { delivery_zones: zones.map((z, i) => ({ name: z.name, fee: fees[i]! })) } } : {}),
      },
    });
    setBusy(false);
    if (data) { setSettings(data); return toast(t.saved); }
    setErrors(fieldErrors(error));
    toast(errorText(error, response.status), 'error');
  }

  const render: Record<SettingsSection, () => ReactNode> = {
    general: () => (
      <Group key="general" label={t.general.title} padded footer={t.general.hint}>
        <Field label={t.general.deposit} value={deposit} onChangeText={setDeposit} keyboardType="numeric" editable={isOwner}
          hint={t.general.depositHint} error={errors.deposit_percent} />
        <Field label={t.general.hold} value={hold} onChangeText={setHold} keyboardType="numeric" editable={isOwner}
          hint={t.general.holdHint} error={errors.hold_hours} />
      </Group>
    ),
    rentals: () => (
      <View key="rentals">
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
      </View>
    ),
    inspection: () => (
      <Group key="inspection" label={t.inspectionTitle} footer={t.inspectionHint}>
        <Cell label={t.inspectionToggle} accessory={
          <Switch accessibilityLabel={t.inspectionToggle} value={inspectionOn} disabled={!isOwner} onValueChange={setInspectionOn} />} />
        {inspectionOn ? (
          <View style={{ paddingHorizontal: 16, paddingTop: 4 }}>
            <Field label={t.inspectionHours} value={hours} onChangeText={setHours} keyboardType="numeric" editable={isOwner} error={errors.inspection_minutes} />
          </View>
        ) : null}
      </Group>
    ),
    delivery: () => (
      <Group key="delivery" label={t.delivery.title} padded footer={errors.delivery_zones ?? t.delivery.hint}>
        {zones.length === 0 && <Text color="labelSecondary" style={{ marginBottom: 12 }}>{t.delivery.empty}</Text>}
        {zones.map((z, i) => (
          <View key={i} style={{ flexDirection: 'row', gap: 8, alignItems: 'flex-start' }}>
            <View style={{ flex: 2 }}>
              <Field label={t.delivery.zone} value={z.name} editable={isOwner}
                onChangeText={(name) => setZones(zones.map((x, j) => (j === i ? { ...x, name } : x)))} />
            </View>
            <View style={{ flex: 1 }}>
              <Field label={t.delivery.fee} value={z.fee} keyboardType="decimal-pad" editable={isOwner}
                onChangeText={(fee) => setZones(zones.map((x, j) => (j === i ? { ...x, fee } : x)))} />
            </View>
            {isOwner && (
              <Pressable accessibilityRole="button" accessibilityLabel={`${t.delivery.remove} ${z.name}`} hitSlop={10}
                onPress={() => setZones(zones.filter((_, j) => j !== i))} style={{ paddingTop: 36 }}>
                <Ionicons name="trash-outline" size={20} color={ui.danger} />
              </Pressable>
            )}
          </View>
        ))}
        {isOwner && <Button size="sm" icon="add" onPress={() => setZones([...zones, { name: '', fee: '' }])}>{t.delivery.add}</Button>}
      </Group>
    ),
    // Saved at once (not with the button), so it goes after it.
    documents: () => null,
  };

  return (
    <Screen>
      <Stack.Screen options={{ title: t.title }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 20 }}>{t.title}</Text>
      {!isOwner && <View style={{ marginBottom: 16 }}><Notice>{t.onlyOwner}</Notice></View>}
      {sections.map((s) => render[s]())}
      {isOwner && <Button variant="primary" busy={busy} onPress={save} style={{ marginBottom: 28 }}>{t.save}</Button>}
      {has('documents') && (
        <>
          <Text variant="footnote" color="labelSecondary" style={{ marginHorizontal: 16, marginBottom: 6 }}>{t.documents.title.toUpperCase()}</Text>
          <DocumentTypesEditor businessId={businessId} isOwner={isOwner} />
          <Text variant="footnote" color="labelSecondary" style={{ marginHorizontal: 16, marginBottom: 28 }}>{t.documents.hint}</Text>
        </>
      )}
    </Screen>
  );
}
