import type { components } from '@lb/api-client';
import { strings } from '@lb/shared';
import { Ionicons } from '@expo/vector-icons';
import { useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Switch, View } from 'react-native';
import { api } from '../lib/api';
import { errorText, fieldErrors } from '../lib/errors';
import { Button, Field, Text, radius, useToast, useUI } from '../ui';

type DocumentType = components['schemas']['DocumentType'];
const t = strings.settings.documents;

/**
 * The extra documents asked in every rental (e.g. "Pagaré"): turn each on/off, make it required for
 * the check-out, rename it, or add a new one. Changes apply to rentals created afterwards; saved at once.
 */
export function DocumentTypesEditor({ businessId, isOwner }: { businessId: string; isOwner: boolean }) {
  const ui = useUI();
  const toast = useToast();
  const [list, setList] = useState<DocumentType[] | null>(null);
  const [editing, setEditing] = useState<string | null>(null);
  const [editName, setEditName] = useState('');
  const [name, setName] = useState('');
  const [required, setRequired] = useState(true);
  const [error, setError] = useState<string | undefined>();
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api.GET('/v1/b/{businessId}/settings/documents', { params: { path: { businessId } } })
      .then(({ data, error: err, response }) => (data ? setList(data.documents) : toast(errorText(err, response.status), 'error')));
  }, [businessId]); // eslint-disable-line react-hooks/exhaustive-deps

  async function update(d: DocumentType, body: { name?: string; required?: boolean; active?: boolean }) {
    const { data, error: err, response } = await api.PATCH('/v1/b/{businessId}/settings/documents/{docTypeId}', {
      params: { path: { businessId, docTypeId: d.id } }, body,
    });
    if (!data) return toast(fieldErrors(err).name ?? errorText(err, response.status), 'error');
    setList((l) => l?.map((x) => (x.id === data.id ? data : x)) ?? null);
    setEditing(null);
    toast(t.saved);
  }

  async function add() {
    setBusy(true);
    setError(undefined);
    const { data, error: err, response } = await api.POST('/v1/b/{businessId}/settings/documents', {
      params: { path: { businessId } }, body: { name, required, active: true },
    });
    setBusy(false);
    if (!data) { setError(fieldErrors(err).name); return toast(errorText(err, response.status), 'error'); }
    setList((l) => [...(l ?? []), data]);
    setName('');
    toast(t.added);
  }

  if (!list) return <ActivityIndicator />;
  return (
    <View style={{ backgroundColor: ui.surface, borderRadius: radius.sm, overflow: 'hidden', marginBottom: 6 }}>
      {list.map((d, i) => (
        <View key={d.id} style={{ paddingHorizontal: 16, paddingVertical: 12, gap: 8, borderTopWidth: i ? StyleSheet.hairlineWidth : 0, borderColor: ui.separator }}>
          {editing === d.id ? (
            <View>
              <Field label={t.name} value={editName} onChangeText={setEditName} autoFocus />
              <View style={{ flexDirection: 'row', gap: 8 }}>
                <Button size="sm" variant="primary" onPress={() => update(d, { name: editName })} style={{ flex: 1 }}>{strings.common.save}</Button>
                <Button size="sm" variant="plain" onPress={() => setEditing(null)}>{strings.common.cancel}</Button>
              </View>
            </View>
          ) : (
            <View style={{ flexDirection: 'row', alignItems: 'center', gap: 10 }}>
              <Ionicons name="document-text" size={20} color={d.active ? ui.accent : ui.labelSecondary} />
              <Pressable accessibilityRole="button" accessibilityHint={t.renameHint} disabled={!isOwner} style={{ flex: 1 }}
                onPress={() => { setEditing(d.id); setEditName(d.name); }}>
                <Text style={{ opacity: d.active ? 1 : 0.6 }}>{d.name}</Text>
                <Text variant="footnote" color="labelSecondary">{d.active ? (d.required ? t.requiredOn : t.optionalOn) : t.off}</Text>
              </Pressable>
              <Switch accessibilityLabel={`${t.activeLabel}: ${d.name}`} value={d.active} disabled={!isOwner} onValueChange={(active) => update(d, { active })} />
            </View>
          )}
          {d.active && editing !== d.id && (
            <View style={{ flexDirection: 'row', alignItems: 'center', gap: 10, paddingLeft: 30 }}>
              <Text variant="subhead" color="labelSecondary" style={{ flex: 1 }}>{t.requiredLabel}</Text>
              <Switch accessibilityLabel={`${t.requiredLabel}: ${d.name}`} value={d.required} disabled={!isOwner} onValueChange={(req) => update(d, { required: req })} />
            </View>
          )}
        </View>
      ))}
      {list.length === 0 && <Text color="labelSecondary" style={{ padding: 16 }}>{t.empty}</Text>}
      {isOwner && (
        <View style={{ padding: 16, borderTopWidth: StyleSheet.hairlineWidth, borderColor: ui.separator }}>
          <Field label={t.newName} placeholder={t.namePlaceholder} value={name} onChangeText={setName} error={error} />
          <View style={{ flexDirection: 'row', alignItems: 'center', gap: 10, marginBottom: 12 }}>
            <Text variant="subhead" style={{ flex: 1 }}>{t.requiredLabel}</Text>
            <Switch accessibilityLabel={t.requiredLabel} value={required} onValueChange={setRequired} />
          </View>
          <Button icon="add" busy={busy} onPress={add}>{t.add}</Button>
        </View>
      )}
    </View>
  );
}
