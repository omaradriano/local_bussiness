import type { components } from '@lb/api-client';
import { customInputOf, customValuesOf, customValueText, strings, type CustomFieldType, type CustomInput } from '@lb/shared';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, View } from 'react-native';
import { Button, Cell, Field, Group, Segmented, Text, useToast, useUI } from '../ui';
import { ConfirmDialog } from './ConfirmDialog';
import { CustomFieldInputs, type CustomField } from './CustomFieldInputs';
import { api } from '../lib/api';
import { errorText, fieldErrors } from '../lib/errors';

type Unit = components['schemas']['Unit'];
type Status = components['schemas']['UnitStatus'];
const t = strings.units;
const STATUSES: Status[] = ['available', 'maintenance', 'retired'];

/** The rental item's pieces (units) and their status, inside the item's edit screen. */
export function UnitsSection({ businessId, itemId, isOwner, fields = [] }: {
  businessId: string; itemId: string; isOwner: boolean; fields?: CustomField[]; // the unit's custom fields (serial number, condition…)
}) {
  const ui = useUI();
  const toast = useToast();
  const [units, setUnits] = useState<Unit[] | null>(null);
  const [editing, setEditing] = useState<{ id: string; code: string; custom: Record<string, CustomInput>; error?: string; errors?: Record<string, string> } | null>(null);
  const [removing, setRemoving] = useState<Unit | null>(null);
  const [busy, setBusy] = useState(false);
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

  function edit(u: Unit) {
    setEditing({ id: u.id, code: u.code, custom: Object.fromEntries(fields.map((f) => [f.id, customInputOf(f.type as CustomFieldType, u.custom_values[f.id])])) });
  }

  async function saveCode() {
    if (!editing) return;
    const cv = customValuesOf(fields, editing.custom);
    if (cv.errors) return setEditing({ ...editing, errors: cv.errors });
    const { data, error, response } = await api.PATCH('/v1/b/{businessId}/catalog/{itemId}/units/{unitId}', {
      params: { path: { ...path, unitId: editing.id } },
      body: { code: editing.code, ...(fields.length ? { custom_values: cv.values } : {}) },
    });
    if (!data) {
      const errs = fieldErrors(error);
      return setEditing({ ...editing, error: errs.code ?? (fields.length ? undefined : errorText(error, response.status)), errors: errs });
    }
    setUnits((prev) => prev?.map((x) => (x.id === data.id ? data : x)).sort((a, b) => a.code.localeCompare(b.code)) ?? null);
    setEditing(null);
  }

  async function remove() {
    if (!removing) return;
    setBusy(true);
    const { error, response } = await api.DELETE('/v1/b/{businessId}/catalog/{itemId}/units/{unitId}', { params: { path: { ...path, unitId: removing.id } } });
    setBusy(false);
    if (!response.ok) { setRemoving(null); return toast(errorText(error, response.status), 'error'); }
    setUnits((prev) => prev?.filter((x) => x.id !== removing.id) ?? null);
    setRemoving(null);
    toast(t.removed);
  }

  return (
    <Group label={t.title} footer={error ?? t.hint}>
      {units === null && !error ? <View style={{ padding: 16 }}><ActivityIndicator accessibilityLabel={strings.common.loading} /></View> : null}
      {units?.length === 0 ? <Cell label={t.empty} /> : null}
      {(units ?? []).map((u) => (
        <View key={u.id} style={{ paddingHorizontal: 16, paddingVertical: 12, gap: 8 }}>
          <View style={{ flexDirection: 'row', alignItems: 'center', gap: 10 }}>
            <View style={{ width: 10, height: 10, borderRadius: 5, backgroundColor: u.status === 'available' ? ui.success : u.status === 'maintenance' ? ui.warning : ui.gray }} />
            <Text style={{ flex: 1, fontVariant: ['tabular-nums'] }}>{u.code}</Text>
            <Text variant="footnote" color="labelSecondary">{t.status[u.status]}</Text>
          </View>
          {fields.some((f) => customValueText(f.type as CustomFieldType, u.custom_values[f.id])) && (
            <Text variant="footnote" color="labelSecondary">
              {fields.map((f) => [f.label, customValueText(f.type as CustomFieldType, u.custom_values[f.id])]).filter(([, v]) => v).map(([l, v]) => `${l}: ${v}`).join(' · ')}
            </Text>
          )}
          {isOwner && (
            <Segmented options={STATUSES.map((st) => ({ value: st, label: t.status[st] }))} value={u.status} onChange={(st) => st !== u.status && setStatus(u, st)} />
          )}
          {isOwner && editing?.id === u.id && (
            <View style={{ gap: 8 }}>
              <Field label={t.code} autoCapitalize="characters" value={editing.code} onChangeText={(code) => setEditing({ ...editing, code })}
                onSubmitEditing={saveCode} error={editing.error} />
              <CustomFieldInputs fields={fields} inputs={editing.custom} errors={editing.errors}
                onChange={(id, v) => setEditing({ ...editing, custom: { ...editing.custom, [id]: v } })} />
              <Button size="sm" variant="primary" onPress={saveCode}>{fields.length ? strings.common.save : t.saveCode}</Button>
            </View>
          )}
          {isOwner && editing?.id !== u.id && (
            <View style={{ flexDirection: 'row', gap: 8 }}>
              <Button size="sm" icon="create" onPress={() => edit(u)} style={{ flex: 1 }}>{fields.length ? t.edit : t.editCode}</Button>
              <Button size="sm" icon="trash" variant="plain" onPress={() => setRemoving(u)} style={{ flex: 1 }}>{t.remove}</Button>
            </View>
          )}
        </View>
      ))}
      {isOwner ? (
        <View style={{ padding: 16 }}>
          <Field label={t.code} autoCapitalize="characters" value={code} onChangeText={setCode} onSubmitEditing={add} error={codeError} />
          <Button icon="add" onPress={add}>{t.add}</Button>
        </View>
      ) : null}
      <ConfirmDialog open={!!removing} title={removing ? t.removeTitle(removing.code) : ''} body={t.removeBody} confirmLabel={t.remove}
        busy={busy} onConfirm={remove} onClose={() => setRemoving(null)} />
    </Group>
  );
}
