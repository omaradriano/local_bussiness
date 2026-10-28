import type { components } from '@lb/api-client';
import { strings } from '@lb/shared';
import { router, Stack, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, View } from 'react-native';
import { BookingHistory } from '../../../../../components/BookingHistory';
import { CustomerDocuments, CustomerForm } from '../../../../../components/CustomerForm';
import { api } from '../../../../../lib/api';
import { useBusiness } from '../../../../../lib/useMe';
import { Screen, Text } from '../../../../../ui';

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
    <Screen>
      <Stack.Screen options={{ title: customer?.name ?? t.new }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 20 }}>{customer?.name ?? t.new}</Text>
      <CustomerForm key={customer?.id ?? 'new'} businessId={businessId} customer={customer ?? undefined} isOwner={isOwner}
        onSaved={(c) => (isNew ? open(c.id) : setCustomer(c))} onExisting={open} />
      {customer && <View style={{ marginTop: 28 }}><CustomerDocuments businessId={businessId} customer={customer} isOwner={isOwner} onChange={setCustomer} /></View>}
      {customer && (
        <View style={{ marginTop: 28 }}>
          <Text variant="title" style={{ marginBottom: 12 }}>{strings.history.customerRentals}</Text>
          <BookingHistory businessId={businessId} customerId={customer.id} compact />
        </View>
      )}
    </Screen>
  );
}
