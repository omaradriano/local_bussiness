-- Fase 1 · parte 2: reservas (citas, pedidos y rentas), recursos ocupados, pagos, rentas,
-- bitácora, pasos configurables y avisos. La cola de jobs es River (schema river).
-- Igual que en schema_core: business_id + FK compuesta en cada hija para no cruzar negocios.

-- ---------------------------------------------------------------- reservas

create table public.bookings (
  id                uuid primary key default gen_random_uuid(),
  business_id       uuid not null references public.businesses (id) on delete cascade,
  customer_id       uuid not null,
  kind              text not null check (kind in ('service', 'product', 'rental')),
  -- Una sola máquina de estados; cada tipo usa solo los suyos (la valida la API en internal/bookings/machine.go).
  status            text not null check (status in (
                      'pending_deposit', 'confirmed', 'in_progress', 'ready', 'out_for_delivery',
                      'checked_out', 'overdue', 'returned', 'completed', 'cancelled', 'expired', 'no_show')),
  sub_status        text, -- etiqueta libre por negocio, sin efecto en la lógica
  starts_at         timestamptz not null,
  ends_at           timestamptz,
  fulfillment       text check (fulfillment in ('pickup', 'delivery')),
  delivery_address  text,
  delivery_window   tstzrange,
  subtotal          bigint not null default 0 check (subtotal >= 0),
  deposit_required  bigint not null default 0 check (deposit_required >= 0),
  total             bigint not null default 0 check (total >= 0),
  notes             text,
  source            text not null check (source in ('public', 'admin')),
  public_token      text not null unique default encode(extensions.gen_random_bytes(32), 'hex'),
  hold_expires_at   timestamptz, -- apartado sin anticipo: expira y libera recursos
  created_at        timestamptz not null default now(),
  updated_at        timestamptz not null default now(),
  unique (id, business_id),
  check (ends_at is null or ends_at > starts_at),
  check (fulfillment <> 'delivery' or delivery_address is not null),
  foreign key (customer_id, business_id) references public.customers (id, business_id)
);
create index on public.bookings (business_id, starts_at);
create index on public.bookings (business_id, status);
create index on public.bookings (hold_expires_at) where status = 'pending_deposit';

create table public.booking_items (
  id           uuid primary key default gen_random_uuid(),
  business_id  uuid not null,
  booking_id   uuid not null,
  item_id      uuid not null,
  quantity     integer not null default 1 check (quantity > 0),
  unit_price   bigint not null check (unit_price >= 0),
  options      jsonb not null default '{}'::jsonb, -- opciones y composición congeladas con sus precios
  foreign key (booking_id, business_id) references public.bookings (id, business_id) on delete cascade,
  foreign key (item_id, business_id) references public.catalog_items (id, business_id)
);
create index on public.booking_items (booking_id);

-- Lo que queda ocupado. Las exclusiones impiden encimar una empleada o una unidad;
-- cancelar o expirar una reserva borra sus filas aquí y libera el recurso.
create table public.booking_resources (
  id            uuid primary key default gen_random_uuid(),
  business_id   uuid not null,
  booking_id    uuid not null,
  staff_id      uuid,
  item_unit_id  uuid,
  during        tstzrange not null check (not isempty(during)),
  check ((staff_id is null) <> (item_unit_id is null)),
  foreign key (booking_id, business_id) references public.bookings (id, business_id) on delete cascade,
  foreign key (staff_id, business_id) references public.staff (id, business_id),
  foreign key (item_unit_id, business_id) references public.item_units (id, business_id),
  constraint booking_resources_staff_no_overlap
    exclude using gist (staff_id with =, during with &&) where (staff_id is not null),
  constraint booking_resources_unit_no_overlap
    exclude using gist (item_unit_id with =, during with &&) where (item_unit_id is not null)
);
create index on public.booking_resources (booking_id);

-- Libro de pagos: saldo = total − suma(cobros) + suma(reembolsos). Montos siempre positivos; el tipo da el signo.
create table public.payments (
  id           uuid primary key default gen_random_uuid(),
  business_id  uuid not null,
  booking_id   uuid not null,
  type         text not null check (type in ('advance', 'balance', 'security_deposit', 'refund', 'late_fee', 'damage_fee')),
  method       text not null check (method in ('cash', 'transfer', 'card')),
  amount       bigint not null check (amount > 0),
  paid_at      timestamptz not null default now(),
  recorded_by  uuid references auth.users (id) on delete set null,
  notes        text,
  created_at   timestamptz not null default now(),
  foreign key (booking_id, business_id) references public.bookings (id, business_id) on delete cascade
);
create index on public.payments (booking_id);

create table public.rental_checkouts (
  id                uuid primary key default gen_random_uuid(),
  business_id       uuid not null,
  booking_id        uuid not null,
  item_unit_id      uuid not null,
  out_at            timestamptz not null default now(),
  out_condition     text,
  out_photos        text[] not null default '{}', -- llaves S3 (rental-evidence)
  returned_at       timestamptz,
  return_condition  text,
  return_photos     text[] not null default '{}',
  unique (booking_id, item_unit_id),
  check (returned_at is null or returned_at >= out_at),
  foreign key (booking_id, business_id) references public.bookings (id, business_id) on delete cascade,
  foreign key (item_unit_id, business_id) references public.item_units (id, business_id)
);

create table public.booking_events (
  id           uuid primary key default gen_random_uuid(),
  business_id  uuid not null,
  booking_id   uuid not null,
  from_status  text,
  to_status    text not null,
  actor        uuid references auth.users (id) on delete set null, -- null = cliente o sistema
  at           timestamptz not null default now(),
  note         text,
  foreign key (booking_id, business_id) references public.bookings (id, business_id) on delete cascade
);
create index on public.booking_events (booking_id, at);

-- ---------------------------------------------------------------- pasos configurables

create table public.workflow_steps (
  id               uuid primary key default gen_random_uuid(),
  business_id      uuid not null references public.businesses (id) on delete cascade,
  applies_to_kind  text not null check (applies_to_kind in ('service', 'product', 'rental')),
  item_id          uuid,
  actor            text not null check (actor in ('business', 'customer')),
  moment           text not null check (moment in ('on_booking', 'before_status')),
  to_status        text,
  step_type        text not null, -- se valida contra el registro StepType en Go
  label            text not null check (length(trim(label)) > 0),
  config           jsonb not null default '{}'::jsonb,
  condition        jsonb,
  required         boolean not null default true,
  sort             integer not null default 0,
  active           boolean not null default true,
  check ((moment = 'before_status') = (to_status is not null)),
  foreign key (item_id, business_id) references public.catalog_items (id, business_id) on delete cascade
);
create index on public.workflow_steps (business_id, applies_to_kind) where active;

-- Copia de los pasos aplicables al crear la reserva: cambiar la configuración no rompe reservas en curso.
create table public.booking_steps (
  id             uuid primary key default gen_random_uuid(),
  business_id    uuid not null,
  booking_id     uuid not null,
  step_snapshot  jsonb not null,
  status         text not null default 'pending' check (status in ('pending', 'done', 'skipped')),
  completed_by   uuid references auth.users (id) on delete set null,
  completed_at   timestamptz,
  data           jsonb,
  s3_keys        text[] not null default '{}',
  check ((status = 'pending') = (completed_at is null)),
  foreign key (booking_id, business_id) references public.bookings (id, business_id) on delete cascade
);
create index on public.booking_steps (booking_id);

-- ---------------------------------------------------------------- avisos

create table public.notification_rules (
  id            uuid primary key default gen_random_uuid(),
  business_id   uuid not null references public.businesses (id) on delete cascade,
  event         text not null,
  audience      text not null check (audience in ('customer', 'owner')),
  channels      text[] not null check (cardinality(channels) > 0), -- en orden de preferencia
  "offset"      interval not null default '0',
  enabled       boolean not null default true,
  template_key  text,
  unique (business_id, event, audience)
);

-- business_id null = plantilla por defecto del giro.
create table public.notification_templates (
  id           uuid primary key default gen_random_uuid(),
  business_id  uuid references public.businesses (id) on delete cascade,
  vertical     text check (vertical in ('nails', 'flowers', 'tool_rental')),
  event        text not null,
  channel      text not null,
  body         text not null,
  check ((business_id is null) = (vertical is not null))
);
create unique index on public.notification_templates (coalesce(business_id::text, vertical), event, channel);

create table public.notifications (
  id                   uuid primary key default gen_random_uuid(),
  business_id          uuid not null references public.businesses (id) on delete cascade,
  booking_id           uuid,
  channel              text not null check (channel in ('push', 'whatsapp_link', 'whatsapp_api', 'email', 'sms')),
  template             text not null,
  recipient            text,
  scheduled_for        timestamptz not null default now(),
  sent_at              timestamptz,
  status               text not null default 'queued' check (status in ('queued', 'sent', 'failed', 'skipped')),
  provider_message_id  text,
  error                text,
  foreign key (booking_id, business_id) references public.bookings (id, business_id) on delete cascade
);
create index on public.notifications (business_id, scheduled_for);

alter table public.bookings               enable row level security;
alter table public.booking_items          enable row level security;
alter table public.booking_resources      enable row level security;
alter table public.payments               enable row level security;
alter table public.rental_checkouts       enable row level security;
alter table public.booking_events         enable row level security;
alter table public.workflow_steps         enable row level security;
alter table public.booking_steps          enable row level security;
alter table public.notification_rules     enable row level security;
alter table public.notification_templates enable row level security;
alter table public.notifications          enable row level security;
