import createClient from 'openapi-fetch';
import type { components, paths } from './schema';

export type { components, paths };
export type ApiError = components['schemas']['Error'];
export type Membership = components['schemas']['Membership'];

/**
 * Typed API client. `getToken` returns the current Supabase access token (or null when signed out);
 * it is read on every request so refreshed tokens are picked up.
 */
export function createApi(baseUrl: string, getToken: () => Promise<string | null>, fetchImpl?: typeof fetch) {
  const client = createClient<paths>({ baseUrl, fetch: fetchImpl });
  client.use({
    async onRequest({ request }) {
      const token = await getToken();
      if (token) request.headers.set('Authorization', `Bearer ${token}`);
      return request;
    },
    // Network failures (offline, CORS, server down) come back as a normal API error instead of throwing.
    onError() {
      const body: ApiError = { code: 'network', message: 'No hay conexión con el servidor. Revisa tu internet e intenta de nuevo.' };
      return new Response(JSON.stringify(body), { status: 503, headers: { 'Content-Type': 'application/json' } });
    },
  });
  return client;
}

export type Api = ReturnType<typeof createApi>;
