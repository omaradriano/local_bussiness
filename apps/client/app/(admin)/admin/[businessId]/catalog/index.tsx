import type { components } from '@lb/api-client';
import { formatMXN, strings, term } from '@lb/shared';
import { Link, router, Stack, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator, Button, ScrollView, Text, View } from 'react-native';
import { api } from '../../../../../lib/api';
import { errorText } from '../../../../../lib/errors';
import { useBusiness } from '../../../../../lib/useMe';

type Item = components['schemas']['CatalogItem'];
const t = strings.catalog;

function detail(it: Item): string {
  if (it.kind === 'service') return `${it.duration_minutes} min`;
  if (it.kind === 'product') return `${it.prep_hours} h de anticipación`;
  return `por ${t.priceUnits[it.price_unit ?? 'day'].toLowerCase()} · depósito ${formatMXN(it.security_deposit ?? 0)}`;
}

export default function CatalogList() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const { business } = useBusiness(businessId);
  const [items, setItems] = useState<Item[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const isOwner = business?.role === 'owner';

  const load = useCallback(async () => {
    setError(null);
    const { data, error, response } = await api.GET('/v1/b/{businessId}/catalog', { params: { path: { businessId } } });
    if (data) setItems(data.items);
    else setError(errorText(error, response.status));
  }, [businessId]);

  // Reload when coming back from the edit screen.
  useFocusEffect(useCallback(() => { load(); }, [load]));

  async function toggle(it: Item) {
    const { data, error, response } = await api.PATCH('/v1/b/{businessId}/catalog/{itemId}', {
      params: { path: { businessId, itemId: it.id } },
      body: { active: !it.active },
    });
    if (data) setItems((prev) => prev?.map((x) => (x.id === data.id ? data : x)) ?? null);
    else setError(errorText(error, response.status));
  }

  const title = (business && term(business.vertical, 'catalog')) || '';
  return (
    <ScrollView contentContainerStyle={{ padding: 16, gap: 12 }}>
      <Stack.Screen options={{ title }} />
      {isOwner && (
        <Button title={t.add} onPress={() => router.push({ pathname: '/admin/[businessId]/catalog/[itemId]', params: { businessId, itemId: 'new' } })} />
      )}
      {error && <Text accessibilityRole="alert">{error}</Text>}
      {items === null && !error && <ActivityIndicator accessibilityLabel={strings.common.loading} />}
      {items?.length === 0 && <Text>{t.empty}</Text>}
      {items?.map((it) => (
        <View key={it.id} style={{ borderWidth: 1, borderRadius: 8, padding: 12, gap: 4, opacity: it.active ? 1 : 0.55 }}>
          <Text style={{ fontWeight: '600' }}>
            {it.name} · {formatMXN(it.price)} {it.active ? '' : `(${t.inactive})`}
          </Text>
          <Text>{detail(it)}</Text>
          {isOwner && (
            <View style={{ flexDirection: 'row', gap: 16, marginTop: 4 }}>
              <Link href={{ pathname: '/admin/[businessId]/catalog/[itemId]', params: { businessId, itemId: it.id } }}>
                <Text style={{ textDecorationLine: 'underline' }}>Editar</Text>
              </Link>
              <Text accessibilityRole="button" onPress={() => toggle(it)} style={{ textDecorationLine: 'underline' }}>
                {it.active ? t.deactivate : t.activate}
              </Text>
            </View>
          )}
        </View>
      ))}
    </ScrollView>
  );
}
