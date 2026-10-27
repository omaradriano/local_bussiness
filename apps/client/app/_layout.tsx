import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { SessionProvider } from '../lib/session';

export default function RootLayout() {
  return (
    <SessionProvider>
      <Stack>
        {/* The panel has its own Stack (and header with the back arrow); avoid a second header. */}
        <Stack.Screen name="(admin)" options={{ headerShown: false }} />
      </Stack>
      <StatusBar style="auto" />
    </SessionProvider>
  );
}
