import { shiftsToWeek, strings, weekToShifts, type DayRow } from '@lb/shared';
import { Stack, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, View } from 'react-native';
import { HoursEditor } from '../../../../components/HoursEditor';
import { api } from '../../../../lib/api';
import { errorText } from '../../../../lib/errors';
import { useBusiness } from '../../../../lib/useMe';
import { Button, Notice, Screen, Text } from '../../../../ui';

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
    <Screen>
      <Stack.Screen options={{ title: t.title }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 8 }}>{t.title}</Text>
      <Text variant="subhead" color="labelSecondary" style={{ marginBottom: 20 }}>{t.hint}</Text>
      {!week ? (message ? <Notice tone="warning">{message}</Notice> : <ActivityIndicator />) : (
        <>
          <HoursEditor week={week} onChange={setWeek} editable={isOwner} />
          {message && <View style={{ marginBottom: 12 }}><Notice tone={message === t.saved ? 'success' : 'warning'}>{message}</Notice></View>}
          {isOwner && <Button variant="primary" busy={busy} onPress={save}>{t.save}</Button>}
        </>
      )}
    </Screen>
  );
}
