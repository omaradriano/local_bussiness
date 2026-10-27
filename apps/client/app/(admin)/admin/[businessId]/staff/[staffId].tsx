import type { components } from '@lb/api-client';
import { shiftsToWeek, strings, weekToShifts, type DayRow } from '@lb/shared';
import { Stack, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, Button, Pressable, ScrollView, Text, TextInput, View } from 'react-native';
import { HoursEditor } from '../../../../../components/HoursEditor';
import { api } from '../../../../../lib/api';
import { errorText, fieldErrors } from '../../../../../lib/errors';
import { useBusiness } from '../../../../../lib/useMe';

type Staff = components['schemas']['StaffMember'];
type Item = components['schemas']['CatalogItem'];
const t = strings.staff;

export default function StaffDetail() {
  const { businessId, staffId } = useLocalSearchParams<{ businessId: string; staffId: string }>();
  const { business } = useBusiness(businessId);
  const isOwner = business?.role === 'owner';
  const [person, setPerson] = useState<Staff | null>(null);
  const [services, setServices] = useState<Item[]>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [name, setName] = useState('');
  const [week, setWeek] = useState<DayRow[] | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const path = { businessId, staffId };

  useEffect(() => {
    Promise.all([
      api.GET('/v1/b/{businessId}/staff', { params: { path: { businessId } } }),
      api.GET('/v1/b/{businessId}/catalog', { params: { path: { businessId } } }),
    ]).then(([st, cat]) => {
      const p = st.data?.staff.find((s) => s.id === staffId);
      if (!p) return setMessage(st.data ? strings.common.notFound : errorText(st.error, st.response.status));
      setPerson(p);
      setName(p.name);
      setSelected(new Set(p.service_ids));
      setWeek(shiftsToWeek(p.hours));
      setServices(cat.data?.items.filter((i) => i.kind === 'service') ?? []);
    });
  }, [businessId, staffId]);

  if (!person || !week) return message ? <Text style={{ padding: 16 }}>{message}</Text> : <ActivityIndicator style={{ marginTop: 24 }} />;

  function toggle(id: string) {
    setSelected((prev) => { const n = new Set(prev); if (n.has(id)) n.delete(id); else n.add(id); return n; });
  }

  // Name/active, services and hours are three calls; stop at the first one that fails.
  async function save(active = person!.active) {
    setBusy(true);
    setMessage(null);
    const steps = [
      () => api.PATCH('/v1/b/{businessId}/staff/{staffId}/', { params: { path }, body: { name, active } }),
      () => api.PUT('/v1/b/{businessId}/staff/{staffId}/services', { params: { path }, body: { service_ids: [...selected] } }),
      () => api.PUT('/v1/b/{businessId}/staff/{staffId}/hours', { params: { path }, body: { hours: weekToShifts(week!) } }),
    ];
    for (const step of steps) {
      const { error, response } = await step();
      if (!response.ok) {
        setBusy(false);
        return setMessage(fieldErrors(error).name ?? errorText(error, response.status));
      }
    }
    setBusy(false);
    setPerson({ ...person!, name, active });
    setMessage(t.saved);
  }

  return (
    <ScrollView contentContainerStyle={{ padding: 16, gap: 14, maxWidth: 560, width: '100%', alignSelf: 'center' }}>
      <Stack.Screen options={{ title: person.name }} />
      <Text>{t.name}</Text>
      <TextInput accessibilityLabel={t.name} editable={isOwner} value={name} onChangeText={setName} style={{ borderWidth: 1, borderRadius: 6, padding: 10 }} />

      <Text style={{ fontWeight: '600' }}>{t.services}</Text>
      <View style={{ gap: 6 }}>
        {services.map((it) => (
          <Pressable key={it.id} accessibilityRole="checkbox" accessibilityState={{ checked: selected.has(it.id), disabled: !isOwner }}
            disabled={!isOwner} onPress={() => toggle(it.id)}
            style={{ flexDirection: 'row', gap: 8, alignItems: 'center' }}>
            <Text style={{ width: 20 }}>{selected.has(it.id) ? '☑' : '☐'}</Text>
            <Text>{it.name}</Text>
          </Pressable>
        ))}
      </View>

      <Text style={{ fontWeight: '600' }}>{t.hours}</Text>
      <HoursEditor week={week} onChange={setWeek} editable={isOwner} />

      {message && <Text accessibilityRole="alert">{message}</Text>}
      {isOwner && (
        <View style={{ gap: 8 }}>
          <Button title={t.save} onPress={() => save()} disabled={busy} />
          {!person.is_owner && (
            <Button title={person.active ? t.deactivate : t.activate} onPress={() => save(!person.active)} disabled={busy} />
          )}
        </View>
      )}
    </ScrollView>
  );
}
