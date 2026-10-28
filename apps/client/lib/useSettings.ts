import type { components } from '@lb/api-client';
import { useCallback, useEffect, useState } from 'react';
import { api } from './api';

export type Settings = components['schemas']['Settings'];

/** Business settings (GET /v1/b/{id}/settings); null while loading. */
export function useSettings(businessId: string) {
  const [settings, setSettings] = useState<Settings | null>(null);
  const reload = useCallback(async () => {
    const { data } = await api.GET('/v1/b/{businessId}/settings', { params: { path: { businessId } } });
    if (data) setSettings(data);
  }, [businessId]);
  useEffect(() => { reload(); }, [reload]);
  return { settings, setSettings, reload };
}
