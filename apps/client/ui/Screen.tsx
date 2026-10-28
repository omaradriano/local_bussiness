import type { ReactNode } from 'react';
import { RefreshControl, ScrollView, StyleSheet, type ScrollViewProps } from 'react-native';

import { contentWidth, useUI } from './tokens';

/**
 * Scrollable screen body under a native large-title header. The automatic
 * content inset lets iOS place it below the translucent header and collapse
 * the large title on scroll.
 */
export function Screen({
  children,
  refreshing,
  onRefresh,
  ...props
}: ScrollViewProps & { children: ReactNode; refreshing?: boolean; onRefresh?: () => void }) {
  const ui = useUI();
  return (
    <ScrollView
      contentInsetAdjustmentBehavior="automatic"
      keyboardShouldPersistTaps="handled"
      keyboardDismissMode="interactive"
      style={{ backgroundColor: ui.bg }}
      contentContainerStyle={styles.content}
      refreshControl={onRefresh ? <RefreshControl refreshing={!!refreshing} onRefresh={onRefresh} /> : undefined}
      {...props}>
      {children}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  content: { padding: 16, paddingBottom: 40, width: '100%', maxWidth: contentWidth, alignSelf: 'center' },
});
