import type { components } from '@lb/api-client';
import { formatMXN, strings, term } from '@lb/shared';
import { router, Stack, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useState } from 'react';
import { Ionicons } from '@expo/vector-icons';
import { Image } from 'expo-image';
import { ActivityIndicator, Pressable, StyleSheet, View } from 'react-native';
import { api } from '../../../../../lib/api';
import { errorText } from '../../../../../lib/errors';
import { useBusiness } from '../../../../../lib/useMe';
import { Button, EmptyState, Notice, Screen, Text, radius, useUI } from '../../../../../ui';

type Item = components['schemas']['CatalogItem'];
const t = strings.catalog;

function detail(it: Item): string {
  if (it.kind === 'service') return `${it.duration_minutes} min`;
  if (it.kind === 'product') return `${it.prep_hours} h de anticipación`;
  const unit = `por ${t.priceUnits[it.price_unit ?? 'day'].toLowerCase()}`;
  return it.security_deposit ? `${unit} · depósito ${formatMXN(it.security_deposit)}` : unit;
}

export default function CatalogList() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const { business } = useBusiness(businessId);
  const ui = useUI();
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

  const title = (business && term(business.vertical, 'catalog')) || '';
  return (
    <Screen>
      <Stack.Screen options={{ title }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 16 }}>{title}</Text>
      {isOwner && (
        <Button variant="primary" icon="add-circle" style={{ marginBottom: 24 }}
          onPress={() => router.push({ pathname: '/admin/[businessId]/catalog/[itemId]', params: { businessId, itemId: 'new' } })}>{t.add}</Button>
      )}
      {error && <Notice tone="warning">{error}</Notice>}
      {items === null && !error && <ActivityIndicator accessibilityLabel={strings.common.loading} />}
      {items?.length === 0 && <EmptyState icon="construct-outline" title={t.empty} />}
      {!!items?.length && (
        <View style={{ backgroundColor: ui.surface, borderRadius: radius.sm, overflow: 'hidden' }}>
          {items.map((it, i) => (
            <View key={it.id}>
              {i > 0 && <View style={{ height: StyleSheet.hairlineWidth, backgroundColor: ui.separator, marginLeft: 76 }} />}
              <Pressable accessibilityRole="button" accessibilityLabel={it.name}
                onPress={() => router.push({ pathname: '/admin/[businessId]/catalog/[itemId]', params: { businessId, itemId: it.id } })}
                style={({ pressed }) => ({ flexDirection: 'row', alignItems: 'center', gap: 12, padding: 12, paddingLeft: 16, backgroundColor: pressed ? ui.fill : 'transparent' })}>
                {it.images?.[0]?.urls?.thumb ? (
                  <Image source={it.images[0].urls.thumb} placeholder={it.images[0].blurhash ? { blurhash: it.images[0].blurhash } : undefined}
                    style={{ width: 48, height: 48, borderRadius: 10, opacity: it.active ? 1 : 0.5 }} contentFit="cover" />
                ) : (
                  <View style={{ width: 48, height: 48, borderRadius: 10, backgroundColor: ui.fill, alignItems: 'center', justifyContent: 'center' }}>
                    <Ionicons name="image-outline" size={22} color={ui.labelSecondary} />
                  </View>
                )}
                <View style={{ flex: 1, minWidth: 0, opacity: it.active ? 1 : 0.5 }}>
                  <Text numberOfLines={1}>{it.name}</Text>
                  <Text variant="footnote" color="labelSecondary" numberOfLines={1}>{it.active ? detail(it) : `${t.inactive} · ${detail(it)}`}</Text>
                </View>
                <Text color="labelSecondary" style={{ fontVariant: ['tabular-nums'] }}>{formatMXN(it.price)}</Text>
                <Ionicons name="chevron-forward" size={18} color={ui.labelSecondary} style={{ opacity: 0.6 }} />
              </Pressable>
            </View>
          ))}
        </View>
      )}
    </Screen>
  );
}
