-- Campos personalizados por negocio y motor (módulo de motor): el dueño define nombre, tipo y, para
-- listas, sus opciones; cada campo es del artículo (marca, modelo) o de cada unidad (número de
-- serie, condición). Los valores viven en custom_values (jsonb, clave = id del campo); la API los
-- valida contra la definición.
create table public.custom_fields (
  id               uuid primary key default gen_random_uuid(),
  business_id      uuid not null references public.businesses (id) on delete cascade,
  applies_to_kind  text not null check (applies_to_kind in ('service', 'product', 'rental')),
  target           text not null check (target in ('item', 'unit')),
  label            text not null check (length(trim(label)) between 1 and 60),
  type             text not null check (type in ('text', 'number', 'select', 'date', 'boolean')),
  options          jsonb not null default '[]'::jsonb check (jsonb_typeof(options) = 'array'),
  required         boolean not null default false,
  active           boolean not null default true,
  sort             int not null default 0,
  created_at       timestamptz not null default now(),
  unique (id, business_id) -- «de cada unidad» solo en motores con unidades: lo valida la API
);
create index on public.custom_fields (business_id, applies_to_kind, sort);

alter table public.custom_fields enable row level security;
create policy members_read on public.custom_fields for select to authenticated using (private.is_member(business_id));

alter table public.catalog_items add column custom_values jsonb not null default '{}'::jsonb
  check (jsonb_typeof(custom_values) = 'object');
alter table public.item_units add column custom_values jsonb not null default '{}'::jsonb
  check (jsonb_typeof(custom_values) = 'object');
