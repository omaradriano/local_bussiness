import { slugify, strings, VERTICALS, type Vertical } from '@lb/shared';
import { Ionicons } from '@expo/vector-icons';
import { router, Stack } from 'expo-router';
import { useState } from 'react';
import { api } from '../../lib/api';
import { Button, Cell, Field, Group, Notice, Screen, Text, useUI } from '../../ui';

const t = strings.onboarding;

export default function Onboarding() {
  const ui = useUI();
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [slugEdited, setSlugEdited] = useState(false);
  const [vertical, setVertical] = useState<Vertical | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);

  function onName(v: string) {
    setName(v);
    if (!slugEdited) setSlug(slugify(v));
  }

  async function submit() {
    if (!vertical) return setErrors({ vertical: 'Elige un giro.' });
    setBusy(true);
    setErrors({});
    const { data, error } = await api.POST('/v1/businesses', { body: { name, slug, vertical } });
    setBusy(false);
    if (data) return router.replace('/admin');
    const details = error?.details as Record<string, string> | undefined;
    setErrors(details ?? { form: error?.message ?? strings.common.error });
  }

  return (
    <Screen>
      <Stack.Screen options={{ title: t.title }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 20 }}>{t.title}</Text>
      <Group padded>
        <Field label={t.name} value={name} onChangeText={onName} error={errors.name} />
        <Field label={t.slug} autoCapitalize="none" value={slug} error={errors.slug} hint={`${t.slugHint}${slug || '…'}`}
          onChangeText={(v) => { setSlugEdited(true); setSlug(v.toLowerCase()); }} />
      </Group>
      <Group label={t.vertical} footer={errors.vertical ?? t.templateNote}>
        {VERTICALS.map((v) => (
          <Cell key={v} label={t.verticals[v]} onPress={() => setVertical(v)}
            accessory={vertical === v ? <Ionicons name="checkmark" size={20} color={ui.accent} /> : undefined} />
        ))}
      </Group>
      {errors.form && <Notice tone="warning">{errors.form}</Notice>}
      <Button variant="primary" busy={busy} onPress={submit} style={{ marginTop: 8 }}>{t.submit}</Button>
    </Screen>
  );
}
