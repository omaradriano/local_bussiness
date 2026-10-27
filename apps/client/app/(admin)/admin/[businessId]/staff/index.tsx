import type { components } from '@lb/api-client';
import { strings } from '@lb/shared';
import { Link, Stack, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator, Button, ScrollView, Text, TextInput, View } from 'react-native';
import { api } from '../../../../../lib/api';
import { errorText, fieldErrors } from '../../../../../lib/errors';
import { useBusiness } from '../../../../../lib/useMe';

type Staff = components['schemas']['StaffMember'];
const t = strings.staff;

export default function StaffList() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const { business } = useBusiness(businessId);
  const isOwner = business?.role === 'owner';
  const [staff, setStaff] = useState<Staff[] | null>(null);
  const [name, setName] = useState('');
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    const { data, error, response } = await api.GET('/v1/b/{businessId}/staff', { params: { path: { businessId } } });
    if (data) setStaff(data.staff);
    else setError(errorText(error, response.status));
  }, [businessId]);

  useFocusEffect(useCallback(() => { load(); }, [load]));

  async function add() {
    setError(null);
    const { data, error, response } = await api.POST('/v1/b/{businessId}/staff', { params: { path: { businessId } }, body: { name } });
    if (data) { setStaff((prev) => [...(prev ?? []), data]); setName(''); }
    else setError(fieldErrors(error).name ?? errorText(error, response.status));
  }

  return (
    <ScrollView contentContainerStyle={{ padding: 16, gap: 12, maxWidth: 520, width: '100%', alignSelf: 'center' }}>
      <Stack.Screen options={{ title: t.title }} />
      <Text style={{ opacity: 0.7 }}>{t.hint}</Text>
      {staff === null && !error && <ActivityIndicator accessibilityLabel={strings.common.loading} />}
      {staff?.map((s) => (
        <Link key={s.id} href={{ pathname: '/admin/[businessId]/staff/[staffId]', params: { businessId, staffId: s.id } }}>
          <View style={{ borderWidth: 1, borderRadius: 8, padding: 12, opacity: s.active ? 1 : 0.55 }}>
            <Text style={{ fontWeight: '600' }}>
              {s.name}{s.is_owner ? ` (${t.you})` : ''}{s.active ? '' : ` · ${t.inactive}`}
            </Text>
            <Text>{s.service_ids.length} servicios · {s.hours.length ? `${new Set(s.hours.map((h) => h.weekday)).size} días` : 'sin horario'}</Text>
          </View>
        </Link>
      ))}
      {error && <Text accessibilityRole="alert">{error}</Text>}
      {isOwner && (
        <View style={{ gap: 6 }}>
          <Text>{t.name}</Text>
          <TextInput accessibilityLabel={t.name} value={name} onChangeText={setName} onSubmitEditing={add} style={{ borderWidth: 1, borderRadius: 6, padding: 10 }} />
          <Button title={t.add} onPress={add} />
        </View>
      )}
    </ScrollView>
  );
}
