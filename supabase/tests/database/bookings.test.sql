begin;
create extension if not exists pgtap with schema extensions;
select plan(16);

-- Fixtures: negocio A (uñas) con dos empleadas y una herramienta con una unidad; negocio B aparte.
insert into public.businesses (id, slug, name, vertical) values
  ('00000000-0000-0000-0000-00000000000a', 'unas-a', 'Uñas A', 'nails'),
  ('00000000-0000-0000-0000-00000000000b', 'flores-b', 'Flores B', 'flowers');
insert into public.staff (id, business_id, name) values
  ('00000000-0000-0000-0000-0000000000d1', '00000000-0000-0000-0000-00000000000a', 'Karla'),
  ('00000000-0000-0000-0000-0000000000d2', '00000000-0000-0000-0000-00000000000a', 'Mayra');
insert into public.catalog_items (id, business_id, kind, name, price, duration_minutes) values
  ('00000000-0000-0000-0000-0000000000c1', '00000000-0000-0000-0000-00000000000a', 'service', 'Gelish', 25000, 60);
insert into public.catalog_items (id, business_id, kind, name, price, price_unit, security_deposit, late_fee_per_day) values
  ('00000000-0000-0000-0000-0000000000c2', '00000000-0000-0000-0000-00000000000a', 'rental', 'Rotomartillo', 30000, 'day', 100000, 30000);
insert into public.catalog_items (id, business_id, kind, name, price, prep_hours) values
  ('00000000-0000-0000-0000-0000000000c3', '00000000-0000-0000-0000-00000000000b', 'product', 'Ramo', 65000, 24);
insert into public.item_units (id, business_id, item_id, code) values
  ('00000000-0000-0000-0000-0000000000e1', '00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-0000000000c2', 'RM-01');
insert into public.customers (id, business_id, name, phone) values
  ('00000000-0000-0000-0000-0000000000f1', '00000000-0000-0000-0000-00000000000a', 'Ana', '+526141234567');
insert into public.bookings (id, business_id, customer_id, kind, status, starts_at, ends_at, source) values
  ('00000000-0000-0000-0000-000000000b01', '00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-0000000000f1', 'service', 'confirmed', '2026-11-02 16:00Z', '2026-11-02 17:00Z', 'admin'),
  ('00000000-0000-0000-0000-000000000b02', '00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-0000000000f1', 'service', 'confirmed', '2026-11-02 16:30Z', '2026-11-02 17:30Z', 'admin'),
  ('00000000-0000-0000-0000-000000000b03', '00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-0000000000f1', 'rental', 'confirmed', '2026-11-03 15:00Z', '2026-11-05 15:00Z', 'admin'),
  ('00000000-0000-0000-0000-000000000b04', '00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-0000000000f1', 'rental', 'pending_deposit', '2026-11-04 15:00Z', '2026-11-06 15:00Z', 'public');

-- ---- F1-2: dos booking_resources cruzados para la misma empleada o unidad fallan
insert into public.booking_resources (business_id, booking_id, staff_id, during) values
  ('00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-000000000b01', '00000000-0000-0000-0000-0000000000d1', '[2026-11-02 16:00Z, 2026-11-02 17:00Z)');

select throws_ok($$
  insert into public.booking_resources (business_id, booking_id, staff_id, during) values
    ('00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-000000000b02', '00000000-0000-0000-0000-0000000000d1', '[2026-11-02 16:30Z, 2026-11-02 17:30Z)') $$,
  '23P01', null, 'F1-2: misma empleada en horarios cruzados se rechaza');

select lives_ok($$
  insert into public.booking_resources (business_id, booking_id, staff_id, during) values
    ('00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-000000000b02', '00000000-0000-0000-0000-0000000000d2', '[2026-11-02 16:30Z, 2026-11-02 17:30Z)') $$,
  'F1-2: otra empleada en el mismo horario se acepta');

select lives_ok($$
  insert into public.booking_resources (business_id, booking_id, staff_id, during) values
    ('00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-000000000b02', '00000000-0000-0000-0000-0000000000d1', '[2026-11-02 17:00Z, 2026-11-02 18:00Z)') $$,
  'F1-2: cita contigua (empieza cuando termina la otra) se acepta');

insert into public.booking_resources (business_id, booking_id, item_unit_id, during) values
  ('00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-000000000b03', '00000000-0000-0000-0000-0000000000e1', '[2026-11-03 15:00Z, 2026-11-05 15:00Z)');

select throws_ok($$
  insert into public.booking_resources (business_id, booking_id, item_unit_id, during) values
    ('00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-000000000b04', '00000000-0000-0000-0000-0000000000e1', '[2026-11-04 15:00Z, 2026-11-06 15:00Z)') $$,
  '23P01', null, 'F1-2: misma unidad en rangos cruzados se rechaza');

-- Cancelar libera: se borran los recursos de la reserva y el rango queda disponible.
delete from public.booking_resources where booking_id = '00000000-0000-0000-0000-000000000b03';
select lives_ok($$
  insert into public.booking_resources (business_id, booking_id, item_unit_id, during) values
    ('00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-000000000b04', '00000000-0000-0000-0000-0000000000e1', '[2026-11-04 15:00Z, 2026-11-06 15:00Z)') $$,
  'unidad liberada por cancelación se puede volver a apartar');

select throws_ok($$
  insert into public.booking_resources (business_id, booking_id, staff_id, item_unit_id, during) values
    ('00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-000000000b01', '00000000-0000-0000-0000-0000000000d2',
     '00000000-0000-0000-0000-0000000000e1', '[2026-11-10 15:00Z, 2026-11-10 16:00Z)') $$,
  '23514', null, 'un recurso es empleada o unidad, no ambos');

select throws_ok($$
  insert into public.booking_resources (business_id, booking_id, staff_id, during) values
    ('00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-000000000b01', '00000000-0000-0000-0000-0000000000d2', 'empty') $$,
  '23514', null, 'rango vacío se rechaza');

-- ---- reservas
select throws_ok($$
  insert into public.bookings (business_id, customer_id, kind, status, starts_at, source) values
    ('00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-0000000000f1', 'service', 'paid', now(), 'admin') $$,
  '23514', null, 'estado fuera de la máquina se rechaza');

select throws_ok($$
  insert into public.bookings (business_id, customer_id, kind, status, starts_at, ends_at, source) values
    ('00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-0000000000f1', 'rental', 'confirmed', '2026-11-05Z', '2026-11-04Z', 'admin') $$,
  '23514', null, 'regreso antes de la salida se rechaza');

select throws_ok($$
  insert into public.bookings (business_id, customer_id, kind, status, starts_at, source) values
    ('00000000-0000-0000-0000-00000000000b', '00000000-0000-0000-0000-0000000000f1', 'product', 'confirmed', now(), 'admin') $$,
  '23503', null, 'reserva con cliente de otro negocio se rechaza');

select throws_ok($$
  insert into public.booking_items (business_id, booking_id, item_id, unit_price) values
    ('00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-000000000b01', '00000000-0000-0000-0000-0000000000c3', 65000) $$,
  '23503', null, 'línea con artículo de otro negocio se rechaza');

select matches(
  (select public_token from public.bookings where id = '00000000-0000-0000-0000-000000000b01'),
  '^[0-9a-f]{64}$', 'public_token se genera con 32 bytes aleatorios');

-- ---- pagos y pasos
select throws_ok($$
  insert into public.payments (business_id, booking_id, type, method, amount) values
    ('00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-000000000b01', 'advance', 'cash', 0) $$,
  '23514', null, 'pago en cero se rechaza');

select throws_ok($$
  insert into public.workflow_steps (business_id, applies_to_kind, actor, moment, step_type, label) values
    ('00000000-0000-0000-0000-00000000000a', 'rental', 'business', 'before_status', 'photo', 'INE') $$,
  '23514', null, 'paso before_status sin estado destino se rechaza');

-- ---- River fuera del alcance de la API pública
select ok(not has_schema_privilege('anon', 'river', 'usage'), 'anon no puede usar el schema river');
select ok(not has_schema_privilege('authenticated', 'river', 'usage'), 'authenticated no puede usar el schema river');

select * from finish();
rollback;
