import { authErrorMessage, PRIVACY_VERSION, strings, TERMS_VERSION, validateCredentials, validateEmail } from '@lb/shared';
import * as Linking from 'expo-linking';
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
  const [mode, setMode] = useState<'login' | 'signup' | 'forgot'>('login');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [privacy, setPrivacy] = useState(false);
  const toast = useToast();
  const [busy, setBusy] = useState(false);

  if (session) return <Redirect href="/admin" />;

  // Recovery: Supabase emails a link back to /reset, where the new password is set.
  async function sendLink() {
    const badEmail = validateEmail(email);
    if (badEmail) return toast(badEmail, 'error');
    setBusy(true);
    const { error } = await supabase.auth.resetPasswordForEmail(email.trim(), { redirectTo: Linking.createURL('/reset') });
    setBusy(false);
    // Same answer whether the account exists or not (don't reveal who has an account).
    if (error && error.code?.startsWith('over_')) return toast(authErrorMessage(error.code), 'error');
    toast(t.linkSent, 'info');
    setMode('login');
  }

  async function submit() {
    if (mode === 'forgot') return sendLink();
    const invalid = validateCredentials(email, password);
    if (invalid) return toast(invalid, 'error');
    if (mode === 'signup' && !privacy) return toast(strings.privacy.required, 'error');
    setBusy(true);
    const creds = { email: email.trim(), password };
    if (mode === 'login') {
      const { error } = await supabase.auth.signInWithPassword(creds);
      if (error) toast(authErrorMessage(error.code), 'error');
    } else {
      // Record which privacy notice and terms the owner accepted, and when (user metadata).
      const accepted = new Date().toISOString();
      const { data, error } = await supabase.auth.signUp({
        ...creds, options: { data: { privacy_version: PRIVACY_VERSION, privacy_accepted_at: accepted, terms_version: TERMS_VERSION, terms_accepted_at: accepted } },
      });
      if (error) toast(authErrorMessage(error.code), 'error');
      else if (!data.session) toast(t.checkEmail, 'info'); // confirmación por correo activa
    }
    setBusy(false);
  }

  const title = mode === 'login' ? t.loginTitle : mode === 'signup' ? t.signUpTitle : t.forgotTitle;
  return (
    <Screen>
      <Stack.Screen options={{ title, headerShadowVisible: false, headerStyle: { backgroundColor: ui.bg }, headerTitleStyle: { color: ui.label } }} />
      <View style={{ maxWidth: 420, width: '100%', alignSelf: 'center', paddingTop: 24 }}>
        <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 24 }}>{title}</Text>
        {mode === 'forgot' && <Text color="labelSecondary" style={{ marginBottom: 16 }}>{t.forgotHint}</Text>}
        <Field label={t.email} autoCapitalize="none" autoComplete="email" keyboardType="email-address" value={email} onChangeText={setEmail}
          onSubmitEditing={mode === 'forgot' ? submit : undefined} />
        {mode !== 'forgot' && (
          <Field label={t.password} secureTextEntry autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
            value={password} onChangeText={setPassword} onSubmitEditing={submit} />
        )}
        {mode === 'signup' && (
          <View style={{ marginBottom: 16 }}>
            <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12 }}>
              <Text style={{ flex: 1 }}>{strings.privacy.accept}</Text>
              <Switch accessibilityLabel={strings.privacy.accept} value={privacy} onValueChange={setPrivacy} />
            </View>
            <View style={{ flexDirection: 'row', flexWrap: 'wrap' }}>
              <Button variant="plain" size="sm" onPress={() => router.push('/terminos')}>{strings.privacy.readTerms}</Button>
              <Button variant="plain" size="sm" onPress={() => router.push('/privacidad')}>{strings.privacy.read}</Button>
            </View>
          </View>
        )}
        <Button variant="primary" busy={busy} onPress={submit}>{mode === 'login' ? t.submitLogin : mode === 'signup' ? t.submitSignUp : t.sendLink}</Button>
        {mode === 'login' && <Button variant="plain" size="sm" onPress={() => setMode('forgot')} style={{ marginTop: 8 }}>{t.forgot}</Button>}
        <Button variant="plain" onPress={() => { setMode(mode === 'login' ? 'signup' : 'login'); }} style={{ marginTop: 8 }}>
          {mode === 'login' ? t.toSignUp : mode === 'signup' ? t.toLogin : t.backToLogin}
        </Button>
        <Welcome />
      </View>
    </Screen>
  );
}
