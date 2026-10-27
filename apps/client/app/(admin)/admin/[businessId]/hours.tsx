import { shiftsToWeek, strings, weekToShifts, type DayRow } from '@lb/shared';
import { Stack, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, Button, ScrollView, Text } from 'react-native';
import { HoursEditor } from '../../../../components/HoursEditor';
import { api } from '../../../../lib/api';
import { errorText } from '../../../../lib/errors';
import { useBusiness } from '../../../../lib/useMe';

const t = strings.hours;

export default function BusinessHours() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const { business } = useBusiness(businessId);
  const [week, setWeek] = useState<DayRow[] | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api.GET('/v1/b/{businessId}/hours', { params: { path: { businessId } } }).then(({ data, error, response }) => {
      if (data) setWeek(shiftsToWeek(data.hours));
      else setMessage(errorText(error, response.status));
    });
  }, [businessId]);

  async function save() {
    setBusy(true);
    const { error, response } = await api.PUT('/v1/b/{businessId}/hours', { params: { path: { businessId } }, body: { hours: weekToShifts(week!) } });
    setBusy(false);
    setMessage(response.ok ? t.saved : errorText(error, response.status));
  }

  const isOwner = business?.role === 'owner';
  return (
    <ScrollView contentContainerStyle={{ padding: 16, gap: 16, maxWidth: 560, width: '100%', alignSelf: 'center' }}>
      <Stack.Screen options={{ title: t.title }} />
      {!week ? (message ? <Text accessibilityRole="alert">{message}</Text> : <ActivityIndicator />) : (
        <>
          <HoursEditor week={week} onChange={setWeek} editable={isOwner} />
          {message && <Text accessibilityRole="alert">{message}</Text>}
          {isOwner && <Button title={t.save} onPress={save} disabled={busy} />}
        </>
      )}
    </ScrollView>
  );
}
