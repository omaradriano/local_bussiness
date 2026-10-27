/**
 * Resize target so the longest side is at most `max` (keeps aspect: the other side is null).
 * null when the image is already small enough.
 */
export function fitWithin(width: number, height: number, max = 2048): { width: number | null; height: number | null } | null {
  if (width <= max && height <= max) return null;
  return width >= height ? { width: max, height: null } : { width: null, height: max };
}
