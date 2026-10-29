import { authErrorMessage, PRIVACY_VERSION, strings, validateCredentials } from '@lb/shared';
import { Redirect, router, Stack } from 'expo-router';
import { useState } from 'react';
import { Switch, View } from 'react-native';
import { useSession } from '../../lib/session';
import { supabase } from '../../lib/supabase';
import { Welcome } from '../../components/Welcome';
import { Button, Field, Screen, Text, useToast, useUI } from '../../ui';

const t = strings.auth;

export default function Login() {
  const { session } = useSession();
  const ui = useUI();
  const [mode, setMode] = useState<'login' | 'signup'>('login');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [privacy, setPrivacy] = useState(false);
  const toast = useToast();
  const [busy, setBusy] = useState(false);

  if (session) return <Redirect href="/admin" />;

  async function submit() {
    const invalid = validateCredentials(email, password);
    if (invalid) return toast(invalid, 'error');
    if (mode === 'signup' && !privacy) return toast(strings.privacy.required, 'error');
    setBusy(true);
    const creds = { email: email.trim(), password };
    if (mode === 'login') {
      const { error } = await supabase.auth.signInWithPassword(creds);
      if (error) toast(authErrorMessage(error.code), 'error');
    } else {
      // Record which privacy notice the owner accepted, and when (user metadata).
      const { data, error } = await supabase.auth.signUp({
        ...creds, options: { data: { privacy_version: PRIVACY_VERSION, privacy_accepted_at: new Date().toISOString() } },
      });
      if (error) toast(authErrorMessage(error.code), 'error');
      else if (!data.session) toast(t.checkEmail, 'info'); // confirmación por correo activa
    }
    setBusy(false);
  }

  const title = mode === 'login' ? t.loginTitle : t.signUpTitle;
  return (
    <Screen>
      <Stack.Screen options={{ title, headerShadowVisible: false, headerStyle: { backgroundColor: ui.bg }, headerTitleStyle: { color: ui.label } }} />
      <View style={{ maxWidth: 420, width: '100%', alignSelf: 'center', paddingTop: 24 }}>
        <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 24 }}>{title}</Text>
        <Field label={t.email} autoCapitalize="none" autoComplete="email" keyboardType="email-address" value={email} onChangeText={setEmail} />
        <Field label={t.password} secureTextEntry autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
          value={password} onChangeText={setPassword} onSubmitEditing={submit} />
        {mode === 'signup' && (
          <View style={{ marginBottom: 16 }}>
            <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12 }}>
              <Text style={{ flex: 1 }}>{strings.privacy.accept}</Text>
              <Switch accessibilityLabel={strings.privacy.accept} value={privacy} onValueChange={setPrivacy} />
            </View>
            <Button variant="plain" size="sm" onPress={() => router.push('/privacidad')} style={{ alignSelf: 'flex-start' }}>{strings.privacy.read}</Button>
          </View>
        )}
        <Button variant="primary" busy={busy} onPress={submit}>{mode === 'login' ? t.submitLogin : t.submitSignUp}</Button>
        <Button variant="plain" onPress={() => { setMode(mode === 'login' ? 'signup' : 'login'); }} style={{ marginTop: 8 }}>
          {mode === 'login' ? t.toSignUp : t.toLogin}
        </Button>
        <Welcome />
      </View>
    </Screen>
  );
}
