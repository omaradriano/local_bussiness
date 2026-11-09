import { Ionicons } from '@expo/vector-icons';
import { parentPath, strings } from '@lb/shared';
import { usePathname } from 'expo-router';
import { goBack } from '../lib/nav';
import { useIsSection } from './AdminNav';
import { PressableScale, useUI } from '../ui';

/** Header arrow to the previous menu; hidden at the panel root and on the menu's sections. */
export function BackButton() {
  const pathname = usePathname();
  const ui = useUI();
  const isSection = useIsSection();
  if ((!parentPath(pathname) && pathname !== '/onboarding') || isSection) return null;

  return (
    <PressableScale accessibilityRole="button" accessibilityLabel={strings.common.back} hitSlop={12}
      onPress={() => goBack(pathname)} style={{ paddingHorizontal: 8, paddingVertical: 4 }}>
      <Ionicons name="chevron-back" size={26} color={ui.accent} />
    </PressableScale>
  );
}
