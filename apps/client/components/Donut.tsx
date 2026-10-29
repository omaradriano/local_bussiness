import { formatMXN } from '@lb/shared';
import { useColorScheme, View } from 'react-native';
import Svg, { Circle, G } from 'react-native-svg';
import { Text, useUI } from '../ui';

// Categorical colors in fixed order (dataviz reference palette, validated for these surfaces:
// light #ffffff, dark #1c1c1e). Light slots 3–5 sit under 3:1, so the legend is always shown.
const LIGHT = ['#2a78d6', '#eb6834', '#1baf7a', '#eda100', '#e87ba4', '#6250d6'];
const DARK = ['#3987e5', '#d95926', '#199e70', '#c98500', '#d55181', '#9085e9'];
const MAX_SLICES = 6; // past that, the smallest fold into "Otros"

export type Slice = { key: string; label: string; amount: number };

/**
 * Part-to-whole donut with its legend (name, amount, %), so identity never relies on color alone.
 * Colors follow the slice key (stable across periods), not its rank.
 */
export function Donut({ title, total, slices, colorKeys, empty }: {
  title: string; total: number; slices: Slice[]; colorKeys: string[]; empty: string;
}) {
  const ui = useUI();
  const palette = useColorScheme() === 'dark' ? DARK : LIGHT;
  const sorted = [...slices].filter((s) => s.amount > 0).sort((a, b) => b.amount - a.amount);
  const shown = sorted.length > MAX_SLICES ? sorted.slice(0, MAX_SLICES - 1) : sorted;
  const rest = sorted.slice(shown.length).reduce((n, s) => n + s.amount, 0);
  const parts = rest > 0 ? [...shown, { key: '__other', label: 'Otros', amount: rest }] : shown;
  const sum = parts.reduce((n, s) => n + s.amount, 0);
  const color = (key: string) => (key === '__other' ? ui.gray : palette[colorKeys.indexOf(key) % palette.length] ?? ui.gray);

  const size = 168, stroke = 26, r = (size - stroke) / 2, c = 2 * Math.PI * r;
  const gap = parts.length > 1 ? 2 : 0; // 2px surface gap between slices
  let offset = 0;

  return (
    <View style={{ gap: 14 }}>
      <Text variant="headline">{title}</Text>
      {sum === 0 ? <Text color="labelSecondary">{empty}</Text> : (
        <View style={{ flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', gap: 20 }}>
          <View style={{ width: size, height: size, alignItems: 'center', justifyContent: 'center' }}>
            {/* Hidden on the View: react-native-svg forwards a11y props to the DOM on web */}
            <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
            <Svg width={size} height={size}>
              <G rotation={-90} origin={`${size / 2}, ${size / 2}`}>
                {parts.map((p) => {
                  const len = (p.amount / sum) * c;
                  const el = (
                    <Circle key={p.key} cx={size / 2} cy={size / 2} r={r} fill="none" stroke={color(p.key)} strokeWidth={stroke}
                      strokeDasharray={`${Math.max(len - gap, 0.1)} ${c}`} strokeDashoffset={-offset} />
                  );
                  offset += len;
                  return el;
                })}
              </G>
            </Svg>
            </View>
            <View style={{ position: 'absolute', alignItems: 'center' }}>
              <Text variant="caption" color="labelSecondary">Total</Text>
              <Text variant="headline" style={{ fontVariant: ['tabular-nums'] }}>{formatMXN(total)}</Text>
            </View>
          </View>
          <View style={{ flex: 1, minWidth: 180, gap: 8 }}>
            {parts.map((p) => (
              <View key={p.key} accessibilityLabel={`${p.label}: ${formatMXN(p.amount)}, ${Math.round((p.amount / sum) * 100)} %`}
                style={{ flexDirection: 'row', alignItems: 'center', gap: 8 }}>
                <View style={{ width: 10, height: 10, borderRadius: 3, backgroundColor: color(p.key) }} />
                <Text variant="subhead" style={{ flex: 1 }} numberOfLines={1}>{p.label}</Text>
                <Text variant="subhead" style={{ fontVariant: ['tabular-nums'] }}>{formatMXN(p.amount)}</Text>
                <Text variant="footnote" color="labelSecondary" style={{ width: 40, textAlign: 'right', fontVariant: ['tabular-nums'] }}>
                  {Math.round((p.amount / sum) * 100)} %
                </Text>
              </View>
            ))}
          </View>
        </View>
      )}
    </View>
  );
}
