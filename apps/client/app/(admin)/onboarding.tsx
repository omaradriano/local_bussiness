import { slugify, strings, VERTICALS, type Vertical } from '@lb/shared';
import { router, Stack } from 'expo-router';
import { useState } from 'react';
import { Button, Pressable, Text, TextInput, View } from 'react-native';
import { api } from '../../lib/api';

const t = strings.onboarding;

export default function Onboarding() {
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

  const input = { borderWidth: 1, borderRadius: 6, padding: 10 } as const;
  return (
    <View style={{ flex: 1, padding: 16, gap: 10, maxWidth: 480, width: '100%', alignSelf: 'center' }}>
      <Stack.Screen options={{ title: t.title }} />
      <Text accessibilityRole="header" style={{ fontSize: 22, fontWeight: '600' }}>{t.title}</Text>

      <Text>{t.name}</Text>
      <TextInput accessibilityLabel={t.name} value={name} onChangeText={onName} style={input} />
      {errors.name && <Text accessibilityRole="alert">{errors.name}</Text>}

      <Text>{t.slug}</Text>
      <TextInput
        accessibilityLabel={t.slug}
        autoCapitalize="none"
        value={slug}
        onChangeText={(v) => { setSlugEdited(true); setSlug(v.toLowerCase()); }}
        style={input}
      />
      <Text style={{ opacity: 0.7 }}>{t.slugHint}{slug || '…'}</Text>
      {errors.slug && <Text accessibilityRole="alert">{errors.slug}</Text>}

      <Text>{t.vertical}</Text>
      <View accessibilityRole="radiogroup" style={{ gap: 6 }}>
        {VERTICALS.map((v) => (
          <Pressable
            key={v}
            accessibilityRole="radio"
            accessibilityState={{ checked: vertical === v }}
            onPress={() => setVertical(v)}
            style={{ borderWidth: vertical === v ? 2 : 1, borderRadius: 6, padding: 10 }}
          >
            <Text>{t.verticals[v]}</Text>
          </Pressable>
        ))}
      </View>
      {errors.vertical && <Text accessibilityRole="alert">{errors.vertical}</Text>}
      {errors.form && <Text accessibilityRole="alert">{errors.form}</Text>}

      <Text style={{ opacity: 0.7 }}>{t.templateNote}</Text>
      <Button title={t.submit} onPress={submit} disabled={busy} />
    </View>
  );
}
