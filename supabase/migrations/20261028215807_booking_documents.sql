-- Documentos adicionales de una renta.
-- 1) Los tipos que el negocio configura en Ajustes son pasos (config.section = 'additional'), de
--    archivo (foto o PDF). El pagaré queda como uno más, apagado hasta que el dueño lo active.
update public.workflow_steps
set step_type = 'file_upload', active = false
where config->>'section' = 'additional' and label = 'Pagaré';

-- Las rentas que todavía no salen ya no lo piden (se agregó hoy mismo y nadie lo activó).
delete from public.booking_steps bs
using public.bookings b
where b.id = bs.booking_id and bs.status = 'pending'
  and bs.step_snapshot->'config'->>'section' = 'additional' and bs.step_snapshot->>'label' = 'Pagaré'
  and b.status in ('pending_deposit', 'confirmed');

-- 2) Además, cualquier archivo con el nombre que le den, aunque no haya un tipo configurado.
create table public.booking_documents (
  id           uuid primary key default gen_random_uuid(),
  business_id  uuid not null,
  booking_id   uuid not null,
  name         text not null check (length(trim(name)) between 1 and 60),
  s3_keys      text[] not null check (cardinality(s3_keys) between 1 and 6), -- bucket private-docs
  uploaded_by  uuid references auth.users (id) on delete set null,
  created_at   timestamptz not null default now(),
  foreign key (booking_id, business_id) references public.bookings (id, business_id) on delete cascade
);
create index on public.booking_documents (booking_id, created_at);

alter table public.booking_documents enable row level security;
create policy members_read on public.booking_documents for select to authenticated using (private.is_member(business_id));
