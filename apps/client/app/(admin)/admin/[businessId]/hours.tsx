import { shiftsToWeek, strings, weekToShifts, type DayRow } from '@lb/shared';
import { Stack, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator } from 'react-native';
import { HoursEditor } from '../../../../components/HoursEditor';
import { api } from '../../../../lib/api';
import { errorText } from '../../../../lib/errors';
import { useBusiness } from '../../../../lib/useMe';
import { Button, Notice, Screen, Text, useToast } from '../../../../ui';

const t = strings.hours;

export default function BusinessHours() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const { business } = useBusiness(businessId);
  const toast = useToast();
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
    if (response.ok) toast(t.saved);
    else toast(errorText(error, response.status), 'error');
  }

  const isOwner = business?.role === 'owner';
  return (
    <Screen>
      <Stack.Screen options={{ title: t.title }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 8 }}>{t.title}</Text>
      <Text variant="subhead" color="labelSecondary" style={{ marginBottom: 20 }}>{t.hint}</Text>
      {!week ? (message ? <Notice tone="warning">{message}</Notice> : <ActivityIndicator />) : (
        <>
          <HoursEditor week={week} onChange={setWeek} editable={isOwner} />
          {isOwner && <Button variant="primary" busy={busy} onPress={save}>{t.save}</Button>}
        </>
      )}
    </Screen>
  );
}
