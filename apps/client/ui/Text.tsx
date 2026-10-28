import { Text as RNText, type TextProps } from 'react-native';

import { type, useUI, type TextVariant, type UIColor } from './tokens';

/** System-font text with an iOS text style and a token color. */
export function Text({
  variant = 'body',
  color = 'label',
  style,
  ...props
}: TextProps & { variant?: TextVariant; color?: UIColor }) {
  const ui = useUI();
  return <RNText style={[type[variant], { color: ui[color] }, style]} {...props} />;
}
