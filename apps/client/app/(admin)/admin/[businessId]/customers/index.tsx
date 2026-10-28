import type { components } from '@lb/api-client';
import { formatPhone, strings } from '@lb/shared';
import { Link, Stack, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useState } from 'react';
import { FlatList, Text, TextInput, View } from 'react-native';
import { missingDocs } from '../../../../../components/CustomerPicker';
import { api } from '../../../../../lib/api';

type Customer = components['schemas']['Customer'];
const t = strings.customers;

export default function Customers() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const [q, setQ] = useState('');
  const [list, setList] = useState<Customer[] | null>(null);

  const search = useCallback(async (query: string) => {
    const { data } = await api.GET('/v1/b/{businessId}/customers', { params: { path: { businessId }, query: { q: query } } });
    setList(data?.customers ?? []);
  }, [businessId]);
  useFocusEffect(useCallback(() => { search(q); }, [search])); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <View style={{ flex: 1, padding: 16, gap: 12, maxWidth: 640, width: '100%', alignSelf: 'center' }}>
      <Stack.Screen options={{ title: t.title }} />
      <Link href={{ pathname: '/admin/[businessId]/customers/[customerId]', params: { businessId, customerId: 'new' } }}>
        <Text style={{ textDecorationLine: 'underline', fontSize: 16 }}>+ {t.new}</Text>
      </Link>
      <TextInput accessibilityLabel={t.search} placeholder={t.search} value={q} onChangeText={(v) => { setQ(v); search(v); }}
        style={{ borderWidth: 1, borderRadius: 6, padding: 10 }} />
      <FlatList data={list ?? []} keyExtractor={(c) => c.id}
        ListEmptyComponent={list ? <Text>{t.empty}</Text> : undefined}
        renderItem={({ item: c }) => {
          const missing = missingDocs(c);
          return (
            <Link href={{ pathname: '/admin/[businessId]/customers/[customerId]', params: { businessId, customerId: c.id } }} style={{ paddingVertical: 10, borderBottomWidth: 1, borderColor: '#ddd' }}>
              <View>
                <Text style={{ fontWeight: '600' }}>{c.name}{c.trusted ? ` · ${t.trustedBadge}` : ''}</Text>
                <Text>{formatPhone(c.phone)}</Text>
                {missing.length > 0 && <Text style={{ opacity: 0.7 }}>{t.missingDocs(missing.join(', '))}</Text>}
              </View>
            </Link>
          );
        }} />
    </View>
  );
}
