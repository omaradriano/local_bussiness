import type { components } from '@lb/api-client';
import { formatPhone, strings } from '@lb/shared';
import { router, Stack, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator } from 'react-native';
import { missingDocs } from '../../../../../components/CustomerPicker';
import { api } from '../../../../../lib/api';
import { useTexts } from '../../../../../lib/useTexts';
import { Button, Cell, EmptyState, Field, Group, Screen, Text, useUI } from '../../../../../ui';

type Customer = components['schemas']['Customer'];
const t = strings.customers;

export default function Customers() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const ui = useUI();
  const { has } = useTexts();
  const trusted = (c: { trusted: boolean }) => has('trustedCustomers') && c.trusted;
  const [q, setQ] = useState('');
  const [list, setList] = useState<Customer[] | null>(null);

  const search = useCallback(async (query: string) => {
    const { data } = await api.GET('/v1/b/{businessId}/customers', { params: { path: { businessId }, query: { q: query } } });
    setList(data?.customers ?? []);
  }, [businessId]);
  useFocusEffect(useCallback(() => { search(q); }, [search])); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <Screen>
      <Stack.Screen options={{ title: t.title }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 16 }}>{t.title}</Text>
      <Field label={t.search} placeholder={t.searchPlaceholder} value={q} onChangeText={(v) => { setQ(v); search(v); }} autoCorrect={false} />
      <Button variant="primary" icon="person-add" style={{ marginBottom: 24 }}
        onPress={() => router.push({ pathname: '/admin/[businessId]/customers/[customerId]', params: { businessId, customerId: 'new' } })}>{t.new}</Button>
      {list === null && <ActivityIndicator />}
      {list?.length === 0 && <EmptyState icon="people-outline" title={t.empty} />}
      {!!list?.length && (
        <Group separatorInset={58}>
          {list.map((c) => {
            const missing = has('customerDocuments') ? missingDocs(c) : [];
            return (
              <Cell key={c.id} icon={trusted(c) ? 'shield-checkmark' : 'person'} tint={trusted(c) ? ui.success : ui.indigo}
                label={c.name}
                sublabel={`${formatPhone(c.phone)}${trusted(c) ? ` · ${t.trustedBadge}` : missing.length ? ` · ${t.missingDocs(missing.join(', '))}` : ''}`}
                onPress={() => router.push({ pathname: '/admin/[businessId]/customers/[customerId]', params: { businessId, customerId: c.id } })} />
            );
          })}
        </Group>
      )}
    </Screen>
  );
}
