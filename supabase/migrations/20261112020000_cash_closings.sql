-- Corte de caja (tanda B2): el efectivo contado al cerrar un día contra lo que esperaba el reporte.
create table public.cash_closings (
  id           uuid primary key default gen_random_uuid(),
  business_id  uuid not null references public.businesses (id) on delete cascade,
  day          date not null, -- fecha local del negocio
  counted      bigint not null check (counted >= 0),
  expected     bigint not null, -- puede ser negativo si salió más de lo que entró
  note         text check (note is null or length(note) <= 200),
  closed_by    uuid references auth.users (id) on delete set null,
  closed_at    timestamptz not null default now(),
  unique (business_id, day)
);

alter table public.cash_closings enable row level security;
create policy owners_read on public.cash_closings for select to authenticated using (private.is_member(business_id, array['owner']));
