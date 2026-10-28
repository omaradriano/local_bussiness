import { Children, Fragment, isValidElement, type ReactNode } from 'react';
import { Pressable, StyleSheet, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';

import { Text } from './Text';
import { radius, space, useUI } from './tokens';

// iOS Settings-style grouped list.

export function Group({
  label,
  footer,
  padded,
  separatorInset = 16,
  children,
}: {
  label?: string;
  footer?: ReactNode;
  /** Free content (forms, text) instead of a list of cells. */
  padded?: boolean;
  /** Where separators start (58 when the cells have icons). */
  separatorInset?: number;
  children: ReactNode;
}) {
  const ui = useUI();
  const items = Children.toArray(children).filter(isValidElement);
  return (
    <View style={styles.group}>
      {label && (
        <Text variant="footnote" color="labelSecondary" style={styles.label}>
          {label.toUpperCase()}
        </Text>
      )}
      <View style={[styles.body, { backgroundColor: ui.surface }, padded && styles.padded]}>
        {padded
          ? children
          : items.map((child, i) => (
              <Fragment key={child.key ?? i}>
                {i > 0 && (
                  <View style={[styles.separator, { marginLeft: separatorInset, backgroundColor: ui.separator }]} />
                )}
                {child}
              </Fragment>
            ))}
      </View>
      {footer ? (
        <Text variant="footnote" color="labelSecondary" style={styles.footer}>
          {footer}
        </Text>
      ) : null}
    </View>
  );
}

export function Cell({
  label,
  sublabel,
  icon,
  tint,
  value,
  badge,
  destructive,
  chevron,
  accessory,
  onPress,
}: {
  label: string;
  sublabel?: string;
  icon?: keyof typeof Ionicons.glyphMap;
  /** Icon tile color; defaults to the accent. */
  tint?: string;
  value?: string;
  badge?: number;
  destructive?: boolean;
  /** Shown by default on cells that navigate (onPress and not destructive). */
  chevron?: boolean;
  /** Right-side control, e.g. a Switch. */
  accessory?: ReactNode;
  onPress?: () => void;
}) {
  const ui = useUI();
  const showChevron = chevron ?? (!!onPress && !destructive && !accessory);
  const content = (
    <>
      {icon && (
        <View style={[styles.icon, { backgroundColor: tint ?? ui.accent }]}>
          <Ionicons name={icon} size={18} color="#fff" />
        </View>
      )}
      <View style={styles.texts}>
        <Text style={destructive ? { color: ui.danger } : undefined} numberOfLines={1}>
          {label}
        </Text>
        {sublabel ? (
          <Text variant="footnote" color="labelSecondary" numberOfLines={2}>
            {sublabel}
          </Text>
        ) : null}
      </View>
      {value ? (
        <Text color="labelSecondary" style={styles.value}>
          {value}
        </Text>
      ) : null}
      {!!badge && (
        <View style={[styles.badge, { backgroundColor: ui.danger }]}>
          <Text variant="caption" style={styles.badgeText}>
            {badge}
          </Text>
        </View>
      )}
      {accessory}
      {showChevron && <Ionicons name="chevron-forward" size={18} color={ui.labelSecondary} style={styles.chevron} />}
    </>
  );

  if (!onPress) return <View style={styles.cell}>{content}</View>;
  return (
    // iOS cells highlight instantly on touch, no scale.
    <Pressable
      onPress={onPress}
      accessibilityRole="button"
      style={({ pressed }) => [styles.cell, pressed && { backgroundColor: ui.fill }]}>
      {content}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  group: { marginBottom: space.xxl },
  label: { marginHorizontal: space.lg, marginBottom: 6 },
  body: { borderRadius: radius.sm, overflow: 'hidden' },
  padded: { padding: space.lg },
  footer: { marginHorizontal: space.lg, marginTop: 6 },
  separator: { height: StyleSheet.hairlineWidth },
  cell: { flexDirection: 'row', alignItems: 'center', gap: space.md, minHeight: 44, paddingVertical: 10, paddingHorizontal: space.lg },
  icon: { width: 30, height: 30, borderRadius: 7, alignItems: 'center', justifyContent: 'center' },
  texts: { flex: 1, minWidth: 0 },
  value: { fontVariant: ['tabular-nums'] },
  badge: { minWidth: 22, height: 22, borderRadius: 11, paddingHorizontal: 6, alignItems: 'center', justifyContent: 'center' },
  badgeText: { color: '#fff', fontWeight: '600' },
  chevron: { marginRight: -4, opacity: 0.6 },
});
