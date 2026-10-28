-- Clientes guardados para rentas: dirección, aval, "de confianza" y documentos en su ficha
-- (INE, comprobante de domicilio, identificación del aval). Los documentos se reutilizan en
-- cada renta en lugar de pedirse otra vez.

alter table public.customers
  add column address text check (address is null or length(address) <= 300),
  add column guarantor_name text check (guarantor_name is null or length(trim(guarantor_name)) between 1 and 80),
  add column guarantor_phone text check (guarantor_phone ~ '^\+[1-9][0-9]{7,14}$'),
  add column trusted boolean not null default false, -- el dueño no exige documentos a este cliente
  add column updated_at timestamptz not null default now();

create table public.customer_documents (
  id           uuid primary key default gen_random_uuid(),
  business_id  uuid not null,
  customer_id  uuid not null,
  doc_type     text not null check (doc_type in ('ine', 'proof_of_address', 'guarantor_id')),
  s3_key       text not null unique, -- bucket private-docs
  uploaded_by  uuid references auth.users (id) on delete set null,
  created_at   timestamptz not null default now(),
  foreign key (customer_id, business_id) references public.customers (id, business_id) on delete cascade
);
create index on public.customer_documents (customer_id, doc_type, created_at desc);

alter table public.customer_documents enable row level security;
create policy members_read on public.customer_documents for select to authenticated using (private.is_member(business_id));

-- Los pasos de documentos se cumplen con la ficha del cliente: se marcan con config.customer_doc.
update public.workflow_steps
set config = config || '{"customer_doc": "ine"}'::jsonb
where step_type = 'photo' and label = 'Foto del INE' and not config ? 'customer_doc';

insert into public.workflow_steps (business_id, applies_to_kind, actor, moment, to_status, step_type, label, config, required, sort)
select ws.business_id, 'rental', 'business', 'before_status', 'checked_out', 'photo', 'Comprobante de domicilio',
       '{"bucket": "private-docs", "max_files": 2, "customer_doc": "proof_of_address"}'::jsonb, true, ws.sort
from public.workflow_steps ws
where ws.label = 'Foto del INE' and ws.applies_to_kind = 'rental'
  and not exists (select 1 from public.workflow_steps x where x.business_id = ws.business_id and x.config->>'customer_doc' = 'proof_of_address');

-- Reservas en curso: su copia del paso de INE también se toma de la ficha.
update public.booking_steps
set step_snapshot = jsonb_set(step_snapshot, '{config,customer_doc}', '"ine"')
where status = 'pending' and step_snapshot->>'label' = 'Foto del INE';
