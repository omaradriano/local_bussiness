import type { Membership } from '@lb/api-client';
import { useCallback, useEffect, useState } from 'react';
import { api } from './api';
import { errorText } from './errors';
import { supabase } from './supabase';

// Last answer, shown right away by the next screen (and the header) while it refreshes.
let cached: Membership[] | null = null;

/** The signed-in user's businesses (GET /v1/me). */
export function useMe() {
  const [businesses, setBusinesses] = useState<Membership[] | null>(cached);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    setError(null);
    try {
      const { data, error, response } = await api.GET('/v1/me');
      if (data) { cached = data.businesses; setBusinesses(data.businesses); }
      else setError(errorText(error, response.status));
    } catch {
      setError(errorText(undefined));
    }
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  return { businesses, error, reload };
}

export function useBusiness(businessId: string) {
  const me = useMe();
  return { ...me, business: me.businesses?.find((b) => b.business_id === businessId) ?? null };
}

/** Sign out and forget the cached businesses (the next account must not see them). */
export async function signOut() {
  cached = null;
  await supabase.auth.signOut();
}
