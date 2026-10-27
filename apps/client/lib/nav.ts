import { parentPath } from '@lb/shared';
import { router, type Href } from 'expo-router';

/** Back in history, or up to the previous menu when there is none (web reload or direct link). */
export function goBack(pathname: string) {
  if (router.canGoBack()) router.back();
  else router.replace((parentPath(pathname) ?? '/admin') as Href);
}
