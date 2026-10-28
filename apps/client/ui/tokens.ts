import { useColorScheme, type TextStyle } from 'react-native';
import { Easing } from 'react-native-reanimated';

// Design tokens: iOS grouped greys, system font, radii 10/12/14 and Emil Kowalski's curves.
// Same system as the fastmarketcuu app, so both products feel alike.

const light = {
  bg: '#F2F2F7',
  surface: '#FFFFFF',
  surfaceRaised: '#FFFFFF',
  separator: 'rgba(60,60,67,0.18)',
  label: '#000000',
  labelSecondary: 'rgba(60,60,67,0.6)',
  fill: 'rgba(118,118,128,0.12)',
  accent: '#0052CC',
  accentFill: 'rgba(0,82,204,0.1)',
  success: '#34C759',
  successFill: 'rgba(52,199,89,0.14)',
  danger: '#FF3B30',
  warning: '#FF9500',
  warningFill: 'rgba(255,149,0,0.14)',
  purple: '#AF52DE',
  indigo: '#5856D6',
  gray: '#8E8E93',
  onAccent: '#FFFFFF',
};

const dark: typeof light = {
  bg: '#000000',
  surface: '#1C1C1E',
  surfaceRaised: '#2C2C2E',
  separator: 'rgba(84,84,88,0.6)',
  label: '#FFFFFF',
  labelSecondary: 'rgba(235,235,245,0.6)',
  fill: 'rgba(118,118,128,0.24)',
  accent: '#2684FF',
  accentFill: 'rgba(38,132,255,0.18)',
  success: '#30D158',
  successFill: 'rgba(48,209,88,0.18)',
  danger: '#FF453A',
  warning: '#FF9F0A',
  warningFill: 'rgba(255,159,10,0.18)',
  purple: '#BF5AF2',
  indigo: '#5E5CE6',
  gray: '#98989D',
  onAccent: '#FFFFFF',
};

export type UIColors = typeof light;
export type UIColor = keyof UIColors;

/** Colors for the current light/dark appearance. */
export function useUI(): UIColors {
  return useColorScheme() === 'dark' ? dark : light;
}

export const radius = { sm: 10, md: 12, lg: 14, full: 999 } as const;
export const space = { xs: 4, sm: 8, md: 12, lg: 16, xl: 20, xxl: 28 } as const;

// System font (SF Pro on iOS), iOS text styles.
export const type = {
  largeTitle: { fontSize: 34, lineHeight: 41, fontWeight: '700', letterSpacing: 0.37 },
  title: { fontSize: 22, lineHeight: 28, fontWeight: '700' },
  headline: { fontSize: 17, lineHeight: 22, fontWeight: '600' },
  body: { fontSize: 17, lineHeight: 22 },
  callout: { fontSize: 16, lineHeight: 21 },
  subhead: { fontSize: 15, lineHeight: 20 },
  footnote: { fontSize: 13, lineHeight: 18 },
  caption: { fontSize: 12, lineHeight: 16 },
} as const satisfies Record<string, TextStyle>;

export type TextVariant = keyof typeof type;

// Emil Kowalski's curves (never ease-in on UI).
export const ease = {
  out: Easing.bezier(0.23, 1, 0.32, 1),
  inOut: Easing.bezier(0.77, 0, 0.175, 1),
  sheet: Easing.bezier(0.32, 0.72, 0, 1),
};

/** Max width of a screen's content on web/tablet. */
export const contentWidth = 640;
