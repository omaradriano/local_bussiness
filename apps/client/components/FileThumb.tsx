import { Ionicons } from '@expo/vector-icons';
import { strings } from '@lb/shared';
import { Linking, Pressable, View } from 'react-native';
import { Text, useUI } from '../ui';
import { ZoomableImage } from './ZoomableImage';

const isPdf = (key: string) => key.toLowerCase().endsWith('.pdf');

/** A stored file: photos open enlarged, PDFs open in the browser/viewer. `url` is a short-lived signed URL. */
export function FileThumb({ fileKey, url, size = 80, label }: { fileKey: string; url?: string; size?: number; label: string }) {
  const ui = useUI();
  const box = { width: size, height: size, borderRadius: 10 };
  if (!url) return <View style={[box, { backgroundColor: ui.surface }]} />;
  if (!isPdf(fileKey)) return <ZoomableImage source={url} style={box} contentFit="cover" accessibilityLabel={label} />;
  return (
    <Pressable accessibilityRole="link" accessibilityLabel={`${label} · ${strings.files.openPdf}`} onPress={() => Linking.openURL(url)}
      style={[box, { backgroundColor: ui.surface, alignItems: 'center', justifyContent: 'center', gap: 2 }]}>
      <Ionicons name="document-text" size={size / 2.6} color={ui.danger} />
      <Text variant="caption" style={{ fontWeight: '700', color: ui.danger }}>PDF</Text>
    </Pressable>
  );
}
