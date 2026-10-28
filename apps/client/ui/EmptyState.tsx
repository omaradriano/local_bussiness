import type { ReactNode } from 'react';
import { StyleSheet, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';

import { Text } from './Text';
import { useUI } from './tokens';

export function EmptyState({
  icon,
  title,
  children,
  action,
}: {
  icon?: keyof typeof Ionicons.glyphMap;
  title: string;
  children?: string;
  action?: ReactNode;
}) {
  const ui = useUI();
  return (
    <View style={styles.empty}>
      {icon && <Ionicons name={icon} size={44} color={ui.labelSecondary} />}
      <Text variant="title" style={styles.center}>
        {title}
      </Text>
      {children ? (
        <Text variant="subhead" color="labelSecondary" style={[styles.center, styles.text]}>
          {children}
        </Text>
      ) : null}
      {action ? <View style={styles.action}>{action}</View> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  empty: { alignItems: 'center', gap: 6, paddingVertical: 48, paddingHorizontal: 24 },
  center: { textAlign: 'center' },
  text: { maxWidth: 320 },
  action: { marginTop: 14 },
});
