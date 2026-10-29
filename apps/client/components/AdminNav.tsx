import { strings, term } from '@lb/shared';
import { Ionicons } from '@expo/vector-icons';
import { router, type Href } from 'expo-router';
import { useState } from 'react';
import { Modal, ScrollView, StyleSheet, useWindowDimensions, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { signOut, useBusiness } from '../lib/useMe';
import { useCurrentBusinessId, useTexts } from '../lib/useTexts';
import { Cell, Group, PressableScale, Text, contentWidth, radius, useUI } from '../ui';
import { useInstallAction } from './InstallApp';
import { useNewAction } from './NewAction';

const WIDE = 900; // from here the sections live in a sidebar instead of the ☰ menu

// Tablets in portrait use the phone menu: keep it as wide as the screens' content.
const centered = { width: '100%', maxWidth: contentWidth, alignSelf: 'center' } as const;

type Section = { path: string; label: string; icon: keyof typeof Ionicons.glyphMap };
type SectionGroup = { label: string; sections: Section[] };

/** Every section of the current business, grouped (day to day, finances, the business), plus navigation helpers. */
function useNav() {
  const { businessId, rest } = useCurrentBusinessId();
  const { texts, has, can } = useTexts();
  const { business } = useBusiness(businessId ?? '');
  const add = useNewAction(businessId ?? '');
  if (!businessId || !business) return null;

  const s = strings.business.sections;
  const owner = business.role === 'owner';
  const groups: SectionGroup[] = [
    { label: strings.business.groups.daily, sections: [
      { path: '/today', label: s.today, icon: 'today' },
      { path: '/calendar', label: s.calendar, icon: 'calendar' },
      { path: '/bookings', label: texts.bookings.plural, icon: 'list' },
      { path: '/customers', label: s.customers, icon: 'people' },
      ...(has('productSales') ? [{ path: '/products', label: strings.products.title, icon: 'pricetags' } as Section] : []),
    ] },
    ...(owner ? [{ label: strings.finances.title, sections: [
      { path: '/finances', label: strings.nav.summary, icon: 'stats-chart' },
      { path: '/finances/expenses', label: strings.finances.expenses, icon: 'receipt' },
      ...(can('payroll') ? [{ path: '/finances/pay', label: strings.nav.pay, icon: 'cash' } as Section] : []),
    ] } as SectionGroup] : []),
    { label: strings.business.groups.business, sections: [
      { path: '/catalog', label: term(business.vertical, 'catalog') ?? s.catalog, icon: 'construct' },
      ...(has('staff') ? [{ path: '/staff', label: s.staff, icon: 'people-circle' } as Section] : []),
      { path: '/hours', label: s.hours, icon: 'time' },
      ...(owner ? [{ path: '/settings', label: s.settings, icon: 'settings' } as Section] : []),
    ] },
  ];
  // The longest matching path wins, so /finances/expenses doesn't also light up /finances.
  const current = groups.flatMap((g) => g.sections.map((sec) => sec.path))
    .filter((p) => rest === p || rest.startsWith(`${p}/`)).sort((a, b) => b.length - a.length)[0];
  const go = (path: string) => router.push(`/admin/${businessId}${path}` as Href);
  return { business, groups, add, active: (path: string) => path === current, go };
}

/** Header part of the navigation: the ☰ menu on phones; on wide screens the sidebar takes over. */
export function AdminNav() {
  const nav = useNav();
  const { width } = useWindowDimensions();
  const insets = useSafeAreaInsets();
  const ui = useUI();
  const [open, setOpen] = useState(false);
  const install = useInstallAction();
  if (!nav || width >= WIDE) return null;
  const close = (then: () => void) => () => { setOpen(false); then(); };

  return (
    <View style={{ paddingRight: 8 }}>
      <PressableScale accessibilityRole="button" accessibilityLabel={strings.nav.menu} hitSlop={10} onPress={() => setOpen(true)} style={{ padding: 6 }}>
        <Ionicons name="menu" size={26} color={ui.accent} />
      </PressableScale>
      {/* Full screen on phones: the whole width and height, below the notch and above the home bar. */}
      <Modal visible={open} animationType="fade" presentationStyle="fullScreen" onRequestClose={() => setOpen(false)}>
        <View style={{ flex: 1, backgroundColor: ui.bg, paddingTop: insets.top }}>
          <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, paddingHorizontal: 16, paddingVertical: 10, ...centered }}>
            <View style={{ flex: 1 }}>
              <Text variant="footnote" color="labelSecondary">{strings.nav.menu.toUpperCase()}</Text>
              <Text variant="title" accessibilityRole="header" numberOfLines={1}>{nav.business.name}</Text>
            </View>
            <PressableScale accessibilityRole="button" accessibilityLabel={strings.photos.close} hitSlop={10} onPress={() => setOpen(false)}
              style={{ width: 36, height: 36, borderRadius: radius.full, backgroundColor: ui.fill, alignItems: 'center', justifyContent: 'center' }}>
              <Ionicons name="close" size={22} color={ui.label} />
            </PressableScale>
          </View>
          <ScrollView contentContainerStyle={{ paddingHorizontal: 16, paddingTop: 8, paddingBottom: insets.bottom + 24, ...centered }}>
            <Group separatorInset={58}>
              {nav.add.choices.map((c) => <Cell key={c.path} icon="add-circle" tint={ui.accent} label={c.label} onPress={close(() => nav.go(c.path))} />)}
            </Group>
            {nav.groups.map((g) => (
              <Group key={g.label} label={g.label} separatorInset={58}>
                {g.sections.map((sec) => (
                  <Cell key={sec.path} icon={sec.icon} tint={nav.active(sec.path) ? ui.accent : ui.gray} label={sec.label} onPress={close(() => nav.go(sec.path))} />
                ))}
              </Group>
            ))}
            <Group separatorInset={58}>
              {install.show && <Cell icon="download" tint={ui.success} label={install.label} onPress={close(install.onPress)} />}
              <Cell icon="storefront" tint={ui.indigo} label={strings.nav.businesses} onPress={close(() => router.push('/admin'))} />
              <Cell label={strings.admin.signOut} destructive onPress={close(signOut)} />
            </Group>
          </ScrollView>
        </View>
      </Modal>
      {install.sheet}
    </View>
  );
}

/** Fixed sidebar with every section on wide screens; renders nothing on phones or outside a business. */
export function AdminSidebar() {
  const nav = useNav();
  const { width } = useWindowDimensions();
  const insets = useSafeAreaInsets();
  const ui = useUI();
  const install = useInstallAction();
  if (!nav || width < WIDE) return null;

  return (
    <ScrollView accessibilityRole="menu" style={{ width: 240, flexGrow: 0, borderRightWidth: StyleSheet.hairlineWidth, borderRightColor: ui.separator, backgroundColor: ui.bg }}
      contentContainerStyle={{ paddingTop: insets.top + 16, paddingBottom: 24, paddingHorizontal: 12, gap: 20 }}>
      <View style={{ gap: 12, paddingHorizontal: 8 }}>
        <Text variant="headline" numberOfLines={2}>{nav.business.name}</Text>
        <PlanLine account={nav.business.account} />
        <View ref={nav.add.anchor} collapsable={false}>
        <PressableScale accessibilityRole="button" onPress={nav.add.onPress}
          style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'center', gap: 6, paddingVertical: 9, borderRadius: radius.full, backgroundColor: ui.accent }}>
          <Ionicons name="add" size={18} color={ui.onAccent} />
          <Text variant="subhead" style={{ color: ui.onAccent, fontWeight: '600' }}>{nav.add.label}</Text>
        </PressableScale>
        </View>
        {nav.add.sheet}
      </View>
      {nav.groups.map((g) => (
        <View key={g.label} style={{ gap: 2 }}>
          <Text variant="footnote" color="labelSecondary" style={{ paddingHorizontal: 8, marginBottom: 4 }}>{g.label.toUpperCase()}</Text>
          {g.sections.map((sec) => <SideLink key={sec.path} section={sec} active={nav.active(sec.path)} onPress={() => nav.go(sec.path)} />)}
        </View>
      ))}
      <View style={{ gap: 2 }}>
        {install.show && <SideLink section={{ path: 'install', label: install.label, icon: 'download' }} active={false} onPress={install.onPress} />}
        {install.sheet}
        <SideLink section={{ path: '/admin', label: strings.nav.businesses, icon: 'storefront' }} active={false} onPress={() => router.push('/admin')} />
        <SideLink section={{ path: 'signout', label: strings.admin.signOut, icon: 'log-out' }} active={false} onPress={signOut} />
      </View>
    </ScrollView>
  );
}

function SideLink({ section, active, onPress }: { section: Section; active: boolean; onPress: () => void }) {
  const ui = useUI();
  return (
    <PressableScale accessibilityRole="link" accessibilityState={{ selected: active }} onPress={onPress}
      style={{ flexDirection: 'row', alignItems: 'center', gap: 10, paddingHorizontal: 8, paddingVertical: 7, borderRadius: radius.sm, backgroundColor: active ? ui.accentFill : 'transparent' }}>
      <Ionicons name={section.icon} size={18} color={active ? ui.accent : ui.gray} />
      <Text variant="subhead" style={{ color: active ? ui.accent : ui.label, fontWeight: active ? '600' : '400' }}>{section.label}</Text>
    </PressableScale>
  );
}

/** Under the business name in the sidebar: the trial's days left, or read-only. */
function PlanLine({ account }: { account: { plan: string; status: string; days_left: number } }) {
  const ui = useUI();
  if (account.status === 'active') return null;
  const expired = account.status === 'expired';
  return (
    <Text variant="footnote" style={{ color: expired ? ui.warning : ui.labelSecondary, marginTop: -6 }}>
      {expired ? strings.plans.readOnly : strings.plans.trialLeft(account.days_left)}
    </Text>
  );
}
