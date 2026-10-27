/**
 * Resize target so the longest side is at most `max`, keeping the aspect ratio. Both sides are given
 * explicitly (expo-image-manipulator on web turns a null side into NaN). null when already small enough.
 */
export function fitWithin(width: number, height: number, max = 2048): { width: number; height: number } | null {
  if (width <= max && height <= max) return null;
  const scale = max / Math.max(width, height);
  return { width: Math.max(1, Math.round(width * scale)), height: Math.max(1, Math.round(height * scale)) };
}
