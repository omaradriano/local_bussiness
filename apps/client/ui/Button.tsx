import { ActivityIndicator, StyleSheet, View, type StyleProp, type ViewStyle } from 'react-native';
import { Ionicons } from '@expo/vector-icons';

import { PressableScale } from './PressableScale';
import { Text } from './Text';
import { radius, useUI } from './tokens';

type Variant = 'primary' | 'secondary' | 'destructive' | 'plain';

/** iOS-style button: filled, tinted, destructive or plain text. */
export function Button({
  variant = 'secondary',
  size = 'md',
  icon,
  busy,
  busyLabel = 'Procesando…',
  disabled,
  onPress,
  style,
  children,
}: {
  variant?: Variant;
  size?: 'md' | 'sm';
  icon?: keyof typeof Ionicons.glyphMap;
  /** Disables the button and shows a spinner + busyLabel. */
  busy?: boolean;
  busyLabel?: string;
  disabled?: boolean;
  onPress?: () => void;
  /** Layout only (flex, alignSelf, margins): applied to the touch area. */
  style?: StyleProp<ViewStyle>;
  children: string;
}) {
  const ui = useUI();
  const bg = variant === 'primary' ? ui.accent : variant === 'plain' ? 'transparent' : ui.fill;
  const fg = variant === 'primary' ? ui.onAccent : variant === 'destructive' ? ui.danger : ui.accent;
  const off = disabled || busy;

  return (
    <PressableScale
      onPress={onPress}
      disabled={off}
      accessibilityRole="button"
      accessibilityState={{ disabled: off, busy }}
      containerStyle={style}
      style={[
        styles.base,
        size === 'sm' ? styles.sm : styles.md,
        variant === 'plain' && styles.plain,
        { backgroundColor: bg, opacity: disabled && !busy ? 0.45 : 1 },
      ]}>
      <View style={styles.content}>
        {busy ? (
          <ActivityIndicator size="small" color={fg} />
        ) : (
          icon && <Ionicons name={icon} size={size === 'sm' ? 16 : 18} color={fg} />
        )}
        <Text variant={size === 'sm' ? 'subhead' : 'headline'} style={{ color: fg, fontWeight: '600' }}>
          {busy ? busyLabel : children}
        </Text>
      </View>
    </PressableScale>
  );
}

const styles = StyleSheet.create({
  base: { borderRadius: radius.sm, alignItems: 'center', justifyContent: 'center' },
  md: { minHeight: 50, paddingHorizontal: 18 },
  sm: { minHeight: 34, paddingHorizontal: 12, borderRadius: 8 },
  plain: { paddingHorizontal: 8 },
  content: { flexDirection: 'row', alignItems: 'center', gap: 6 },
});
