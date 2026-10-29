import AsyncStorage from '@react-native-async-storage/async-storage';
import { Ionicons } from '@expo/vector-icons';
import { PLATFORM_CONTACT, strings } from '@lb/shared';
import { router } from 'expo-router';
import { useEffect, useState } from 'react';
import { Modal, Pressable, ScrollView, View } from 'react-native';
import { Button, Text, radius, useUI } from '../ui';

const SEEN = 'welcome-seen-v1'; // bump to show a new version of the text again
const w = strings.welcome;

/**
 * What the project is, before signing in or up: the 14-day trial, that it's in development and open
 * to feedback. Opens by itself the first time on each device; the link opens it again.
 */
export function Welcome() {
  const ui = useUI();
  const [open, setOpen] = useState(false);

  useEffect(() => {
    AsyncStorage.getItem(SEEN).then((v) => { if (!v) setOpen(true); }).catch(() => setOpen(true));
  }, []);

  function close() {
    setOpen(false);
    AsyncStorage.setItem(SEEN, '1').catch(() => {});
  }

  return (
    <>
      <Button variant="plain" size="sm" icon="information-circle" onPress={() => setOpen(true)} style={{ alignSelf: 'center', marginTop: 16 }}>{w.open}</Button>
      <Modal visible={open} transparent animationType="fade" onRequestClose={close}>
        <Pressable accessibilityLabel={strings.photos.close} onPress={close}
          style={{ flex: 1, backgroundColor: 'rgba(0,0,0,0.4)', justifyContent: 'center', alignItems: 'center', padding: 16 }}>
          {/* Stop taps inside the panel from closing it. */}
          <Pressable onPress={() => {}} style={{ width: '100%', maxWidth: 440, maxHeight: '90%', backgroundColor: ui.surface, borderRadius: radius.lg }}>
            <ScrollView contentContainerStyle={{ padding: 20, gap: 18 }}>
              <Text variant="title" accessibilityRole="header">{w.title}</Text>
              {w.sections.map((s) => (
                <View key={s.title} style={{ flexDirection: 'row', gap: 12 }}>
                  <Ionicons name={s.icon as keyof typeof Ionicons.glyphMap} size={24} color={ui.accent} style={{ marginTop: 2 }} />
                  <View style={{ flex: 1, gap: 4 }}>
                    <Text variant="headline">{s.title}</Text>
                    <Text color="labelSecondary">{s.body}</Text>
                  </View>
                </View>
              ))}
              <Text variant="subhead" style={{ color: ui.accent, marginLeft: 36, marginTop: -10 }} selectable>{PLATFORM_CONTACT}</Text>
              <Button variant="primary" onPress={close}>{w.done}</Button>
              <Button variant="plain" size="sm" onPress={() => { close(); router.push('/privacidad'); }}>{w.privacy}</Button>
            </ScrollView>
          </Pressable>
        </Pressable>
      </Modal>
    </>
  );
}
