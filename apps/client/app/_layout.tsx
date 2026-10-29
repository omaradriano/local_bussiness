import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { Platform } from 'react-native';
import { initMonitoring, withMonitoring } from '../lib/monitor';
import { SessionProvider } from '../lib/session';
import { ToastProvider } from '../ui';

// PWA service worker (public/sw.js), only in web builds: in development it would cache Metro's bundles.
if (Platform.OS === 'web' && !__DEV__ && 'serviceWorker' in navigator) navigator.serviceWorker.register('/sw.js');

initMonitoring();

function RootLayout() {
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

export default withMonitoring(RootLayout);
