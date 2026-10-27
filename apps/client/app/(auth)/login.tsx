import { authErrorMessage, strings, validateCredentials } from '@lb/shared';
import { Redirect, Stack } from 'expo-router';
import { useState } from 'react';
import { Button, Pressable, Text, TextInput, View } from 'react-native';
import { useSession } from '../../lib/session';
import { supabase } from '../../lib/supabase';

const t = strings.auth;

export default function Login() {
  const { session } = useSession();
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
    <View style={{ flex: 1, padding: 16, gap: 12, maxWidth: 420, width: '100%', alignSelf: 'center' }}>
      <Stack.Screen options={{ title }} />
      <Text accessibilityRole="header" style={{ fontSize: 22, fontWeight: '600' }}>{title}</Text>
      <TextInput
        accessibilityLabel={t.email}
        placeholder={t.email}
        autoCapitalize="none"
        autoComplete="email"
        keyboardType="email-address"
        value={email}
        onChangeText={setEmail}
        style={{ borderWidth: 1, borderRadius: 6, padding: 10 }}
      />
      <TextInput
        accessibilityLabel={t.password}
        placeholder={t.password}
        secureTextEntry
        autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
        value={password}
        onChangeText={setPassword}
        onSubmitEditing={submit}
        style={{ borderWidth: 1, borderRadius: 6, padding: 10 }}
      />
      {message && <Text accessibilityRole="alert">{message}</Text>}
      <Button title={mode === 'login' ? t.submitLogin : t.submitSignUp} onPress={submit} disabled={busy} />
      <Pressable accessibilityRole="button" onPress={() => { setMode(mode === 'login' ? 'signup' : 'login'); setMessage(null); }}>
        <Text style={{ textDecorationLine: 'underline' }}>{mode === 'login' ? t.toSignUp : t.toLogin}</Text>
      </Pressable>
    </View>
  );
}
