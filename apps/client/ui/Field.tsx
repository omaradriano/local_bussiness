import { StyleSheet, TextInput, View, type TextInputProps } from 'react-native';

import { Text } from './Text';
import { radius, space, useUI } from './tokens';

/**
 * Labelled text input. Background and text colors are explicit: without
 * them the text is unreadable in dark mode on real devices.
 * The error replaces the hint while it exists.
 */
export function Field({
  label,
  hint,
  error,
  multiline,
  style,
  ...props
}: TextInputProps & { label: string; hint?: string; error?: string | null }) {
  const ui = useUI();
  return (
    <View style={styles.field}>
      <Text variant="footnote" color="labelSecondary" style={styles.label}>
        {label}
      </Text>
      <TextInput
        {...props}
        multiline={multiline}
        accessibilityLabel={label}
        placeholderTextColor={ui.labelSecondary}
        style={[
          styles.input,
          multiline && styles.area,
          { backgroundColor: ui.fill, color: ui.label },
          error ? { borderColor: ui.danger, borderWidth: 1 } : null,
          style,
        ]}
      />
      {error ? (
        <Text variant="footnote" style={[styles.help, { color: ui.danger }]}>
          {error}
        </Text>
      ) : hint ? (
        <Text variant="footnote" color="labelSecondary" style={styles.help}>
          {hint}
        </Text>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  field: { gap: 6, marginBottom: space.lg },
  label: { fontWeight: '600', paddingHorizontal: 4 },
  input: { minHeight: 44, borderRadius: radius.sm, paddingHorizontal: 12, paddingVertical: 10, fontSize: 17 },
  area: { minHeight: 96, textAlignVertical: 'top' },
  help: { paddingHorizontal: 4 },
});
