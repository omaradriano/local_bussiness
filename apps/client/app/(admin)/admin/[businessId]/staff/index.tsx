import type { components } from '@lb/api-client';
import { strings } from '@lb/shared';
import { router, Stack, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator } from 'react-native';
import { api } from '../../../../../lib/api';
import { errorText, fieldErrors } from '../../../../../lib/errors';
import { useBusiness } from '../../../../../lib/useMe';
import { Button, Cell, Field, Group, Screen, Text, useUI } from '../../../../../ui';

type Staff = components['schemas']['StaffMember'];
const t = strings.staff;

export default function StaffList() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const { business } = useBusiness(businessId);
  const isOwner = business?.role === 'owner';
  const ui = useUI();
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
    <Screen>
      <Stack.Screen options={{ title: t.title }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 8 }}>{t.title}</Text>
      <Text variant="subhead" color="labelSecondary" style={{ marginBottom: 20 }}>{t.hint}</Text>
      {staff === null && !error && <ActivityIndicator accessibilityLabel={strings.common.loading} />}
      {!!staff?.length && (
        <Group separatorInset={58}>
          {staff.map((p) => (
            <Cell key={p.id} icon="person" tint={p.active ? ui.success : ui.gray}
              label={`${p.name}${p.is_owner ? ` (${t.you})` : ''}${p.active ? '' : ` · ${t.inactive}`}`}
              sublabel={`${p.service_ids.length} servicios · ${p.hours.length ? `${new Set(p.hours.map((h) => h.weekday)).size} días` : 'sin horario'}`}
              onPress={() => router.push({ pathname: '/admin/[businessId]/staff/[staffId]', params: { businessId, staffId: p.id } })} />
          ))}
        </Group>
      )}
      {isOwner && (
        <Group label={t.add} padded>
          <Field label={t.name} value={name} onChangeText={setName} onSubmitEditing={add} error={error} />
          <Button icon="person-add" onPress={add}>{t.add}</Button>
        </Group>
      )}
    </Screen>
  );
}
