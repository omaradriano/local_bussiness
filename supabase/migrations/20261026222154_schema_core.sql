-- Fase 1 · parte 1: esquema núcleo (negocios, equipo, horarios, catálogo, unidades, clientes).
-- Montos en centavos (bigint, MXN). Fechas en timestamptz (UTC); horarios del negocio en su zona.
-- Cada tabla hija lleva business_id y una FK compuesta (x_id, business_id) hacia su padre,
-- así Postgres impide referencias cruzadas entre negocios aunque la API tenga un bug.

create extension if not exists btree_gist with schema extensions;

-- ---------------------------------------------------------------- negocios

create table public.businesses (
  id              uuid primary key default gen_random_uuid(),
  slug            text not null unique check (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$' and length(slug) between 3 and 60),
  name            text not null check (length(trim(name)) > 0),
  vertical        text not null check (vertical in ('nails', 'flowers', 'tool_rental')),
  timezone        text not null default 'America/Chihuahua',
  phone_whatsapp  text check (phone_whatsapp ~ '^\+[1-9][0-9]{7,14}$'),
  currency        text not null default 'MXN' check (currency = 'MXN'),
  settings        jsonb not null default '{}'::jsonb check (jsonb_typeof(settings) = 'object'),
  plan            text not null default 'trial',
  status          text not null default 'active' check (status in ('active', 'suspended')),
  created_at      timestamptz not null default now()
);

create table public.staff (
  id           uuid primary key default gen_random_uuid(),
  business_id  uuid not null references public.businesses (id) on delete cascade,
  name         text not null check (length(trim(name)) > 0),
  active       boolean not null default true,
  color        text,
  created_at   timestamptz not null default now(),
  unique (id, business_id)
);

create table public.business_members (
  business_id  uuid not null references public.businesses (id) on delete cascade,
  user_id      uuid not null references auth.users (id) on delete cascade,
  role         text not null check (role in ('owner', 'staff')),
  staff_id     uuid,
  created_at   timestamptz not null default now(),
  primary key (business_id, user_id),
  foreign key (staff_id, business_id) references public.staff (id, business_id) on delete set null (staff_id)
);
create index on public.business_members (user_id);

-- staff_id null = horario general del negocio.
create table public.business_hours (
  id           uuid primary key default gen_random_uuid(),
  business_id  uuid not null references public.businesses (id) on delete cascade,
  staff_id     uuid,
  weekday      smallint not null check (weekday between 0 and 6), -- 0 = domingo
  opens_at     time not null,
  closes_at    time not null,
  check (closes_at > opens_at),
  foreign key (staff_id, business_id) references public.staff (id, business_id) on delete cascade
);
create index on public.business_hours (business_id, weekday);

-- ---------------------------------------------------------------- catálogo

create table public.catalog_items (
  id                uuid primary key default gen_random_uuid(),
  business_id       uuid not null references public.businesses (id) on delete cascade,
  kind              text not null check (kind in ('service', 'product', 'rental')),
  name              text not null check (length(trim(name)) > 0),
  description       text,
  price             bigint not null check (price >= 0),
  active            boolean not null default true,
  sort              integer not null default 0,
  -- service
  duration_minutes  integer check (duration_minutes > 0),
  buffer_minutes    integer not null default 0 check (buffer_minutes >= 0),
  -- product
  prep_hours        integer check (prep_hours >= 0),
  daily_capacity    integer check (daily_capacity > 0),
  -- rental
  price_unit        text check (price_unit in ('day', 'weekend', 'week')),
  security_deposit  bigint check (security_deposit >= 0),
  late_fee_per_day  bigint check (late_fee_per_day >= 0),
  created_at        timestamptz not null default now(),
  unique (id, business_id),
  constraint catalog_items_fields_by_kind check (
    case kind
      when 'service' then duration_minutes is not null
      when 'product' then prep_hours is not null
      when 'rental'  then price_unit is not null and security_deposit is not null and late_fee_per_day is not null
    end
  )
);
create index on public.catalog_items (business_id, active, sort);

create table public.catalog_item_images (
  id           uuid primary key default gen_random_uuid(),
  business_id  uuid not null,
  item_id      uuid not null,
  s3_key       text not null unique,
  variants     jsonb not null default '{}'::jsonb,
  width        integer check (width > 0),
  height       integer check (height > 0),
  blurhash     text,
  status       text not null default 'processing' check (status in ('processing', 'ready', 'failed')),
  sort         integer not null default 0,
  created_at   timestamptz not null default now(),
  foreign key (item_id, business_id) references public.catalog_items (id, business_id) on delete cascade
);
create index on public.catalog_item_images (item_id, sort);

create table public.item_options (
  id           uuid primary key default gen_random_uuid(),
  business_id  uuid not null,
  item_id      uuid not null,
  name         text not null check (length(trim(name)) > 0),
  type         text not null check (type in ('text', 'choice')),
  choices      jsonb check (choices is null or jsonb_typeof(choices) = 'array'),
  extra_price  bigint not null default 0 check (extra_price >= 0),
  required     boolean not null default false,
  sort         integer not null default 0,
  check (type <> 'choice' or choices is not null),
  foreign key (item_id, business_id) references public.catalog_items (id, business_id) on delete cascade
);
create index on public.item_options (item_id);

create table public.staff_services (
  business_id  uuid not null,
  staff_id     uuid not null,
  item_id      uuid not null,
  primary key (staff_id, item_id),
  foreign key (staff_id, business_id) references public.staff (id, business_id) on delete cascade,
  foreign key (item_id, business_id) references public.catalog_items (id, business_id) on delete cascade
);
create index on public.staff_services (item_id);

create table public.item_units (
  id           uuid primary key default gen_random_uuid(),
  business_id  uuid not null,
  item_id      uuid not null,
  code         text not null check (length(trim(code)) > 0),
  status       text not null default 'available' check (status in ('available', 'maintenance', 'retired')),
  notes        text,
  created_at   timestamptz not null default now(),
  unique (item_id, code),
  unique (id, business_id),
  foreign key (item_id, business_id) references public.catalog_items (id, business_id) on delete cascade
);

-- Armadores de producto (ej. ramo: grupo "Flores" de 12 a 36 piezas).
create table public.item_component_groups (
  id           uuid primary key default gen_random_uuid(),
  business_id  uuid not null,
  item_id      uuid not null,
  name         text not null check (length(trim(name)) > 0),
  min_total    integer not null default 0 check (min_total >= 0),
  max_total    integer check (max_total > 0),
  required     boolean not null default false,
  sort         integer not null default 0,
  unique (id, business_id),
  check (max_total is null or max_total >= min_total),
  foreign key (item_id, business_id) references public.catalog_items (id, business_id) on delete cascade
);

create table public.item_components (
  id           uuid primary key default gen_random_uuid(),
  business_id  uuid not null,
  group_id     uuid not null,
  name         text not null check (length(trim(name)) > 0),
  unit_price   bigint not null check (unit_price >= 0),
  max_qty      integer check (max_qty > 0),
  active       boolean not null default true,
  foreign key (group_id, business_id) references public.item_component_groups (id, business_id) on delete cascade
);
create index on public.item_components (group_id);

-- staff_id / item_unit_id nulos = cierre de todo el negocio.
create table public.blackouts (
  id            uuid primary key default gen_random_uuid(),
  business_id   uuid not null references public.businesses (id) on delete cascade,
  staff_id      uuid,
  item_unit_id  uuid,
  starts_at     timestamptz not null,
  ends_at       timestamptz not null,
  reason        text,
  check (ends_at > starts_at),
  check (staff_id is null or item_unit_id is null),
  foreign key (staff_id, business_id) references public.staff (id, business_id) on delete cascade,
  foreign key (item_unit_id, business_id) references public.item_units (id, business_id) on delete cascade
);
create index on public.blackouts using gist (business_id, tstzrange(starts_at, ends_at));

-- ---------------------------------------------------------------- clientes

create table public.customers (
  id                   uuid primary key default gen_random_uuid(),
  business_id          uuid not null references public.businesses (id) on delete cascade,
  name                 text not null check (length(trim(name)) > 0),
  phone                text not null check (phone ~ '^\+[1-9][0-9]{7,14}$'), -- E.164
  email                text,
  notes                text,
  id_document_key      text, -- INE en el bucket privado; solo rentas
  preferred_channel    text check (preferred_channel in ('whatsapp', 'sms', 'email')),
  consent_whatsapp_at  timestamptz,
  consent_sms_at       timestamptz,
  consent_email_at     timestamptz,
  privacy_accepted_at  timestamptz,
  privacy_version      text,
  created_at           timestamptz not null default now(),
  unique (business_id, phone),
  unique (id, business_id)
);

-- RLS activo desde ya y sin políticas: nadie fuera del service role lee nada hasta la parte 3.
alter table public.businesses            enable row level security;
alter table public.staff                 enable row level security;
alter table public.business_members      enable row level security;
alter table public.business_hours        enable row level security;
alter table public.catalog_items         enable row level security;
alter table public.catalog_item_images   enable row level security;
alter table public.item_options          enable row level security;
alter table public.staff_services        enable row level security;
alter table public.item_units            enable row level security;
alter table public.item_component_groups enable row level security;
alter table public.item_components       enable row level security;
alter table public.blackouts             enable row level security;
alter table public.customers             enable row level security;
