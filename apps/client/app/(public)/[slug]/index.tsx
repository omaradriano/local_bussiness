import { Stack, useLocalSearchParams } from 'expo-router';
import { Text, View } from 'react-native';
import { strings } from '@lb/shared';

export default function PublicCatalog() {
  const { slug } = useLocalSearchParams<{ slug: string }>();
  return (
    <View style={{ flex: 1, padding: 16 }}>
      <Stack.Screen options={{ title: slug }} />
      <Text>{strings.publicCatalog.placeholder}</Text>
    </View>
  );
}
