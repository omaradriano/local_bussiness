import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { SessionProvider } from '../lib/session';
import { ToastProvider } from '../ui';

export default function RootLayout() {
  return (
    <SessionProvider>
      <ToastProvider>
        <Stack>
          {/* The panel has its own Stack (and header with the back arrow); avoid a second header. */}
          <Stack.Screen name="(admin)" options={{ headerShown: false }} />
        </Stack>
        <StatusBar style="auto" />
      </ToastProvider>
    </SessionProvider>
  );
}
