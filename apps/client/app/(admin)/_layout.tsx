import { Redirect, Stack } from 'expo-router';
import { ActivityIndicator, View } from 'react-native';
import { BackButton } from '../../components/BackButton';
import { useSession } from '../../lib/session';
import { useUI } from '../../ui';

// Everything under (admin) requires a session. A Stack gives each screen a header with the back arrow.
export default function AdminLayout() {
  const { session, loading } = useSession();
  const ui = useUI();
  if (loading) {
    return (
      <View style={{ flex: 1, justifyContent: 'center' }}>
        <ActivityIndicator />
      </View>
    );
  }
  if (!session) return <Redirect href="/login" />;
  return (
    <Stack screenOptions={{
      headerLeft: () => <BackButton />, headerBackVisible: false,
      headerShadowVisible: false, headerStyle: { backgroundColor: ui.bg }, headerTintColor: ui.accent,
      headerTitleStyle: { color: ui.label }, contentStyle: { backgroundColor: ui.bg },
    }} />
  );
}
