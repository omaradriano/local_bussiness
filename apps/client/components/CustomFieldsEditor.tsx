import type { components } from '@lb/api-client';
import { engineFor, strings, type CustomFieldType, type ItemKind } from '@lb/shared';
import { Ionicons } from '@expo/vector-icons';
import { useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Switch, View } from 'react-native';
import { api } from '../lib/api';
import { errorText, fieldErrors } from '../lib/errors';
import { Button, Chip, Chips, Field, Segmented, Text, radius, useToast, useUI } from '../ui';
import { ConfirmDialog } from './ConfirmDialog';

type CustomField = components['schemas']['CustomField'];
type Target = CustomField['target'];
const t = strings.settings.customFields;
const TYPES: CustomFieldType[] = ['text', 'number', 'select', 'date', 'boolean'];
const splitOptions = (s: string) => s.split(',').map((o) => o.trim()).filter(Boolean);

/**
 * The owner's custom fields for this business's catalog (module customFields): add one with its
 * name, type and, for lists, its options; per field, rename it, edit options, make it required,
 * hide it or delete it. Saved at once. The type and where it's filled don't change once created.
 */
export function CustomFieldsEditor({ businessId, kind, isOwner }: { businessId: string; kind: ItemKind; isOwner: boolean }) {
  const ui = useUI();
  const toast = useToast();
  const units = engineFor(kind).resource === 'units'; // «de cada unidad» only where the engine has units
  const [list, setList] = useState<CustomField[] | null>(null);
  const [editing, setEditing] = useState<{ id: string; label: string; options: string } | null>(null);
  const [removing, setRemoving] = useState<CustomField | null>(null);
  const [label, setLabel] = useState('');
  const [type, setType] = useState<CustomFieldType>('text');
  const [target, setTarget] = useState<Target>('item');
  const [options, setOptions] = useState('');
  const [required, setRequired] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api.GET('/v1/b/{businessId}/custom-fields', { params: { path: { businessId }, query: { kind } } })
      .then(({ data, error: err, response }) => (data ? setList(data.fields) : toast(errorText(err, response.status), 'error')));
  }, [businessId, kind]); // eslint-disable-line react-hooks/exhaustive-deps

  async function update(f: CustomField, body: components['schemas']['CustomFieldPatch']) {
    const { data, error: err, response } = await api.PATCH('/v1/b/{businessId}/custom-fields/{fieldId}', {
      params: { path: { businessId, fieldId: f.id } }, body,
    });
    if (!data) { const e = fieldErrors(err); return toast(e.label ?? e.options ?? errorText(err, response.status), 'error'); }
    setList((l) => l?.map((x) => (x.id === data.id ? data : x)) ?? null);
    setEditing(null);
    toast(t.saved);
  }

  async function add() {
    setBusy(true);
    setErrors({});
    const { data, error: err, response } = await api.POST('/v1/b/{businessId}/custom-fields', {
      params: { path: { businessId } },
      body: { kind, target: units ? target : 'item', label, type, required, ...(type === 'select' ? { options: splitOptions(options) } : {}) },
    });
    setBusy(false);
    if (!data) { setErrors(fieldErrors(err)); return toast(errorText(err, response.status), 'error'); }
    setList((l) => [...(l ?? []), data]);
    setLabel('');
    setOptions('');
    setRequired(false);
    toast(t.added);
  }

  async function remove() {
    if (!removing) return;
    setBusy(true);
    const { error: err, response } = await api.DELETE('/v1/b/{businessId}/custom-fields/{fieldId}', { params: { path: { businessId, fieldId: removing.id } } });
    setBusy(false);
    if (!response.ok) { setRemoving(null); return toast(errorText(err, response.status), 'error'); }
    setList((l) => l?.filter((x) => x.id !== removing.id) ?? null);
    setRemoving(null);
    toast(t.removed);
  }

  if (!list) return <ActivityIndicator />;
  return (
    <View style={{ backgroundColor: ui.surface, borderRadius: radius.sm, overflow: 'hidden', marginBottom: 6 }}>
      {list.map((f, i) => (
        <View key={f.id} style={{ paddingHorizontal: 16, paddingVertical: 12, gap: 8, borderTopWidth: i ? StyleSheet.hairlineWidth : 0, borderColor: ui.separator }}>
          {editing?.id === f.id ? (
            <View>
              <Field label={t.label} value={editing.label} onChangeText={(v) => setEditing({ ...editing, label: v })} autoFocus />
              {f.type === 'select' && (
                <Field label={t.options} hint={t.optionsHint} value={editing.options} onChangeText={(v) => setEditing({ ...editing, options: v })} />
              )}
              <View style={{ flexDirection: 'row', gap: 8 }}>
                <Button size="sm" variant="primary" style={{ flex: 1 }}
                  onPress={() => update(f, { label: editing.label, ...(f.type === 'select' ? { options: splitOptions(editing.options) } : {}) })}>
                  {strings.common.save}
                </Button>
                <Button size="sm" variant="plain" onPress={() => setEditing(null)}>{strings.common.cancel}</Button>
              </View>
            </View>
          ) : (
            <View style={{ flexDirection: 'row', alignItems: 'center', gap: 10 }}>
              <Ionicons name="list" size={20} color={f.active ? ui.accent : ui.labelSecondary} />
              <Pressable accessibilityRole="button" disabled={!isOwner} style={{ flex: 1 }}
                onPress={() => setEditing({ id: f.id, label: f.label, options: f.options.join(', ') })}>
                <Text style={{ opacity: f.active ? 1 : 0.6 }}>{f.label}</Text>
                <Text variant="footnote" color="labelSecondary">
                  {t.summary(t.types[f.type] ?? f.type, t.targets[f.target] ?? f.target, f.required)}
                  {f.type === 'select' ? ` · ${f.options.join(', ')}` : ''}
                </Text>
              </Pressable>
              <Switch accessibilityLabel={`${t.active}: ${f.label}`} value={f.active} disabled={!isOwner} onValueChange={(active) => update(f, { active })} />
            </View>
          )}
          {isOwner && editing?.id !== f.id && (
            <View style={{ flexDirection: 'row', alignItems: 'center', gap: 10, paddingLeft: 30 }}>
              <Text variant="subhead" color="labelSecondary" style={{ flex: 1 }}>{t.required}</Text>
              <Switch accessibilityLabel={`${t.required}: ${f.label}`} value={f.required} onValueChange={(req) => update(f, { required: req })} />
              <Button size="sm" variant="plain" icon="trash" onPress={() => setRemoving(f)}>{t.remove}</Button>
            </View>
          )}
        </View>
      ))}
      {list.length === 0 && <Text color="labelSecondary" style={{ padding: 16 }}>{t.empty}</Text>}
      {isOwner && (
        <View style={{ padding: 16, gap: 10, borderTopWidth: StyleSheet.hairlineWidth, borderColor: ui.separator }}>
          <Field label={t.label} placeholder={t.labelPlaceholder} value={label} onChangeText={setLabel} error={errors.label} />
          <Text variant="footnote" color="labelSecondary">{t.type}</Text>
          <Chips>
            {TYPES.map((ty) => <Chip key={ty} selected={type === ty} onPress={() => setType(ty)}>{t.types[ty]!}</Chip>)}
          </Chips>
          {type === 'select' && <Field label={t.options} hint={t.optionsHint} value={options} onChangeText={setOptions} error={errors.options} />}
          {units && (
            <>
              <Text variant="footnote" color="labelSecondary">{t.target}</Text>
              <Segmented options={(['item', 'unit'] as Target[]).map((v) => ({ value: v, label: t.targets[v]! }))} value={target} onChange={setTarget} />
            </>
          )}
          <View style={{ flexDirection: 'row', alignItems: 'center', gap: 10 }}>
            <Text variant="subhead" style={{ flex: 1 }}>{t.required}</Text>
            <Switch accessibilityLabel={t.required} value={required} onValueChange={setRequired} />
          </View>
          <Button icon="add" busy={busy} onPress={add}>{t.add}</Button>
        </View>
      )}
      <ConfirmDialog open={!!removing} title={removing ? t.removeTitle(removing.label) : ''} body={t.removeBody} confirmLabel={t.remove}
        busy={busy} onConfirm={remove} onClose={() => setRemoving(null)} />
    </View>
  );
}
