import type { Membership } from '@lb/api-client';
import { strings, term } from '@lb/shared';
import { router, Stack } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Button, Text, View } from 'react-native';
import { api } from '../../../lib/api';
import { supabase } from '../../../lib/supabase';

const t = strings.admin;

export default function AdminHome() {
  const [businesses, setBusinesses] = useState<Membership[] | null>(null);
  const [failed, setFailed] = useState(false);

  const load = useCallback(async () => {
    setFailed(false);
    const { data, error } = await api.GET('/v1/me');
    if (error || !data) return setFailed(true);
    setBusinesses(data.businesses);
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <View style={{ flex: 1, padding: 16, gap: 12 }}>
      <Stack.Screen options={{ title: t.title }} />
      {failed ? (
        <>
          <Text accessibilityRole="alert">{strings.common.error}</Text>
          <Button title={strings.common.retry} onPress={load} />
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
            <Text key={b.business_id}>
              {b.name} · {term(b.vertical, 'booking')}s · {t.role[b.role]}
            </Text>
          ))}
        </>
      )}
      <Button title={t.signOut} onPress={() => supabase.auth.signOut()} />
    </View>
  );
}
