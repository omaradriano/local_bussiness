import type { components } from '@lb/api-client';
import { type CustomInput, type ItemKind } from '@lb/shared';
import { useEffect, useState } from 'react';
import { Switch, View } from 'react-native';
import { api } from '../lib/api';
import { Chip, Chips, Field, Text } from '../ui';

export type CustomField = components['schemas']['CustomField'];

/** The active custom fields of one target ("item" | "unit") of the engine; [] while loading or without the module. */
export function useCustomFields(businessId: string, kind: ItemKind, target: CustomField['target'], enabled: boolean): CustomField[] {
  const [fields, setFields] = useState<CustomField[]>([]);
  useEffect(() => {
    if (!enabled) return setFields([]);
    api.GET('/v1/b/{businessId}/custom-fields', { params: { path: { businessId }, query: { kind } } })
      .then(({ data }) => setFields((data?.fields ?? []).filter((f) => f.active && f.target === target)));
  }, [businessId, kind, target, enabled]);
  return fields;
}

/** One input per field, by type: text, number, date (DD/MM/AAAA), list (chips) or yes/no (switch). */
export function CustomFieldInputs({ fields, inputs, onChange, errors, editable = true }: {
  fields: CustomField[]; inputs: Record<string, CustomInput>; onChange: (id: string, v: CustomInput) => void;
  errors?: Record<string, string>; editable?: boolean;
}) {
  return (
    <View style={{ gap: 4 }}>
      {fields.map((f) => {
        const label = f.required ? `${f.label} *` : f.label;
        const error = errors?.[f.id] ?? errors?.[`custom_values.${f.id}`];
        if (f.type === 'boolean') {
          return (
            <View key={f.id} style={{ flexDirection: 'row', alignItems: 'center', gap: 10, paddingVertical: 8 }}>
              <Text style={{ flex: 1 }}>{f.label}</Text>
              <Switch accessibilityLabel={f.label} value={inputs[f.id] === true} disabled={!editable} onValueChange={(v) => onChange(f.id, v)} />
            </View>
          );
        }
        if (f.type === 'select') {
          const value = inputs[f.id];
          return (
            <View key={f.id} style={{ gap: 6, marginBottom: 12 }}>
              <Text variant="footnote" color="labelSecondary">{label}</Text>
              <Chips>
                {f.options.map((o) => (
                  <Chip key={o} selected={value === o} onPress={() => editable && onChange(f.id, value === o ? '' : o)}>{o}</Chip>
                ))}
              </Chips>
              {error ? <Text variant="footnote" color="danger">{error}</Text> : null}
            </View>
          );
        }
        return (
          <Field key={f.id} label={label} value={String(inputs[f.id] ?? '')} editable={editable} onChangeText={(v) => onChange(f.id, v)}
            keyboardType={f.type === 'number' ? 'decimal-pad' : 'default'} placeholder={f.type === 'date' ? 'DD/MM/AAAA' : undefined} error={error} />
        );
      })}
    </View>
  );
}
