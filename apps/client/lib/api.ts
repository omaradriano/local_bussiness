import { createApi } from '@lb/api-client';
import { supabase } from './supabase';

const baseUrl = process.env.EXPO_PUBLIC_API_URL;
if (!baseUrl) throw new Error('Falta EXPO_PUBLIC_API_URL (ver apps/client/.env.example)');

export const api = createApi(baseUrl, async () => {
  const { data } = await supabase.auth.getSession();
  return data.session?.access_token ?? null;
});
