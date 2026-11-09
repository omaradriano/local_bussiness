import { Stack, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { Text, View } from 'react-native';
import { strings } from '@lb/shared';
import { api } from '../../../lib/api';

export default function PublicCatalog() {
  const { slug } = useLocalSearchParams<{ slug: string }>();
  // The business's name, not its slug («floreria-latidos»); the public data comes with its privacy notice.
  const [name, setName] = useState('');
  useEffect(() => {
    api.GET('/v1/public/businesses/{slug}/privacy', { params: { path: { slug } } }).then(({ data }) => setName(data?.name ?? ''));
  }, [slug]);
  return (
    <View style={{ flex: 1, padding: 16 }}>
      <Stack.Screen options={{ title: name }} />
      <Text>{strings.publicCatalog.placeholder}</Text>
    </View>
  );
}
