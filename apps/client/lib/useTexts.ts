import { hasFeature, kindFor, textsFor, type Feature, type ItemKind, type Vertical, type VerticalTexts } from '@lb/shared';
import { usePathname } from 'expo-router';
import { useBusiness } from './useMe';

const IN_BUSINESS = /^\/admin\/([0-9a-f-]{36})(\/.*)?$/;

/** The business the current screen belongs to (from the URL), or null outside one. */
export function useCurrentBusinessId(): { businessId: string | null; rest: string } {
  const m = IN_BUSINESS.exec(usePathname());
  return { businessId: m?.[1] ?? null, rest: m?.[2] ?? '' };
}

/**
 * The current business's giro map: its texts ("Rentas" / "Citas" / "Pedidos"), engine and features.
 * Screens ask `has('guarantor')` instead of "is it a rental?" (active features of this business).
 * Until the business loads it uses the rental map (the first giro).
 */
export function useTexts(): { vertical: Vertical; kind: ItemKind; texts: VerticalTexts; has: (f: Feature) => boolean } {
  const { businessId } = useCurrentBusinessId();
  const { business } = useBusiness(businessId ?? '');
  const vertical: Vertical = business?.vertical ?? 'tool_rental';
  // Active features come from the API (giro + platform admin's grants + the owner's choice);
  // until the business loads, the giro's defaults.
  const has = (f: Feature) => (business ? business.features.includes(f) : hasFeature(vertical, f));
  return { vertical, kind: kindFor(vertical), texts: textsFor(vertical), has };
}
