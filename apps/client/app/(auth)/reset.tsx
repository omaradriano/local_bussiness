import { strings, validateNewPassword } from '@lb/shared';
import { router, Stack } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, View } from 'react-native';
import { supabase } from '../../lib/supabase';
import { Button, Field, Notice, Screen, Text, useToast, useUI } from '../../ui';

const t = strings.auth;

/**
 * /reset: where the recovery email lands. Supabase reads the link (a recovery session), then the
 * owner sets their new password and goes into the panel.
 */
export default function ResetPassword() {
  const ui = useUI();
  const toast = useToast();
  const [state, setState] = useState<'checking' | 'ready' | 'invalid'>('checking');
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    // The session from the link arrives as PASSWORD_RECOVERY (or is already there on a reload).
    const { data } = supabase.auth.onAuthStateChange((event, session) => {
      if (event === 'PASSWORD_RECOVERY' || session) setState('ready');
    });
    supabase.auth.getSession().then(({ data: s }) => { if (s.session) setState('ready'); });
    // An expired or used link brings an error in the URL and no session.
    const timer = setTimeout(() => setState((s) => (s === 'checking' ? 'invalid' : s)), 4000);
    return () => { data.subscription.unsubscribe(); clearTimeout(timer); };
  }, []);

  async function save() {
    const invalid = validateNewPassword(password, confirm);
    if (invalid) return toast(invalid, 'error');
    setBusy(true);
    const { error } = await supabase.auth.updateUser({ password });
    setBusy(false);
    if (error) return toast(error.code === 'same_password' ? 'Usa una contraseña distinta a la anterior.' : error.message, 'error');
    toast(t.passwordSaved);
    router.replace('/admin');
  }

  return (
    <Screen>
      <Stack.Screen options={{ title: t.resetTitle, headerShadowVisible: false, headerStyle: { backgroundColor: ui.bg }, headerTitleStyle: { color: ui.label } }} />
      <View style={{ maxWidth: 420, width: '100%', alignSelf: 'center', paddingTop: 24 }}>
        <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 12 }}>{t.resetTitle}</Text>
        {state === 'checking' && (
          <View style={{ alignItems: 'center', gap: 8 }}>
            <ActivityIndicator />
            <Text color="labelSecondary">{t.checkingLink}</Text>
          </View>
        )}
        {state === 'invalid' && (
          <View style={{ gap: 12 }}>
            <Notice tone="warning">{t.invalidLink}</Notice>
            <Button onPress={() => router.replace('/login')}>{t.backToLogin}</Button>
          </View>
        )}
        {state === 'ready' && (
          <>
            <Text color="labelSecondary" style={{ marginBottom: 16 }}>{t.resetHint}</Text>
            <Field label={t.newPassword} secureTextEntry autoComplete="new-password" value={password} onChangeText={setPassword} />
            <Field label={t.confirmPassword} secureTextEntry autoComplete="new-password" value={confirm} onChangeText={setConfirm} onSubmitEditing={save} />
            <Button variant="primary" busy={busy} onPress={save}>{t.savePassword}</Button>
          </>
        )}
      </View>
    </Screen>
  );
}
