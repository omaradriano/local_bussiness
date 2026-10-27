import type { ApiError } from '@lb/api-client';
import { strings } from '@lb/shared';

/** API error -> text for the user. Without an API message (network, non-JSON), show the status to ease debugging. */
export function errorText(error: ApiError | undefined, status?: number): string {
  if (error?.message) return error.message;
  return status ? `${strings.common.error} (${status})` : strings.common.error;
}

/** Field errors from a 422/409 `details`, if any. */
export function fieldErrors(error: ApiError | undefined): Record<string, string> {
  const d = error?.details;
  return d && typeof d === 'object' ? (d as Record<string, string>) : {};
}
