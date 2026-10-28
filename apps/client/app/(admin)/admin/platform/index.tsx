import type { components } from '@lb/api-client';
import { featureInfo, strings, verticalMap } from '@lb/shared';
import { router, Stack, useFocusEffect } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator } from 'react-native';
import { api } from '../../../../lib/api';
import { errorText } from '../../../../lib/errors';
import { Cell, EmptyState, Group, Notice, Screen, Text, useUI } from '../../../../ui';

type AdminBusiness = components['schemas']['AdminBusiness'];
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
