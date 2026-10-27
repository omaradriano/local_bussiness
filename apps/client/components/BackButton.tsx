import { parentPath, strings } from '@lb/shared';
import { usePathname } from 'expo-router';
import { Pressable, Text } from 'react-native';
import { goBack } from '../lib/nav';

/** Header arrow to the previous menu; hidden at the panel root. */
export function BackButton() {
  const pathname = usePathname();
  if (!parentPath(pathname) && pathname !== '/onboarding') return null;

  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={strings.common.back}
      hitSlop={12}
      onPress={() => goBack(pathname)}
      style={{ paddingHorizontal: 12, paddingVertical: 4 }}
    >
      <Text style={{ fontSize: 22 }}>←</Text>
    </Pressable>
  );
}
