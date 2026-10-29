import { centavosToInput, featureInfo, parsePesos, settingsSectionsFor, strings, type SettingsSection } from '@lb/shared';
import { Ionicons } from '@expo/vector-icons';
import { router, Stack, useLocalSearchParams, type Href } from 'expo-router';
import { useEffect, useState, type ReactNode } from 'react';
import { ActivityIndicator, Pressable, Switch, View } from 'react-native';
import { DeleteBusiness } from '../../../../components/DeleteBusiness';
import { ExportData } from '../../../../components/ExportData';
import { ImportData } from '../../../../components/ImportData';
import { DocumentTypesEditor } from '../../../../components/DocumentTypesEditor';
import { api } from '../../../../lib/api';
import { errorText, fieldErrors } from '../../../../lib/errors';
import { useBusiness } from '../../../../lib/useMe';
import { useSettings, type Settings } from '../../../../lib/useSettings';
import { Button, Cell, Field, Group, Notice, Screen, Text, useToast, useUI } from '../../../../ui';

const t = strings.settings;
const pv = strings.privacy.settings;
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
  const [onlineOnly, setOnlineOnly] = useState(false);
  const [buffer, setBuffer] = useState('0');
  const [multiplier, setMultiplier] = useState('1');
  const [inspectionOn, setInspectionOn] = useState(false);
  const [hours, setHours] = useState('1');
  const [zones, setZones] = useState<ZoneRow[]>([]);
  const [privacy, setPrivacy] = useState<Settings['privacy']>({ privacy_legal_name: '', privacy_address: '', privacy_email: '' });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!settings) return;
    setR(settings.rentals);
    setDeposit(String(settings.general.deposit_percent));
    setHold(String(settings.general.hold_hours));
    setOnlineOnly(settings.general.advance_only_online);
    setBuffer(String(settings.agenda.buffer_minutes));
    setMultiplier(String(settings.rentals.late_fee_multiplier));
    // Stored in minutes; captured in whole hours.
    setInspectionOn(settings.rentals.inspection_minutes > 0);
    setHours(String(Math.max(1, Math.round(settings.rentals.inspection_minutes / 60))));
    setZones(settings.delivery.delivery_zones.map((z) => ({ name: z.name, fee: centavosToInput(z.fee) })));
    setPrivacy(settings.privacy);
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
        general: { deposit_percent: Number(deposit) || 0, hold_hours: Number(hold) || 0, advance_only_online: onlineOnly },
        ...(has('agenda') ? { agenda: { buffer_minutes: Number(buffer) || 0 } } : {}),
        ...(has('rentals') ? {
          rentals: { ...r!, late_fee_multiplier: Number(multiplier.replace(',', '.')), inspection_minutes: inspectionOn ? h * 60 : 0 },
        } : {}),
        ...(has('delivery') ? { delivery: { delivery_zones: zones.map((z, i) => ({ name: z.name, fee: fees[i]! })) } } : {}),
        privacy,
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
        <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12 }}>
          <View style={{ flex: 1 }}>
            <Text>{t.general.onlineOnly}</Text>
            <Text variant="footnote" color="labelSecondary">{t.general.onlineOnlyHint}</Text>
          </View>
          <Switch accessibilityLabel={t.general.onlineOnly} value={onlineOnly} disabled={!isOwner} onValueChange={setOnlineOnly} />
        </View>
      </Group>
    ),
    agenda: () => (
      <Group key="agenda" label={t.agenda.title} padded footer={t.agenda.hint}>
        <Field label={t.agenda.buffer} value={buffer} onChangeText={setBuffer} keyboardType="numeric" editable={isOwner} error={errors.buffer_minutes} />
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
      {/* Every business: the responsible in the privacy notice its customers read. */}
      <Group label={pv.title} padded footer={pv.hint}>
        {!privacy.privacy_address && <Notice tone="warning">{pv.missingAddress}</Notice>}
        <Field label={pv.legalName} placeholder={business.name} value={privacy.privacy_legal_name} editable={isOwner}
          onChangeText={(privacy_legal_name) => setPrivacy({ ...privacy, privacy_legal_name })} error={errors.privacy_legal_name} />
        <Field label={pv.address} value={privacy.privacy_address} editable={isOwner} multiline
          onChangeText={(privacy_address) => setPrivacy({ ...privacy, privacy_address })} error={errors.privacy_address} />
        <Field label={pv.email} value={privacy.privacy_email} editable={isOwner} autoCapitalize="none" keyboardType="email-address"
          onChangeText={(privacy_email) => setPrivacy({ ...privacy, privacy_email })} error={errors.privacy_email} />
        <Button size="sm" variant="plain" icon="document-text" onPress={() => router.push(`/${business.slug}/privacidad` as Href)}>{pv.see}</Button>
      </Group>
      {isOwner && <Button variant="primary" busy={busy} onPress={save} style={{ marginBottom: 28 }}>{t.save}</Button>}
      <FeatureToggles businessId={businessId} isOwner={isOwner} features={settings!.features} onSaved={setSettings} />
      {has('documents') && (
        <>
          <Text variant="footnote" color="labelSecondary" style={{ marginHorizontal: 16, marginBottom: 6 }}>{t.documents.title.toUpperCase()}</Text>
          <DocumentTypesEditor businessId={businessId} isOwner={isOwner} />
          <Text variant="footnote" color="labelSecondary" style={{ marginHorizontal: 16, marginBottom: 28 }}>{t.documents.hint}</Text>
        </>
      )}
      {isOwner && <ExportData businessId={businessId} />}
      {isOwner && <ImportData businessId={businessId} />}
      {isOwner && <DeleteBusiness businessId={businessId} name={business.name} onDeleted={() => router.replace('/admin')} />}
    </Screen>
  );
}

/**
 * Parts of the app this business may use (its giro's, plus what the platform admin gave it), each
 * with an on/off switch for the owner. Saved at once.
 */
function FeatureToggles({ businessId, isOwner, features, onSaved }: {
  businessId: string; isOwner: boolean; features: Settings['features']; onSaved: (s: Settings) => void;
}) {
  const toast = useToast();
  const available = features.filter((f) => f.available);
  if (!available.length) return null;

  async function toggle(key: string, on: boolean) {
    const { data, error, response } = await api.PATCH('/v1/b/{businessId}/settings', {
      params: { path: { businessId } }, body: { features: { [key]: on } },
    });
    if (data) { onSaved(data); return toast(t.features.saved); }
    toast(errorText(error, response.status), 'error');
  }

  return (
    <Group label={t.features.title} footer={t.features.hint}>
      {available.map((f) => (
        <Cell key={f.key} label={featureInfo(f.key).label} sublabel={featureInfo(f.key).hint} accessory={
          <Switch accessibilityLabel={featureInfo(f.key).label} value={f.enabled} disabled={!isOwner} onValueChange={(on) => toggle(f.key, on)} />} />
      ))}
    </Group>
  );
}
