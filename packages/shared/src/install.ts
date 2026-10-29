export type InstallMode = 'prompt' | 'ios' | null;

/**
 * How the PWA can be installed here: the browser's own prompt (Android, Chrome, Edge), the steps
 * of the Share menu (iPhone/iPad, no prompt), or not at all (already installed, or unsupported).
 */
export function installMode({ standalone, canPrompt, userAgent, touchPoints }: {
  standalone: boolean; canPrompt: boolean; userAgent: string; touchPoints: number;
}): InstallMode {
  if (standalone) return null;
  if (canPrompt) return 'prompt';
  // iPadOS reports itself as a Mac; touch tells them apart.
  const ios = /iPhone|iPad|iPod/.test(userAgent) || (/Macintosh/.test(userAgent) && touchPoints > 1);
  return ios ? 'ios' : null;
}
