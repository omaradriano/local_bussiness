-- Venta de productos además del servicio (ceras y navajas en una barbería; medidores y tornillos en
-- una renta de herramienta): inventario con costo y existencias, ventas y compras de mercancía.

create table public.retail_products (
  id           uuid primary key default gen_random_uuid(),
  business_id  uuid not null references public.businesses (id) on delete cascade,
  name         text not null check (length(trim(name)) between 1 and 80),
  price        bigint not null check (price >= 0),          -- precio de venta
  cost         bigint not null default 0 check (cost >= 0), -- costo de compra (el último)
  stock        int not null default 0 check (stock >= 0),
  active       boolean not null default true,
  created_at   timestamptz not null default now(),
  unique (id, business_id)
);
create index on public.retail_products (business_id, name);

create table public.sales (
  id           uuid primary key default gen_random_uuid(),
  business_id  uuid not null references public.businesses (id) on delete cascade,
  booking_id   uuid, -- vendida durante una cita o renta (opcional)
  sold_at      timestamptz not null default now(),
  method       text not null check (method in ('cash', 'transfer', 'card')),
  total        bigint not null check (total >= 0),
  cost_total   bigint not null check (cost_total >= 0), -- costo de lo vendido, para la ganancia
  created_by   uuid references auth.users (id) on delete set null,
  unique (id, business_id),
  foreign key (booking_id, business_id) references public.bookings (id, business_id) on delete set null (booking_id)
);
create index on public.sales (business_id, sold_at);

create table public.sale_lines (
  id           uuid primary key default gen_random_uuid(),
  business_id  uuid not null,
  sale_id      uuid not null,
  product_id   uuid not null,
  quantity     int not null check (quantity between 1 and 1000),
  unit_price   bigint not null check (unit_price >= 0),
  unit_cost    bigint not null check (unit_cost >= 0),
  foreign key (sale_id, business_id) references public.sales (id, business_id) on delete cascade,
  foreign key (product_id, business_id) references public.retail_products (id, business_id)
);
create index on public.sale_lines (sale_id);
create index on public.sale_lines (product_id);

-- Compra de mercancía: suma existencias y cuenta como gasto («Mercancía») en su fecha.
create table public.stock_purchases (
  id            uuid primary key default gen_random_uuid(),
  business_id   uuid not null,
  product_id    uuid not null,
  quantity      int not null check (quantity between 1 and 100000),
  unit_cost     bigint not null check (unit_cost >= 0),
  purchased_on  date not null,
  created_by    uuid references auth.users (id) on delete set null,
  created_at    timestamptz not null default now(),
  foreign key (product_id, business_id) references public.retail_products (id, business_id) on delete cascade
);
create index on public.stock_purchases (business_id, purchased_on);

alter table public.retail_products enable row level security;
alter table public.sales enable row level security;
alter table public.sale_lines enable row level security;
alter table public.stock_purchases enable row level security;
create policy members_read on public.retail_products for select to authenticated using (private.is_member(business_id));
create policy members_read on public.sales for select to authenticated using (private.is_member(business_id));
create policy members_read on public.sale_lines for select to authenticated using (private.is_member(business_id));
create policy owners_read on public.stock_purchases for select to authenticated using (private.is_member(business_id, array['owner']));
