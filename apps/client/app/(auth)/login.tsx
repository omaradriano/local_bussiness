import { authErrorMessage, strings, validateCredentials } from '@lb/shared';
import { Redirect, Stack } from 'expo-router';
import { useState } from 'react';
import { View } from 'react-native';
import { useSession } from '../../lib/session';
import { supabase } from '../../lib/supabase';
import { Button, Field, Notice, Screen, Text, useUI } from '../../ui';

const t = strings.auth;

export default function Login() {
  const { session } = useSession();
  const ui = useUI();
  const [mode, setMode] = useState<'login' | 'signup'>('login');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  if (session) return <Redirect href="/admin" />;

  async function submit() {
    const invalid = validateCredentials(email, password);
    if (invalid) return setMessage(invalid);
    setBusy(true);
    setMessage(null);
    const creds = { email: email.trim(), password };
    if (mode === 'login') {
      const { error } = await supabase.auth.signInWithPassword(creds);
      if (error) setMessage(authErrorMessage(error.code));
    } else {
      const { data, error } = await supabase.auth.signUp(creds);
      if (error) setMessage(authErrorMessage(error.code));
      else if (!data.session) setMessage(t.checkEmail); // confirmación por correo activa
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
        {message && <View style={{ marginBottom: 16 }}><Notice tone="warning">{message}</Notice></View>}
        <Button variant="primary" busy={busy} onPress={submit}>{mode === 'login' ? t.submitLogin : t.submitSignUp}</Button>
        <Button variant="plain" onPress={() => { setMode(mode === 'login' ? 'signup' : 'login'); setMessage(null); }} style={{ marginTop: 8 }}>
          {mode === 'login' ? t.toSignUp : t.toLogin}
        </Button>
      </View>
    </Screen>
  );
}
