import { useState } from 'react';
import { Pressable, StyleSheet, View } from 'react-native';
import Animated, { cubicBezier, useReducedMotion } from 'react-native-reanimated';

import { Text } from './Text';
import { useUI } from './tokens';

const EASE_IN_OUT = cubicBezier(0.77, 0, 0.175, 1);

export type SegmentOption<T extends string> = { value: T; label: string; count?: number };

/**
 * iOS segmented control. Columns are always equal (long labels truncate,
 * counts always show) and the white thumb slides to the selected one
 * (220ms ease-in-out, a CSS transition on the UI thread).
 */
export function Segmented<T extends string>({
  options,
  value,
  onChange,
}: {
  options: SegmentOption<T>[];
  value: T;
  onChange: (value: T) => void;
}) {
  const ui = useUI();
  const reduced = useReducedMotion();
  const [width, setWidth] = useState(0);
  const index = Math.max(0, options.findIndex((o) => o.value === value));
  const segment = width > 0 ? (width - 4) / options.length : 0;

  return (
    <View
      accessibilityRole="tablist"
      style={[styles.track, { backgroundColor: ui.fill }]}
      onLayout={(e) => setWidth(e.nativeEvent.layout.width)}>
      {segment > 0 && (
        <Animated.View
          style={[
            styles.thumb,
            {
              width: segment,
              backgroundColor: ui.surfaceRaised,
              transform: [{ translateX: segment * index }],
              transitionProperty: 'transform',
              transitionDuration: reduced ? 0 : 220,
              transitionTimingFunction: EASE_IN_OUT,
            },
          ]}
        />
      )}
      {options.map((o) => {
        const selected = o.value === value;
        return (
          <Pressable
            key={o.value}
            accessibilityRole="tab"
            accessibilityState={{ selected }}
            onPress={() => onChange(o.value)}
            style={styles.option}>
            <Text variant="footnote" numberOfLines={1} style={[styles.label, selected && styles.selected]}>
              {o.label}
            </Text>
            {o.count !== undefined && (
              <Text variant="caption" color="labelSecondary" style={styles.count}>
                {o.count}
              </Text>
            )}
          </Pressable>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  track: { flexDirection: 'row', padding: 2, borderRadius: 9 },
  thumb: {
    position: 'absolute',
    top: 2,
    bottom: 2,
    left: 2,
    borderRadius: 7,
    shadowColor: '#000',
    shadowOpacity: 0.12,
    shadowRadius: 4,
    shadowOffset: { width: 0, height: 2 },
    elevation: 2,
  },
  option: { flex: 1, minWidth: 0, minHeight: 32, flexDirection: 'row', alignItems: 'center', justifyContent: 'center', gap: 3, paddingHorizontal: 4 },
  label: { flexShrink: 1, fontWeight: '500' },
  selected: { fontWeight: '600' },
  count: { flexShrink: 0, fontVariant: ['tabular-nums'] },
});
