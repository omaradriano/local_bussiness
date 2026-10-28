import { kindFor, textsFor, type ItemKind, type Vertical, type VerticalTexts } from '@lb/shared';
import { usePathname } from 'expo-router';
import { useBusiness } from './useMe';

const IN_BUSINESS = /^\/admin\/([0-9a-f-]{36})(\/.*)?$/;

/** The business the current screen belongs to (from the URL), or null outside one. */
export function useCurrentBusinessId(): { businessId: string | null; rest: string } {
  const m = IN_BUSINESS.exec(usePathname());
  return { businessId: m?.[1] ?? null, rest: m?.[2] ?? '' };
}

/**
 * Texts of the current business's type (rentals say "Rentas", nails "Citas", flowers "Pedidos"),
 * with its vertical and item kind. Until the business loads it uses rental texts (the first vertical).
 */
export function useTexts(): { vertical: Vertical; kind: ItemKind; texts: VerticalTexts; isRental: boolean } {
  const { businessId } = useCurrentBusinessId();
  const { business } = useBusiness(businessId ?? '');
  const vertical: Vertical = business?.vertical ?? 'tool_rental';
  const kind = kindFor(vertical);
  return { vertical, kind, texts: textsFor(vertical), isRental: kind === 'rental' };
}
