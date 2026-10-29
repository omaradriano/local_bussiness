-- Finanzas del negocio: gastos, compras a plazos y la paga del personal. Solo el dueño las ve
-- (la API lo exige); RLS de solo lectura para los dueños.

create table public.expenses (
  id           uuid primary key default gen_random_uuid(),
  business_id  uuid not null references public.businesses (id) on delete cascade,
  spent_on     date not null,
  category     text not null check (category in ('transport', 'maintenance', 'rent', 'utilities', 'supplies', 'asset', 'advertising', 'taxes', 'payroll', 'other')),
  amount       bigint not null check (amount > 0),
  description  text check (description is null or length(description) <= 200),
  created_by   uuid references auth.users (id) on delete set null,
  created_at   timestamptz not null default now()
);
create index on public.expenses (business_id, spent_on);

-- Compra a plazos (p. ej. una camioneta): enganche el día de la compra y una mensualidad por mes
-- desde first_due. Cada mensualidad cuenta como gasto en su fecha; no se guardan filas por pago.
create table public.expense_plans (
  id                  uuid primary key default gen_random_uuid(),
  business_id         uuid not null references public.businesses (id) on delete cascade,
  description         text not null check (length(trim(description)) between 1 and 120),
  category            text not null check (category in ('transport', 'maintenance', 'rent', 'utilities', 'supplies', 'asset', 'advertising', 'taxes', 'payroll', 'other')),
  purchased_on        date not null,
  down_payment        bigint not null default 0 check (down_payment >= 0),
  installment_amount  bigint not null check (installment_amount > 0),
  installments        int not null check (installments between 1 and 360),
  first_due           date not null check (first_due >= purchased_on),
  created_by          uuid references auth.users (id) on delete set null,
  created_at          timestamptz not null default now()
);
create index on public.expense_plans (business_id);

-- Paga de cada persona del equipo: sueldo fijo por periodo y/o comisión sobre lo cobrado de sus servicios.
create table public.staff_pay (
  staff_id            uuid primary key references public.staff (id) on delete cascade,
  business_id         uuid not null references public.businesses (id) on delete cascade,
  salary              bigint not null default 0 check (salary >= 0),
  salary_period       text not null default 'monthly' check (salary_period in ('weekly', 'biweekly', 'monthly')),
  commission_percent  numeric(5, 2) not null default 0 check (commission_percent between 0 and 100),
  updated_at          timestamptz not null default now()
);

alter table public.expenses enable row level security;
alter table public.expense_plans enable row level security;
alter table public.staff_pay enable row level security;
create policy owners_read on public.expenses for select to authenticated using (private.is_member(business_id, array['owner']));
create policy owners_read on public.expense_plans for select to authenticated using (private.is_member(business_id, array['owner']));
create policy owners_read on public.staff_pay for select to authenticated using (private.is_member(business_id, array['owner']));
