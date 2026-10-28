import { Ionicons } from '@expo/vector-icons';
import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { Pressable, StyleSheet, View } from 'react-native';
import Animated, { Easing, FadeInUp, FadeOutUp } from 'react-native-reanimated';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { Text } from './Text';
import { contentWidth, radius, useUI } from './tokens';

// Results of an action (saved, refused, missing steps) as a floating toast at the top, so they're
// seen even when the screen is scrolled down. Field errors stay next to their field.

export type ToastTone = 'success' | 'error' | 'info';
type Show = (message: string, tone?: ToastTone) => void;

const ToastContext = createContext<Show>(() => {});

const ENTER = FadeInUp.duration(250).easing(Easing.bezier(0.23, 1, 0.32, 1));
const EXIT = FadeOutUp.duration(200).easing(Easing.bezier(0.23, 1, 0.32, 1));
// Errors need reading (and often acting on), so they stay longer; tapping closes any toast.
const DURATION: Record<ToastTone, number> = { success: 2500, info: 4000, error: 6000 };

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toast, setToast] = useState<{ id: number; message: string; tone: ToastTone } | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const hide = useCallback(() => {
    if (timer.current) clearTimeout(timer.current);
    setToast(null);
  }, []);

  const show = useCallback<Show>((message, tone = 'success') => {
    if (timer.current) clearTimeout(timer.current);
    setToast({ id: Date.now(), message, tone });
    timer.current = setTimeout(() => setToast(null), DURATION[tone]);
  }, []);

  useEffect(() => () => { if (timer.current) clearTimeout(timer.current); }, []);

  return (
    <ToastContext.Provider value={show}>
      {children}
      {toast && <ToastView key={toast.id} message={toast.message} tone={toast.tone} onPress={hide} />}
    </ToastContext.Provider>
  );
}

/** `toast('Guardado')`, `toast('No se pudo…', 'error')` */
export function useToast() {
  return useContext(ToastContext);
}

function ToastView({ message, tone, onPress }: { message: string; tone: ToastTone; onPress: () => void }) {
  const ui = useUI();
  const insets = useSafeAreaInsets();
  const icon = tone === 'success' ? 'checkmark-circle' : tone === 'error' ? 'alert-circle' : 'information-circle';
  const color = tone === 'success' ? ui.success : tone === 'error' ? ui.danger : ui.accent;
  return (
    <View pointerEvents="box-none" style={[styles.host, { top: insets.top + 8 }]}>
      <Animated.View entering={ENTER} exiting={EXIT} style={styles.wrap}>
        <Pressable onPress={onPress} accessibilityRole="alert" accessibilityLiveRegion={tone === 'error' ? 'assertive' : 'polite'}
          accessibilityHint="Toca para cerrar"
          style={[styles.toast, { backgroundColor: ui.surfaceRaised, borderColor: tone === 'error' ? ui.danger : ui.separator }]}>
          <Ionicons name={icon} size={20} color={color} />
          <Text variant="subhead" style={styles.text}>{message}</Text>
        </Pressable>
      </Animated.View>
    </View>
  );
}

const styles = StyleSheet.create({
  host: { position: 'absolute', left: 16, right: 16, alignItems: 'center', zIndex: 1000 },
  wrap: { width: '100%', maxWidth: contentWidth - 32, alignItems: 'center' },
  toast: {
    flexDirection: 'row', alignItems: 'center', gap: 8, paddingVertical: 12, paddingHorizontal: 16,
    borderRadius: radius.md, borderWidth: StyleSheet.hairlineWidth,
    shadowColor: '#000', shadowOpacity: 0.15, shadowRadius: 12, shadowOffset: { width: 0, height: 4 }, elevation: 6,
  },
  text: { fontWeight: '500', flexShrink: 1 },
});
