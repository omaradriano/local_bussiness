import type { Membership } from '@lb/api-client';
import { useCallback, useEffect, useState } from 'react';
import { api } from './api';
import { errorText } from './errors';

/** The signed-in user's businesses (GET /v1/me). */
export function useMe() {
  const [businesses, setBusinesses] = useState<Membership[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    setError(null);
    try {
      const { data, error, response } = await api.GET('/v1/me');
      if (data) setBusinesses(data.businesses);
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
