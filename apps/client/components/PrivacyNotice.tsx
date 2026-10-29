import { strings, type NoticeSection } from '@lb/shared';
import { Stack } from 'expo-router';
import { View } from 'react-native';
import { Screen, Text, useUI } from '../ui';

/** A legal page (privacy notice, terms): title and its sections, readable without a session. */
export function PrivacyNotice({ title, sections, version, headerTitle = strings.privacy.title }: {
  title: string; sections: NoticeSection[]; version: string; headerTitle?: string;
}) {
  const ui = useUI();
  return (
    <Screen>
      <Stack.Screen options={{ title: headerTitle, headerShadowVisible: false, headerStyle: { backgroundColor: ui.bg },
        headerTitleStyle: { color: ui.label }, headerTintColor: ui.accent }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 4 }}>{title}</Text>
      <Text variant="footnote" color="labelSecondary" style={{ marginBottom: 24 }}>{strings.privacy.version(version)}</Text>
      {sections.map((s) => (
        <View key={s.title} style={{ marginBottom: 20, gap: 6 }}>
          <Text variant="headline" accessibilityRole="header">{s.title}</Text>
          {s.body.map((p) => <Text key={p} style={{ lineHeight: 24 }}>{p}</Text>)}
        </View>
      ))}
    </Screen>
  );
}
