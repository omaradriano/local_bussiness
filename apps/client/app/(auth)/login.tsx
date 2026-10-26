import { Stack } from 'expo-router';
import { Text, View } from 'react-native';
import { strings } from '@lb/shared';

export default function Login() {
  return (
    <View style={{ flex: 1, padding: 16 }}>
      <Stack.Screen options={{ title: strings.auth.loginTitle }} />
      <Text>{strings.auth.placeholder}</Text>
    </View>
  );
}
