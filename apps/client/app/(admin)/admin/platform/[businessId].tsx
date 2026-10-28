import type { components } from '@lb/api-client';
import { featureInfo, strings, verticalMap } from '@lb/shared';
import { Stack, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, StyleSheet, View } from 'react-native';
import { api } from '../../../../lib/api';
import { errorText } from '../../../../lib/errors';
import { Notice, Screen, Segmented, Text, radius, useToast, useUI } from '../../../../ui';

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
    </Screen>
  );
}
