import { strings, term } from '@lb/shared';
import { Link, Stack, useLocalSearchParams } from 'expo-router';
import { ActivityIndicator, Text, View } from 'react-native';
import { useBusiness } from '../../../../lib/useMe';

export default function BusinessHome() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const { business, businesses, error } = useBusiness(businessId);

  if (error) return <Text accessibilityRole="alert" style={{ padding: 16 }}>{error}</Text>;
  if (!businesses) return <ActivityIndicator style={{ marginTop: 24 }} accessibilityLabel={strings.common.loading} />;
  if (!business) return <Text style={{ padding: 16 }}>{strings.common.notFound}</Text>;

  return (
    <View style={{ flex: 1, padding: 16, gap: 12 }}>
      <Stack.Screen options={{ title: business.name }} />
      <Text accessibilityRole="header" style={{ fontSize: 22, fontWeight: '600' }}>{business.name}</Text>
      <Link href={{ pathname: '/admin/[businessId]/catalog', params: { businessId } }}>
        <Text style={{ textDecorationLine: 'underline', fontSize: 16 }}>{term(business.vertical, 'catalog')}</Text>
      </Link>
      <Link href="/admin">
        <Text style={{ opacity: 0.7 }}>← {strings.business.back}</Text>
      </Link>
    </View>
  );
}
