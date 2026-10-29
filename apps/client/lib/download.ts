import { Platform } from 'react-native';

/** Saves a text file (web: a download; the PWA is web). Returns false where it isn't available yet. */
export function downloadText(fileName: string, text: string, type = 'text/csv;charset=utf-8'): boolean {
  if (Platform.OS !== 'web') return false;
  const url = URL.createObjectURL(new Blob([text], { type }));
  const a = document.createElement('a');
  a.href = url;
  a.download = fileName;
  a.click();
  URL.revokeObjectURL(url);
  return true;
}

/** Lets the user pick a text file (web) and returns its content, or null if cancelled or not available. */
export function pickTextFile(accept = '.csv,text/csv'): Promise<string | null> {
  if (Platform.OS !== 'web') return Promise.resolve(null);
  return new Promise((resolve) => {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = accept;
    input.onchange = () => {
      const file = input.files?.[0];
      if (!file) return resolve(null);
      file.text().then(resolve, () => resolve(null));
    };
    input.click();
  });
}
