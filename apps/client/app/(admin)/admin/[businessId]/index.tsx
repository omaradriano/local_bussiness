import { strings, term } from '@lb/shared';
import { router, Stack, useLocalSearchParams, type Href } from 'expo-router';
import { ActivityIndicator } from 'react-native';
import { useBusiness } from '../../../../lib/useMe';
import { useTexts } from '../../../../lib/useTexts';
import { Button, Cell, Group, Screen, Text, useUI } from '../../../../ui';

const s = strings.business;

export default function BusinessHome() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const { business, businesses, error } = useBusiness(businessId);
  const ui = useUI();
  const { texts, has } = useTexts();

  if (error) return <Text style={{ padding: 16 }}>{error}</Text>;
  if (!businesses) return <ActivityIndicator style={{ marginTop: 24 }} accessibilityLabel={strings.common.loading} />;
  if (!business) return <Text style={{ padding: 16 }}>{strings.common.notFound}</Text>;
  const go = (pathname: string) => () => router.push({ pathname, params: { businessId } } as Href);

  return (
    <Screen>
      <Stack.Screen options={{ title: business.name }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 16 }}>{business.name}</Text>
      <Button variant="primary" icon="add-circle" onPress={go('/admin/[businessId]/bookings/new')} style={{ marginBottom: 28 }}>
        {strings.bookings.new(term(business.vertical, 'booking') ?? '')}
      </Button>

      <Group label={s.groups.daily} separatorInset={58}>
        <Cell icon="today" tint={ui.warning} label={s.sections.today} sublabel={texts.home.today} onPress={go('/admin/[businessId]/today')} />
        <Cell icon="calendar" tint={ui.accent} label={s.sections.calendar} sublabel={texts.home.calendar} onPress={go('/admin/[businessId]/calendar')} />
        <Cell icon="time" tint={ui.purple} label={texts.bookings.plural} sublabel={texts.home.history} onPress={go('/admin/[businessId]/bookings')} />
        {has('productSales') ? (
          <Cell icon="pricetags" tint={ui.success} label={strings.products.title} sublabel={strings.products.hint} onPress={go('/admin/[businessId]/products')} />
        ) : null}
        <Cell icon="people" tint={ui.indigo} label={s.sections.customers} sublabel={texts.home.customers} onPress={go('/admin/[businessId]/customers')} />
      </Group>

      <Group label={s.groups.business} separatorInset={58}>
        <Cell icon="construct" tint={ui.purple} label={term(business.vertical, 'catalog') ?? ''} sublabel={texts.home.catalog} onPress={go('/admin/[businessId]/catalog')} />
        {has('staff') && (
          <Cell icon="person" tint={ui.success} label={s.sections.staff} onPress={go('/admin/[businessId]/staff')} />
        )}
        {business.role === 'owner' ? (
          <Cell icon="stats-chart" tint={ui.success} label={strings.finances.title} sublabel={strings.finances.hint} onPress={go('/admin/[businessId]/finances')} />
        ) : null}
        <Cell icon="time" tint={ui.gray} label={s.sections.hours} sublabel={s.hints.hours} onPress={go('/admin/[businessId]/hours')} />
        {business.role === 'owner' ? (
          <Cell icon="settings" tint={ui.gray} label={s.sections.settings} sublabel={texts.home.settings} onPress={go('/admin/[businessId]/settings')} />
        ) : null}
      </Group>
    </Screen>
  );
}
