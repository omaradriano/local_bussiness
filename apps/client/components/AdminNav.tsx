import { strings, term } from '@lb/shared';
import { Ionicons } from '@expo/vector-icons';
import { router, type Href } from 'expo-router';
import { useState } from 'react';
import { Modal, Pressable, ScrollView, useWindowDimensions, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { signOut, useBusiness } from '../lib/useMe';
import { useCurrentBusinessId, useTexts } from '../lib/useTexts';
import { Cell, Group, PressableScale, Text, radius, useUI } from '../ui';

const WIDE = 900; // from here the sections fit inline in the header

type Section = { path: string; label: string; icon: keyof typeof Ionicons.glyphMap };

/**
 * Header navigation inside a business: the essential sections inline on wide screens, and a
 * menu button (☰) on phones with every section, switching business and signing out.
 */
export function AdminNav() {
  const { businessId, rest } = useCurrentBusinessId();
  const { texts } = useTexts();
  const { business } = useBusiness(businessId ?? '');
  const { width } = useWindowDimensions();
  const insets = useSafeAreaInsets();
  const ui = useUI();
  const [open, setOpen] = useState(false);
  if (!businessId || !business) return null;

  const s = strings.business.sections;
  const essential: Section[] = [
    { path: '/today', label: s.today, icon: 'today' },
    { path: '/calendar', label: s.calendar, icon: 'calendar' },
    { path: '/bookings', label: texts.bookings.plural, icon: 'list' },
    { path: '/customers', label: s.customers, icon: 'people' },
    { path: '/catalog', label: term(business.vertical, 'catalog') ?? s.catalog, icon: 'construct' },
  ];
  const more: Section[] = [
    { path: '/hours', label: s.hours, icon: 'time' },
    ...(business.role === 'owner' ? [{ path: '/settings', label: s.settings, icon: 'settings' } as Section] : []),
  ];
  const active = (path: string) => rest === path || rest.startsWith(`${path}/`);
  const go = (path: string) => { setOpen(false); router.push(`/admin/${businessId}${path}` as Href); };
  const newBooking = () => go('/bookings/new');

  if (width >= WIDE) {
    return (
      <View style={{ flexDirection: 'row', alignItems: 'center', gap: 4, paddingRight: 12 }}>
        {essential.map((sec) => (
          <PressableScale key={sec.path} accessibilityRole="link" accessibilityState={{ selected: active(sec.path) }} onPress={() => go(sec.path)}
            style={{ paddingHorizontal: 10, paddingVertical: 6, borderRadius: radius.full, backgroundColor: active(sec.path) ? ui.accentFill : 'transparent' }}>
            <Text variant="subhead" style={{ color: active(sec.path) ? ui.accent : ui.label, fontWeight: active(sec.path) ? '600' : '400' }}>{sec.label}</Text>
          </PressableScale>
        ))}
        <PressableScale accessibilityRole="button" onPress={newBooking}
          style={{ flexDirection: 'row', alignItems: 'center', gap: 4, marginLeft: 6, paddingHorizontal: 12, paddingVertical: 7, borderRadius: radius.full, backgroundColor: ui.accent }}>
          <Ionicons name="add" size={16} color={ui.onAccent} />
          <Text variant="subhead" style={{ color: ui.onAccent, fontWeight: '600' }}>{strings.bookings.new(term(business.vertical, 'booking') ?? '')}</Text>
        </PressableScale>
        <MenuButton onPress={() => setOpen(true)} />
        <Menu open={open} onClose={() => setOpen(false)} top={insets.top} sections={more} businessName={business.name} go={go} active={active} />
      </View>
    );
  }

  return (
    <View style={{ paddingRight: 8 }}>
      <MenuButton onPress={() => setOpen(true)} />
      <Menu open={open} onClose={() => setOpen(false)} top={insets.top} businessName={business.name} go={go} active={active}
        sections={essential.concat(more)} newBooking={{ label: strings.bookings.new(term(business.vertical, 'booking') ?? ''), onPress: newBooking }} />
    </View>
  );
}

function MenuButton({ onPress }: { onPress: () => void }) {
  const ui = useUI();
  return (
    <PressableScale accessibilityRole="button" accessibilityLabel={strings.nav.menu} hitSlop={10} onPress={onPress} style={{ padding: 6 }}>
      <Ionicons name="menu" size={26} color={ui.accent} />
    </PressableScale>
  );
}

function Menu({ open, onClose, top, sections, businessName, go, active, newBooking }: {
  open: boolean; onClose: () => void; top: number; sections: Section[]; businessName: string;
  go: (path: string) => void; active: (path: string) => boolean; newBooking?: { label: string; onPress: () => void };
}) {
  const ui = useUI();
  return (
    <Modal visible={open} transparent animationType="fade" onRequestClose={onClose}>
      <Pressable accessibilityLabel={strings.photos.close} onPress={onClose} style={{ flex: 1, backgroundColor: 'rgba(0,0,0,0.4)' }}>
        {/* Stop taps inside the panel from closing it. */}
        <Pressable onPress={() => {}} style={{ marginTop: top + 8, marginHorizontal: 12, alignSelf: 'flex-end', width: '100%', maxWidth: 360,
          backgroundColor: ui.bg, borderRadius: radius.lg, padding: 12, maxHeight: '90%' }}>
          <ScrollView>
            <Text variant="footnote" color="labelSecondary" style={{ marginHorizontal: 16, marginBottom: 8 }}>{businessName.toUpperCase()}</Text>
            {newBooking && (
              <Group>
                <Cell icon="add-circle" tint={ui.accent} label={newBooking.label} onPress={newBooking.onPress} />
              </Group>
            )}
            <Group separatorInset={58}>
              {sections.map((sec) => (
                <Cell key={sec.path} icon={sec.icon} tint={active(sec.path) ? ui.accent : ui.gray} label={sec.label} onPress={() => go(sec.path)} />
              ))}
            </Group>
            <Group separatorInset={58}>
              <Cell icon="storefront" tint={ui.indigo} label={strings.nav.businesses} onPress={() => { onClose(); router.push('/admin'); }} />
              <Cell label={strings.admin.signOut} destructive onPress={() => { onClose(); signOut(); }} />
            </Group>
          </ScrollView>
        </Pressable>
      </Pressable>
    </Modal>
  );
}
