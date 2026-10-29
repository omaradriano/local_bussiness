-- Gastos recurrentes (renta del local, internet, luz): se capturan una vez con un monto aproximado
-- y una frecuencia. Cada cobro queda pendiente hasta que el dueño lo confirma con el monto real
-- (se vuelve un gasto normal, ligado por recurring_id + due_on) o indica que no se pagó.
create table public.recurring_expenses (
  id           uuid primary key default gen_random_uuid(),
  business_id  uuid not null references public.businesses (id) on delete cascade,
  description  text not null check (length(trim(description)) between 1 and 120),
  category     text not null check (category in ('transport', 'maintenance', 'rent', 'utilities', 'supplies', 'asset', 'advertising', 'taxes', 'payroll', 'other')),
  amount       bigint not null check (amount > 0), -- aproximado: se propone al confirmar cada cobro
  frequency    text not null check (frequency in ('weekly', 'biweekly', 'monthly', 'yearly')),
  starts_on    date not null,
  ends_on      date check (ends_on is null or ends_on >= starts_on),
  created_by   uuid references auth.users (id) on delete set null,
  created_at   timestamptz not null default now(),
  unique (id, business_id)
);
create index on public.recurring_expenses (business_id);

-- Un cobro confirmado es un gasto con su fecha de cobro; borrar el gasto lo deja pendiente otra vez.
-- Si se borra el recurrente, sus gastos se conservan.
alter table public.expenses
  add column recurring_id uuid references public.recurring_expenses (id) on delete set null,
  add column due_on date,
  add constraint expenses_recurring_due check (recurring_id is null or due_on is not null);
create unique index expenses_recurring_charge on public.expenses (recurring_id, due_on) where recurring_id is not null;

-- Cobros que el dueño marcó como «No se pagó».
create table public.recurring_skips (
  business_id   uuid not null,
  recurring_id  uuid not null,
  due_on        date not null,
  primary key (recurring_id, due_on),
  foreign key (recurring_id, business_id) references public.recurring_expenses (id, business_id) on delete cascade
);

alter table public.recurring_expenses enable row level security;
alter table public.recurring_skips enable row level security;
create policy owners_read on public.recurring_expenses for select to authenticated using (private.is_member(business_id, array['owner']));
create policy owners_read on public.recurring_skips for select to authenticated using (private.is_member(business_id, array['owner']));
