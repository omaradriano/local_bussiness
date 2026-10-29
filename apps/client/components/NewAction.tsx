import { strings, term } from '@lb/shared';
import { Ionicons } from '@expo/vector-icons';
import { router, type Href } from 'expo-router';
import { useRef, useState, type ComponentRef, type ReactNode, type RefObject } from 'react';
import { Modal, Pressable, StyleSheet, useWindowDimensions, View } from 'react-native';
import Animated, { Easing, Keyframe } from 'react-native-reanimated';
import { useBusiness } from '../lib/useMe';
import { useTexts } from '../lib/useTexts';
import { Cell, radius, useUI } from '../ui';

export type NewChoice = { icon: keyof typeof Ionicons.glyphMap; label: string; sublabel: string; path: string };
type Anchor = { x: number; y: number; width: number; height: number };

// Popover entrance: from 0.96 and transparent, ease-out, under 200 ms.
const popIn = new Keyframe({
  0: { opacity: 0, transform: [{ scale: 0.96 }] },
  100: { opacity: 1, transform: [{ scale: 1 }], easing: Easing.bezier(0.23, 1, 0.32, 1) },
}).duration(160);

/**
 * The single "new" button of a business: straight to a new booking, or — when the business sells
 * products — a popover under the button to choose the booking or a counter sale.
 * The caller wraps its button in a View with `ref={anchor}` and renders `sheet`.
 */
export function useNewAction(businessId: string): {
  label: string; choices: NewChoice[]; onPress: () => void; sheet: ReactNode; anchor: RefObject<ComponentRef<typeof View> | null>;
} {
  const { business } = useBusiness(businessId);
  const { has } = useTexts();
  const anchor = useRef<ComponentRef<typeof View>>(null);
  const [at, setAt] = useState<Anchor | null>(null);
  const go = (path: string) => { setAt(null); router.push(`/admin/${businessId}${path}` as Href); };
  const n = strings.nav;
  const booking: NewChoice = { icon: 'calendar', label: strings.bookings.new(term(business?.vertical ?? 'tool_rental', 'booking') ?? ''), sublabel: n.bookingHint, path: '/bookings/new' };
  if (!has('productSales')) return { label: booking.label, choices: [booking], onPress: () => go(booking.path), sheet: null, anchor };

  const choices: NewChoice[] = [booking, { icon: 'pricetags', label: n.sale, sublabel: n.saleHint, path: '/products' }];
  const open = () => anchor.current?.measureInWindow((x, y, width, height) => setAt({ x, y, width, height }));
  return { label: n.new, choices, onPress: open, anchor, sheet: <Popover at={at} onClose={() => setAt(null)} choices={choices} go={go} /> };
}

function Popover({ at, onClose, choices, go }: { at: Anchor | null; onClose: () => void; choices: NewChoice[]; go: (path: string) => void }) {
  const ui = useUI();
  const { width: screen } = useWindowDimensions();
  if (!at) return null;
  const width = Math.min(Math.max(at.width, 280), screen - 32);
  const left = Math.min(Math.max(at.x, 16), screen - width - 16);

  return (
    <Modal visible transparent animationType="none" onRequestClose={onClose}>
      <Pressable accessibilityLabel={strings.photos.close} onPress={onClose} style={{ flex: 1 }}>
        <Animated.View entering={popIn} accessibilityRole="menu" accessibilityLabel={strings.nav.newHint}
          style={{ position: 'absolute', top: at.y + at.height + 6, left, width, transformOrigin: 'top',
            backgroundColor: ui.surfaceRaised, borderRadius: radius.lg, overflow: 'hidden',
            shadowColor: '#000', shadowOpacity: 0.25, shadowRadius: 24, shadowOffset: { width: 0, height: 8 }, elevation: 12 }}>
          {/* Stop taps inside the popover from closing it. */}
          <Pressable onPress={() => {}}>
            {choices.map((c, i) => (
              <View key={c.path}>
                {i > 0 && <View style={{ height: StyleSheet.hairlineWidth, marginLeft: 58, backgroundColor: ui.separator }} />}
                <Cell icon={c.icon} tint={ui.accent} label={c.label} sublabel={c.sublabel} onPress={() => go(c.path)} />
              </View>
            ))}
          </Pressable>
        </Animated.View>
      </Pressable>
    </Modal>
  );
}
