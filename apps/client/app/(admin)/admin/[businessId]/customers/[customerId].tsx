import type { components } from '@lb/api-client';
import { strings } from '@lb/shared';
import { router, Stack, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, ScrollView, Text } from 'react-native';
import { CustomerDocuments, CustomerForm } from '../../../../../components/CustomerForm';
import { api } from '../../../../../lib/api';
import { useBusiness } from '../../../../../lib/useMe';

type Customer = components['schemas']['Customer'];
const t = strings.customers;

export default function CustomerScreen() {
  const { businessId, customerId } = useLocalSearchParams<{ businessId: string; customerId: string }>();
  const { business } = useBusiness(businessId);
  const isNew = customerId === 'new';
  const [customer, setCustomer] = useState<Customer | null>(null);
  const [notFound, setNotFound] = useState(false);

  useEffect(() => {
    if (isNew) return;
    api.GET('/v1/b/{businessId}/customers/{customerId}/', { params: { path: { businessId, customerId } } })
      .then(({ data }) => (data ? setCustomer(data) : setNotFound(true)));
  }, [businessId, customerId, isNew]);

  if (notFound) return <Text style={{ padding: 16 }}>{strings.common.notFound}</Text>;
  if (!business || (!isNew && !customer)) return <ActivityIndicator style={{ marginTop: 24 }} />;
  const isOwner = business.role === 'owner';
  const open = (id: string) => router.replace({ pathname: '/admin/[businessId]/customers/[customerId]', params: { businessId, customerId: id } });

  return (
    <ScrollView contentContainerStyle={{ padding: 16, gap: 20, maxWidth: 560, width: '100%', alignSelf: 'center' }}>
      <Stack.Screen options={{ title: customer?.name ?? t.new }} />
      <CustomerForm key={customer?.id ?? 'new'} businessId={businessId} customer={customer ?? undefined} isOwner={isOwner}
        onSaved={(c) => (isNew ? open(c.id) : setCustomer(c))} onExisting={open} />
      {customer && <CustomerDocuments businessId={businessId} customer={customer} isOwner={isOwner} onChange={setCustomer} />}
    </ScrollView>
  );
}
