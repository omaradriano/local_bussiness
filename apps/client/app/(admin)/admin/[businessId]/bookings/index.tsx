import { Stack, useLocalSearchParams } from 'expo-router';
import { BookingHistory } from '../../../../../components/BookingHistory';
import { useTexts } from '../../../../../lib/useTexts';
import { Screen, Text } from '../../../../../ui';

/** Every booking of the business, newest first, with filters and search. */
export default function History() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const { texts } = useTexts();
  return (
    <Screen>
      <Stack.Screen options={{ title: texts.bookings.plural }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 16 }}>{texts.bookings.plural}</Text>
      <BookingHistory businessId={businessId} />
    </Screen>
  );
}
