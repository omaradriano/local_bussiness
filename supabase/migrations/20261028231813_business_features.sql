-- Funciones por negocio y superadministrador de la plataforma.

-- Quién administra la plataforma (no un negocio). Se da de alta a mano:
--   insert into public.platform_admins (user_id) select id from auth.users where email = '...';
create table public.platform_admins (
  user_id     uuid primary key references auth.users (id) on delete cascade,
  created_at  timestamptz not null default now()
);
alter table public.platform_admins enable row level security;
create policy self_read on public.platform_admins for select to authenticated using (user_id = auth.uid());

-- Funciones que el superadministrador da (true) o quita (false) a un negocio, por encima de su giro.
-- Va aparte de settings porque el dueño no la puede cambiar; el dueño solo enciende o apaga las
-- disponibles en settings.features.
alter table public.businesses add column feature_grants jsonb not null default '{}'::jsonb
  check (jsonb_typeof(feature_grants) = 'object');

-- Giro nuevo: barbería (agenda por barbero).
alter table public.businesses drop constraint businesses_vertical_check;
alter table public.businesses add constraint businesses_vertical_check
  check (vertical in ('nails', 'flowers', 'tool_rental', 'barbershop'));
alter table public.notification_templates drop constraint notification_templates_vertical_check;
alter table public.notification_templates add constraint notification_templates_vertical_check
  check (vertical in ('nails', 'flowers', 'tool_rental', 'barbershop'));
