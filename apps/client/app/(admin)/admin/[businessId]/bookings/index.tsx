import { strings } from '@lb/shared';
import { Stack, useLocalSearchParams } from 'expo-router';
import { BookingHistory } from '../../../../../components/BookingHistory';
import { Screen, Text } from '../../../../../ui';

/** Every booking of the business, newest first, with filters and search. */
export default function History() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  return (
    <Screen>
      <Stack.Screen options={{ title: strings.history.title }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 16 }}>{strings.history.title}</Text>
      <BookingHistory businessId={businessId} />
    </Screen>
  );
}
