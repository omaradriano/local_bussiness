import { describe, expect, it } from 'vitest';
import { createApi } from './index';

function recordingFetch(body: unknown, status = 200) {
  const seen: Request[] = [];
  const fetchImpl = (async (input: Request) => {
    seen.push(input);
    return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
  }) as typeof fetch;
  return { seen, fetchImpl };
}

describe('createApi', () => {
  it('sends the session token as Bearer', async () => {
    const { seen, fetchImpl } = recordingFetch({ user_id: 'u', businesses: [] });
    const api = createApi('https://api.test', async () => 'tok123', fetchImpl);
    const { data } = await api.GET('/v1/me');
    expect(seen[0].headers.get('Authorization')).toBe('Bearer tok123');
    expect(seen[0].url).toBe('https://api.test/v1/me');
    expect(data?.businesses).toEqual([]);
  });

  it('sends no Authorization header when signed out', async () => {
    const { seen, fetchImpl } = recordingFetch({ code: 'unauthorized', message: 'x' }, 401);
    const api = createApi('https://api.test', async () => null, fetchImpl);
    const { error, response } = await api.GET('/v1/me');
    expect(seen[0].headers.has('Authorization')).toBe(false);
    expect(response.status).toBe(401);
    expect(error?.code).toBe('unauthorized');
  });
});
