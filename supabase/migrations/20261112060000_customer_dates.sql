-- Fechas importantes del cliente (tanda D2): se repiten cada año; Hoy avisa 7 días antes.
create table public.customer_dates (
  id           uuid primary key default gen_random_uuid(),
  business_id  uuid not null,
  customer_id  uuid not null,
  label        text not null check (length(trim(label)) between 1 and 80),
  month        int not null check (month between 1 and 12),
  day          int not null check (day between 1 and 31),
  created_at   timestamptz not null default now(),
  foreign key (customer_id, business_id) references public.customers (id, business_id) on delete cascade
);
create index on public.customer_dates (business_id);

alter table public.customer_dates enable row level security;
create policy members_read on public.customer_dates for select to authenticated using (private.is_member(business_id, array['owner', 'staff']));
