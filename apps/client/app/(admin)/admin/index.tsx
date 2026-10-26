import { Link, Stack } from 'expo-router';
import { Text, View } from 'react-native';
import { strings, term, VERTICALS } from '@lb/shared';

export default function AdminHome() {
  return (
    <View style={{ flex: 1, padding: 16, gap: 12 }}>
      <Stack.Screen options={{ title: strings.admin.title }} />
      <Text>{strings.admin.placeholder}</Text>
      {VERTICALS.map((v) => (
        <Text key={v}>
          {v}: {term(v, 'booking')} / {term(v, 'catalog_item')}
        </Text>
      ))}
      <Link href="/login">{strings.auth.loginTitle}</Link>
      <Link href="/demo-unas">/demo-unas</Link>
    </View>
  );
}
