import { installMode, type InstallMode } from '@lb/shared';
import { useEffect, useState } from 'react';
import { Platform } from 'react-native';

type PromptEvent = Event & { prompt: () => Promise<void>; userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }> };

// The browser fires beforeinstallprompt once, often before the app loads: index.html keeps it in
// window.__installPrompt, and later ones are caught here.
let deferred: PromptEvent | null = null;
const listeners = new Set<() => void>();
const notify = () => listeners.forEach((l) => l());
if (Platform.OS === 'web' && typeof window !== 'undefined') {
  deferred = (window as { __installPrompt?: PromptEvent }).__installPrompt ?? null;
  window.addEventListener('beforeinstallprompt', (e) => { e.preventDefault(); deferred = e as PromptEvent; notify(); });
  window.addEventListener('appinstalled', () => { deferred = null; notify(); });
}

function current(): InstallMode {
  if (Platform.OS !== 'web' || typeof window === 'undefined') return null;
  return installMode({
    standalone: window.matchMedia('(display-mode: standalone)').matches || (navigator as { standalone?: boolean }).standalone === true,
    canPrompt: deferred !== null,
    userAgent: navigator.userAgent,
    touchPoints: navigator.maxTouchPoints ?? 0,
  });
}

/** Whether "Instalar app" applies here and how; `prompt()` opens the browser's dialog (true if installed). */
export function useInstall(): { mode: InstallMode; prompt: () => Promise<boolean> } {
  const [mode, setMode] = useState(current);
  useEffect(() => {
    const l = () => setMode(current());
    listeners.add(l);
    return () => { listeners.delete(l); };
  }, []);
  async function prompt() {
    if (!deferred) return false;
    const e = deferred;
    deferred = null; // a prompt event can be used only once
    await e.prompt();
    const { outcome } = await e.userChoice;
    notify();
    return outcome === 'accepted';
  }
  return { mode, prompt };
}
