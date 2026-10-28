import { strings, term } from '@lb/shared';
import { router, Stack } from 'expo-router';
import { ActivityIndicator } from 'react-native';
import { signOut, useMe } from '../../../lib/useMe';
import { Button, Cell, EmptyState, Group, Notice, Screen, Text, useUI } from '../../../ui';

const t = strings.admin;

export default function AdminHome() {
  const { businesses, error, reload } = useMe();
  const ui = useUI();

  return (
    <Screen>
      <Stack.Screen options={{ title: t.title }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 20 }}>{t.title}</Text>
      {error ? (
        <Group padded>
          <Notice tone="warning">{error}</Notice>
          <Button onPress={reload} style={{ marginTop: 12 }}>{strings.common.retry}</Button>
        </Group>
      ) : businesses === null ? (
        <ActivityIndicator accessibilityLabel={strings.common.loading} />
      ) : businesses.length === 0 ? (
        <EmptyState icon="storefront-outline" title={t.noBusinesses}
          action={<Button variant="primary" icon="add-circle" onPress={() => router.push('/onboarding')}>{t.createBusiness}</Button>} />
      ) : (
        <Group label={t.yourBusinesses} separatorInset={58}>
          {businesses.map((b) => (
            <Cell key={b.business_id} icon="storefront" tint={ui.accent} label={b.name}
              sublabel={`${term(b.vertical, 'catalog')} · ${t.role[b.role]}`}
              onPress={() => router.push({ pathname: '/admin/[businessId]', params: { businessId: b.business_id } })} />
          ))}
        </Group>
      )}
      {businesses && businesses.length > 0 && (
        <Group>
          <Cell icon="add-circle" tint={ui.success} label={t.createAnother} onPress={() => router.push('/onboarding')} />
        </Group>
      )}
      <Group>
        <Cell label={t.signOut} destructive onPress={() => signOut()} />
      </Group>
    </Screen>
  );
}
