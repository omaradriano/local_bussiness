import type { components } from '@lb/api-client';
import { strings } from '@lb/shared';
import { Stack, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Button, ScrollView, Text, TextInput, View } from 'react-native';
import { api } from '../../../../../../lib/api';
import { errorText, fieldErrors } from '../../../../../../lib/errors';
import { useBusiness } from '../../../../../../lib/useMe';

type Unit = components['schemas']['Unit'];
type Status = components['schemas']['UnitStatus'];
const t = strings.units;
const STATUSES: Status[] = ['available', 'maintenance', 'retired'];

export default function Units() {
  const { businessId, itemId } = useLocalSearchParams<{ businessId: string; itemId: string }>();
  const { business } = useBusiness(businessId);
  const isOwner = business?.role === 'owner';
  const [units, setUnits] = useState<Unit[] | null>(null);
  const [code, setCode] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [codeError, setCodeError] = useState<string | null>(null);
  const path = { businessId, itemId };

  const load = useCallback(async () => {
    const { data, error, response } = await api.GET('/v1/b/{businessId}/catalog/{itemId}/units', { params: { path: { businessId, itemId } } });
    if (data) setUnits(data.units);
    else setError(errorText(error, response.status));
  }, [businessId, itemId]);

  useEffect(() => { load(); }, [load]);

  async function add() {
    setCodeError(null);
    const { data, error, response } = await api.POST('/v1/b/{businessId}/catalog/{itemId}/units', { params: { path }, body: { code } });
    if (data) {
      setUnits((prev) => [...(prev ?? []), data].sort((a, b) => a.code.localeCompare(b.code)));
      setCode('');
    } else setCodeError(fieldErrors(error).code ?? errorText(error, response.status));
  }

  async function setStatus(u: Unit, status: Status) {
    const { data, error, response } = await api.PATCH('/v1/b/{businessId}/catalog/{itemId}/units/{unitId}', {
      params: { path: { ...path, unitId: u.id } },
      body: { status },
    });
    if (data) setUnits((prev) => prev?.map((x) => (x.id === data.id ? data : x)) ?? null);
    else setError(errorText(error, response.status));
  }

  return (
    <ScrollView contentContainerStyle={{ padding: 16, gap: 12, maxWidth: 520, width: '100%', alignSelf: 'center' }}>
      <Stack.Screen options={{ title: t.title }} />
      <Text style={{ opacity: 0.7 }}>{t.hint}</Text>
      {error && <Text accessibilityRole="alert">{error}</Text>}
      {units === null && !error && <ActivityIndicator accessibilityLabel={strings.common.loading} />}
      {units?.length === 0 && <Text>{t.empty}</Text>}
      {units?.map((u) => (
        <View key={u.id} style={{ borderWidth: 1, borderRadius: 8, padding: 12, gap: 6 }}>
          <Text style={{ fontWeight: '600' }}>{u.code} · {t.status[u.status]}</Text>
          {isOwner && (
            <View style={{ flexDirection: 'row', gap: 16, flexWrap: 'wrap' }}>
              {STATUSES.filter((s) => s !== u.status).map((s) => (
                <Text key={s} accessibilityRole="button" onPress={() => setStatus(u, s)} style={{ textDecorationLine: 'underline' }}>
                  {t.status[s]}
                </Text>
              ))}
            </View>
          )}
        </View>
      ))}
      {isOwner && (
        <View style={{ gap: 6 }}>
          <Text>{t.code}</Text>
          <TextInput accessibilityLabel={t.code} autoCapitalize="characters" value={code} onChangeText={setCode} onSubmitEditing={add}
            style={{ borderWidth: 1, borderRadius: 6, padding: 10 }} />
          {codeError && <Text accessibilityRole="alert">{codeError}</Text>}
          <Button title={t.add} onPress={add} />
        </View>
      )}
    </ScrollView>
  );
}
