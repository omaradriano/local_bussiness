import type { ReactNode } from 'react';
import { StyleSheet, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';

import { PressableScale } from './PressableScale';
import { Text } from './Text';
import { radius, useUI } from './tokens';

/** Tappable pill for categories, tags and select-type fields. */
export function Chip({
  selected,
  removable,
  onPress,
  children,
}: {
  selected?: boolean;
  /** Shows an ✕ (selected tags removed on tap). */
  removable?: boolean;
  onPress: () => void;
  children: string;
}) {
  const ui = useUI();
  return (
    <PressableScale
      onPress={onPress}
      accessibilityRole="button"
      accessibilityState={{ selected }}
      style={[styles.chip, { backgroundColor: selected ? ui.accent : ui.fill }]}>
      <Text variant="subhead" style={{ color: selected ? ui.onAccent : ui.label }}>
        {children}
      </Text>
      {removable && <Ionicons name="close" size={14} color={selected ? ui.onAccent : ui.label} />}
    </PressableScale>
  );
}

/** Wrapping row of chips. */
export function Chips({ children }: { children: ReactNode }) {
  return <View style={styles.row}>{children}</View>;
}

const styles = StyleSheet.create({
  chip: { flexDirection: 'row', alignItems: 'center', gap: 4, minHeight: 34, paddingHorizontal: 12, borderRadius: radius.full },
  row: { flexDirection: 'row', flexWrap: 'wrap', gap: 8 },
});
