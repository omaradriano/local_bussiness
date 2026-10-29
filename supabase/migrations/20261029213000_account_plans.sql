-- Plan de la cuenta del dueño (cubre todos sus negocios). Sin plan gratis: al abrir su primer negocio
-- el dueño recibe una prueba de Pro (la API la crea); cuando termina la prueba o el plan (ends_at), la
-- cuenta queda de solo lectura hasta que el superadministrador le asigne un plan. ends_at null = sin
-- vencimiento (pilotos y cortesías). Qué incluye cada plan vive en services/api/internal/plans/plans.json.
-- businesses.plan (de la primera migración) queda sin uso.
create table public.account_plans (
  user_id     uuid primary key references auth.users (id) on delete cascade,
  plan        text not null check (plan in ('trial', 'lite', 'pro', 'business')),
  ends_at     timestamptz,
  note        text check (note is null or length(note) <= 200),
  updated_at  timestamptz not null default now()
);
alter table public.account_plans enable row level security;
create policy own_read on public.account_plans for select to authenticated using (user_id = (select auth.uid()));

-- Los dueños que ya existen (pilotos) siguen igual: Pro sin vencimiento.
insert into public.account_plans (user_id, plan)
select distinct user_id, 'pro' from public.business_members where role = 'owner'
on conflict do nothing;
