begin;
create extension if not exists pgtap with schema extensions;
select plan(18);

-- Usuarios: dueño de A, empleada de A, dueño de B y un usuario sin negocio.
insert into auth.users (id, email, aud, role) values
  ('00000000-0000-0000-0000-0000000000a1', 'owner-a@test.mx', 'authenticated', 'authenticated'),
  ('00000000-0000-0000-0000-0000000000a2', 'staff-a@test.mx', 'authenticated', 'authenticated'),
  ('00000000-0000-0000-0000-0000000000b1', 'owner-b@test.mx', 'authenticated', 'authenticated'),
  ('00000000-0000-0000-0000-0000000000c1', 'nadie@test.mx',   'authenticated', 'authenticated');

insert into public.businesses (id, slug, name, vertical) values
  ('00000000-0000-0000-0000-00000000000a', 'unas-a', 'Uñas A', 'nails'),
  ('00000000-0000-0000-0000-00000000000b', 'flores-b', 'Flores B', 'flowers');
insert into public.staff (id, business_id, name) values
  ('00000000-0000-0000-0000-0000000000d1', '00000000-0000-0000-0000-00000000000a', 'Karla');
insert into public.business_members (business_id, user_id, role, staff_id) values
  ('00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-0000000000a1', 'owner', null),
  ('00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-0000000000a2', 'staff', '00000000-0000-0000-0000-0000000000d1'),
  ('00000000-0000-0000-0000-00000000000b', '00000000-0000-0000-0000-0000000000b1', 'owner', null);

insert into public.catalog_items (business_id, kind, name, price, duration_minutes) values
  ('00000000-0000-0000-0000-00000000000a', 'service', 'Gelish', 25000, 60);
insert into public.catalog_items (business_id, kind, name, price, prep_hours) values
  ('00000000-0000-0000-0000-00000000000b', 'product', 'Ramo', 65000, 24);
insert into public.customers (id, business_id, name, phone) values
  ('00000000-0000-0000-0000-0000000000f1', '00000000-0000-0000-0000-00000000000a', 'Ana', '+526141234567'),
  ('00000000-0000-0000-0000-0000000000f2', '00000000-0000-0000-0000-00000000000b', 'Luis', '+526149876543');
insert into public.bookings (id, business_id, customer_id, kind, status, starts_at, source) values
  ('00000000-0000-0000-0000-000000000b01', '00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-0000000000f1', 'service', 'confirmed', '2026-11-02 16:00Z', 'admin'),
  ('00000000-0000-0000-0000-000000000b02', '00000000-0000-0000-0000-00000000000b', '00000000-0000-0000-0000-0000000000f2', 'product', 'confirmed', '2026-11-02 16:00Z', 'admin');
insert into public.notification_rules (business_id, event, audience, channels) values
  ('00000000-0000-0000-0000-00000000000a', 'reminder', 'owner', '{push}');
insert into public.notification_templates (vertical, event, channel, body) values
  ('nails', 'reminder', 'whatsapp_link', 'Hola {{cliente}}, te esperamos el {{fecha}}');

-- Fija el JWT del usuario; el cambio de rol va explícito después de cada login.
create function pg_temp.login(uid uuid) returns void language sql as $$
  select set_config('request.jwt.claims', json_build_object('sub', uid, 'role', 'authenticated')::text, true);
$$;

-- ---- dueño de A
select pg_temp.login('00000000-0000-0000-0000-0000000000a1');
set local role authenticated;
select results_eq('select slug from public.businesses', $$values ('unas-a')$$, 'F1-1: dueño de A solo ve su negocio');
select results_eq('select name from public.catalog_items', $$values ('Gelish')$$, 'F1-1: dueño de A solo ve su catálogo');
select is_empty($$select 1 from public.customers where business_id = '00000000-0000-0000-0000-00000000000b'$$, 'F1-1: dueño de A no ve clientes de B');
select is_empty($$select 1 from public.bookings where business_id = '00000000-0000-0000-0000-00000000000b'$$, 'F1-1: dueño de A no ve reservas de B');
select is((select count(*)::int from public.business_members), 2, 'dueño de A ve a todo su equipo');
select is((select count(*)::int from public.notification_rules), 1, 'dueño de A ve sus avisos');
select throws_ok($$insert into public.catalog_items (business_id, kind, name, price, duration_minutes)
  values ('00000000-0000-0000-0000-00000000000a', 'service', 'Directo', 1, 30)$$,
  '42501', null, 'F1-1: ni el dueño escribe directo; las escrituras van por la API');
select throws_ok($$update public.bookings set status = 'completed'$$,
  '42501', null, 'F1-1: el dueño no cambia estados directo');
select throws_ok($$insert into public.catalog_items (business_id, kind, name, price, prep_hours)
  values ('00000000-0000-0000-0000-00000000000b', 'product', 'Intruso', 1, 1)$$,
  '42501', null, 'F1-1: dueño de A no puede escribir en B');
reset role;

-- ---- empleada de A
select pg_temp.login('00000000-0000-0000-0000-0000000000a2');
set local role authenticated;
select is((select count(*)::int from public.bookings), 1, 'empleada de A ve las reservas de su negocio');
select is((select count(*)::int from public.business_members), 1, 'empleada solo ve su propia membresía');
select is_empty('select 1 from public.notification_rules', 'empleada no ve la configuración de avisos');
select is((select count(*)::int from public.notification_templates), 1, 'plantillas por defecto del giro son visibles');
select throws_ok($$delete from public.business_members$$, '42501', null, 'empleada no puede borrar membresías');
reset role;

-- ---- dueño de B y usuario sin negocio
select pg_temp.login('00000000-0000-0000-0000-0000000000b1');
set local role authenticated;
select results_eq('select status from public.bookings', $$values ('confirmed')$$, 'F1-1: dueño de B solo ve su reserva');
reset role;

select pg_temp.login('00000000-0000-0000-0000-0000000000c1');
set local role authenticated;
select is_empty('select 1 from public.businesses', 'usuario sin negocio no ve ninguno');
reset role;

-- ---- anon y salvaguardas
set local role anon;
select throws_ok('select 1 from public.businesses', '42501', null, 'F1-1: anon no lee tablas; el catálogo público va por la API');
reset role;

select is_empty($$
  select c.relname from pg_class c join pg_namespace n on n.oid = c.relnamespace
  where n.nspname = 'public' and c.relkind = 'r' and not c.relrowsecurity $$,
  'toda tabla de public tiene RLS activo');

select * from finish();
rollback;
