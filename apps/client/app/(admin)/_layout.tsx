import { Redirect, Stack } from 'expo-router';
import { ActivityIndicator, View } from 'react-native';
import { AdminNav, AdminSidebar } from '../../components/AdminNav';
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
  // On wide screens the business sections sit in a sidebar next to the screens.
  return (
    <View style={{ flex: 1, flexDirection: 'row', backgroundColor: ui.bg }}>
      <AdminSidebar />
      <View style={{ flex: 1 }}>
    <Stack screenOptions={{
      headerLeft: () => <BackButton />, headerRight: () => <AdminNav />, headerBackVisible: false,
      headerShadowVisible: false, headerStyle: { backgroundColor: ui.bg }, headerTintColor: ui.accent,
      headerTitleStyle: { color: ui.label }, contentStyle: { backgroundColor: ui.bg },
    }} />
      </View>
    </View>
  );
}
