import type { Membership } from '@lb/api-client';
import { useCallback, useEffect, useState } from 'react';
import { api } from './api';
import { errorText } from './errors';
import { supabase } from './supabase';

// Last answer, shown right away by the next screen (and the header) while it refreshes.
let cached: Membership[] | null = null;
let cachedAdmin = false;
// One request at a time for every screen, header and sidebar mounted together; each hears the answer.
let inflight: Promise<void> | null = null;
const listeners = new Set<() => void>();

function fetchMe(): Promise<void> {
  inflight ??= api.GET('/v1/me').then(({ data, error, response }) => {
    if (data) {
      cached = data.businesses;
      cachedAdmin = data.platform_admin;
      listeners.forEach((l) => l());
    }
    // A failure leaves the cache as it was; each caller shows the error.
    if (!data) throw new Error(errorText(error, response.status));
  }).finally(() => { inflight = null; });
  return inflight;
}

/** The signed-in user's businesses (GET /v1/me). */
export function useMe() {
  const [businesses, setBusinesses] = useState<Membership[] | null>(cached);
  const [platformAdmin, setPlatformAdmin] = useState(cachedAdmin);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const hear = () => { setBusinesses(cached); setPlatformAdmin(cachedAdmin); setError(null); };
    listeners.add(hear);
    return () => { listeners.delete(hear); };
  }, []);

  const reload = useCallback(async () => {
    setError(null);
    try {
      await fetchMe();
    } catch (e) {
      setError(e instanceof Error && e.message ? e.message : errorText(undefined));
    }
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  return { businesses, platformAdmin, error, reload };
}

export function useBusiness(businessId: string) {
  const me = useMe();
  return { ...me, business: me.businesses?.find((b) => b.business_id === businessId) ?? null };
}

/** Sign out and forget the cached businesses (the next account must not see them). */
export async function signOut() {
  cached = null;
  cachedAdmin = false;
  await supabase.auth.signOut();
}
