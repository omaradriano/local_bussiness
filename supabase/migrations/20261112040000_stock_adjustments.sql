-- Mermas y ajustes de conteo (tanda C2): cada cambio de existencias que no es compra ni venta.
create table public.stock_adjustments (
  id           uuid primary key default gen_random_uuid(),
  business_id  uuid not null,
  product_id   uuid not null,
  reason       text not null check (reason in ('waste', 'count')), -- merma | conteo
  quantity     int not null check (quantity <> 0 and quantity between -100000 and 100000), -- + entró, − salió
  unit_cost    bigint not null check (unit_cost >= 0), -- costo del producto al ajustar, para valuar la merma
  note         text check (note is null or length(note) <= 200),
  created_by   uuid references auth.users (id) on delete set null,
  created_at   timestamptz not null default now(),
  foreign key (product_id, business_id) references public.retail_products (id, business_id) on delete cascade
);
create index on public.stock_adjustments (business_id, created_at);

alter table public.stock_adjustments enable row level security;
create policy owners_read on public.stock_adjustments for select to authenticated using (private.is_member(business_id, array['owner']));
