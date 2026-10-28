import { useState } from 'react';
import { Pressable, type PressableProps, type StyleProp, type ViewStyle } from 'react-native';
import Animated, { cubicBezier, useReducedMotion } from 'react-native-reanimated';

const EASE_OUT = cubicBezier(0.23, 1, 0.32, 1);

/**
 * Press feedback: scales to 0.97 on press-in (160ms ease-out, a Reanimated
 * CSS transition on the UI thread). No scale with "Reduce Motion".
 */
export function PressableScale({
  style,
  containerStyle,
  children,
  scaleTo = 0.97,
  ...props
}: Omit<PressableProps, 'style' | 'children'> & {
  /** Look of the scaled view (background, padding, radius…). */
  style?: StyleProp<ViewStyle>;
  /** Layout of the touch area (flex, width, margins). The scaled view fills it. */
  containerStyle?: StyleProp<ViewStyle>;
  children: React.ReactNode;
  scaleTo?: number;
}) {
  const [pressed, setPressed] = useState(false);
  const reduced = useReducedMotion();
  return (
    <Pressable
      {...props}
      style={containerStyle}
      pressRetentionOffset={{ top: 20, bottom: 20, left: 20, right: 20 }}
      onPressIn={(e) => {
        setPressed(true);
        props.onPressIn?.(e);
      }}
      onPressOut={(e) => {
        setPressed(false);
        props.onPressOut?.(e);
      }}>
      <Animated.View
        style={[
          style,
          {
            transform: [{ scale: pressed && !reduced && !props.disabled ? scaleTo : 1 }],
            transitionProperty: 'transform',
            transitionDuration: 160,
            transitionTimingFunction: EASE_OUT,
          },
        ]}>
        {children}
      </Animated.View>
    </Pressable>
  );
}
