import type { components } from '@lb/api-client';
import { featureInfo, strings, verticalMap } from '@lb/shared';
import { router, Stack, useFocusEffect } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator, View } from 'react-native';
import { api } from '../../../../lib/api';
import { errorText } from '../../../../lib/errors';
import { Cell, EmptyState, Group, Notice, Screen, Segmented, Text, useToast, useUI } from '../../../../ui';

type AdminBusiness = components['schemas']['AdminBusiness'];
type Availability = components['schemas']['VerticalAvailability'];
type Status = Availability['status'];
const STATUSES: Status[] = ['public', 'admin', 'off'];
const t = strings.platform;

/** Platform admin: every business, its giro and its active features. */
export default function PlatformHome() {
  const ui = useUI();
  const [list, setList] = useState<AdminBusiness[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useFocusEffect(useCallback(() => {
    api.GET('/v1/admin/businesses').then(({ data, error: err, response }) => (data ? setList(data.businesses) : setError(errorText(err, response.status))));
  }, []));

  return (
    <Screen>
      <Stack.Screen options={{ title: t.title }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 8 }}>{t.title}</Text>
      <Text variant="subhead" color="labelSecondary" style={{ marginBottom: 20 }}>{t.hint}</Text>
      {error && <Notice tone="warning">{error}</Notice>}
      {!list && !error && <ActivityIndicator />}
      {list?.length === 0 && <EmptyState icon="storefront-outline" title={t.empty} />}
      <Verticals />
      {!!list?.length && (
        <Group label={t.businesses(list.length)} separatorInset={58}>
          {list.map((b) => {
            const active = b.features.filter((f) => f.active).map((f) => featureInfo(f.key).label);
            return (
              <Cell key={b.business_id} icon="storefront" tint={ui.indigo} label={b.name}
                sublabel={`${verticalMap(b.vertical).label} · ${active.length ? active.join(', ') : t.noFeatures}`}
                onPress={() => router.push({ pathname: '/admin/platform/[businessId]', params: { businessId: b.business_id } })} />
            );
          })}
        </Group>
      )}
    </Screen>
  );
}

/** Who can open a new business of each giro: everyone, only the platform admin (to try it out), nobody. */
function Verticals() {
  const toast = useToast();
  const [list, setList] = useState<Availability[] | null>(null);
  useFocusEffect(useCallback(() => {
    api.GET('/v1/admin/verticals').then(({ data }) => setList(data?.verticals ?? []));
  }, []));

  async function set(vertical: Availability['vertical'], status: Status) {
    const { data, error, response } = await api.PUT('/v1/admin/verticals/{vertical}', { params: { path: { vertical } }, body: { status } });
    if (!data) return toast(errorText(error, response.status), 'error');
    setList((l) => l?.map((v) => (v.vertical === vertical ? data : v)) ?? null);
    toast(t.verticals.saved(verticalMap(vertical).label, t.verticals.statuses[status]));
  }

  if (!list) return null;
  return (
    <Group label={t.verticals.title} padded footer={t.verticals.hint}>
      {list.map((v) => (
        <View key={v.vertical} style={{ gap: 6, marginBottom: 12 }}>
          <Text>{verticalMap(v.vertical).label}</Text>
          <Segmented options={STATUSES.map((s) => ({ value: s, label: t.verticals.statuses[s] }))} value={v.status} onChange={(s) => set(v.vertical, s)} />
        </View>
      ))}
    </Group>
  );
}
