import { strings, term } from '@lb/shared';
import { Link, router, Stack } from 'expo-router';
import { ActivityIndicator, Button, Text, View } from 'react-native';
import { supabase } from '../../../lib/supabase';
import { useMe } from '../../../lib/useMe';

const t = strings.admin;

export default function AdminHome() {
  const { businesses, error, reload } = useMe();

  return (
    <View style={{ flex: 1, padding: 16, gap: 12 }}>
      <Stack.Screen options={{ title: t.title }} />
      {error ? (
        <>
          <Text accessibilityRole="alert">{error}</Text>
          <Button title={strings.common.retry} onPress={reload} />
        </>
      ) : businesses === null ? (
        <ActivityIndicator accessibilityLabel={strings.common.loading} />
      ) : businesses.length === 0 ? (
        <>
          <Text>{t.noBusinesses}</Text>
          <Button title={t.createBusiness} onPress={() => router.push('/onboarding')} />
        </>
      ) : (
        <>
          <Text accessibilityRole="header" style={{ fontSize: 18, fontWeight: '600' }}>{t.yourBusinesses}</Text>
          {businesses.map((b) => (
            <Link key={b.business_id} href={{ pathname: '/admin/[businessId]', params: { businessId: b.business_id } }}>
              <Text style={{ textDecorationLine: 'underline' }}>
                {b.name} · {term(b.vertical, 'catalog')} · {t.role[b.role]}
              </Text>
            </Link>
          ))}
        </>
      )}
      <Button title={t.signOut} onPress={() => supabase.auth.signOut()} />
    </View>
  );
}
