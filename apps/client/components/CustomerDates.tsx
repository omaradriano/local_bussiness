import type { components } from '@lb/api-client';
import { strings } from '@lb/shared';
import { Ionicons } from '@expo/vector-icons';
import { useEffect, useState } from 'react';
import { Pressable, View } from 'react-native';
import { api } from '../lib/api';
import { errorText, fieldErrors } from '../lib/errors';
import { Button, Cell, Field, Group, useToast, useUI } from '../ui';

type CustomerDate = components['schemas']['CustomerDate'];
const t = strings.customers.dates;

/** «14 mar» for a month and day (no year: it comes back every year). */
export const monthDay = (month: number, day: number) => new Date(2024, month - 1, day).toLocaleDateString('es-MX', { day: 'numeric', month: 'short' });

/** A customer's important dates (birthdays, anniversaries); Hoy reminds them a week ahead. */
export function CustomerDates({ businessId, customerId }: { businessId: string; customerId: string }) {
  const ui = useUI();
  const toast = useToast();
  const [dates, setDates] = useState<CustomerDate[]>([]);
  const [label, setLabel] = useState('');
  const [when, setWhen] = useState('');
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const path = { businessId, customerId };

  useEffect(() => {
    api.GET('/v1/b/{businessId}/customers/{customerId}/dates', { params: { path: { businessId, customerId } } }).then(({ data }) => setDates(data?.dates ?? []));
  }, [businessId, customerId]);

  async function add() {
    const m = when.trim().match(/^(\d{1,2})\s*[/-]\s*(\d{1,2})$/); // día/mes
    if (!m) return setErrors({ day: t.badDate });
    setBusy(true);
    const { data, error, response } = await api.POST('/v1/b/{businessId}/customers/{customerId}/dates', {
      params: { path }, body: { label, day: Number(m[1]), month: Number(m[2]) },
    });
    setBusy(false);
    if (!data) { setErrors(fieldErrors(error)); return toast(errorText(error, response.status), 'error'); }
    setErrors({});
    setLabel(''); setWhen('');
    setDates((d) => [...d, data].sort((a, b) => a.month - b.month || a.day - b.day));
  }

  async function remove(id: string) {
    const { error, response } = await api.DELETE('/v1/b/{businessId}/customers/{customerId}/dates/{dateId}', { params: { path: { ...path, dateId: id } } });
    if (!response.ok) return toast(errorText(error, response.status), 'error');
    setDates((d) => d.filter((x) => x.id !== id));
  }

  return (
    <View style={{ gap: 12 }}>
      <Group label={t.title} footer={t.hint}>
        {dates.length === 0 && <Cell label={t.none} />}
        {dates.map((d) => (
          <Cell key={d.id} icon="calendar" tint={ui.purple} label={d.label} value={monthDay(d.month, d.day)}
            accessory={<Pressable accessibilityRole="button" accessibilityLabel={t.remove(d.label)} hitSlop={10} onPress={() => remove(d.id)}>
              <Ionicons name="trash-outline" size={18} color={ui.danger} />
            </Pressable>} />
        ))}
      </Group>
      <Group padded>
        <Field label={t.label} placeholder={t.labelPlaceholder} value={label} onChangeText={setLabel} maxLength={80} error={errors.label} />
        <Field label={t.when} placeholder="14/03" value={when} onChangeText={setWhen} keyboardType="numbers-and-punctuation" error={errors.day ?? errors.month} />
        <Button icon="add-circle" busy={busy} disabled={!label.trim() || !when.trim()} onPress={add}>{t.add}</Button>
      </Group>
    </View>
  );
}
