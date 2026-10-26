begin;
create extension if not exists pgtap with schema extensions;
select plan(16);

-- Dos negocios para probar que no se cruzan referencias.
insert into public.businesses (id, slug, name, vertical) values
  ('00000000-0000-0000-0000-00000000000a', 'unas-a', 'Uñas A', 'nails'),
  ('00000000-0000-0000-0000-00000000000b', 'flores-b', 'Flores B', 'flowers');

-- ---- catalog_items: campos obligatorios por tipo
select lives_ok($$
  insert into public.catalog_items (business_id, kind, name, price, duration_minutes)
  values ('00000000-0000-0000-0000-00000000000a', 'service', 'Gelish', 25000, 60) $$,
  'servicio con duración se acepta');

select throws_ok($$
  insert into public.catalog_items (business_id, kind, name, price)
  values ('00000000-0000-0000-0000-00000000000a', 'service', 'Sin duración', 25000) $$,
  '23514', null, 'servicio sin duration_minutes se rechaza');

select lives_ok($$
  insert into public.catalog_items (business_id, kind, name, price, prep_hours)
  values ('00000000-0000-0000-0000-00000000000b', 'product', 'Ramo 12 rosas', 65000, 24) $$,
  'producto con prep_hours se acepta');

select throws_ok($$
  insert into public.catalog_items (business_id, kind, name, price)
  values ('00000000-0000-0000-0000-00000000000b', 'product', 'Sin anticipación', 65000) $$,
  '23514', null, 'producto sin prep_hours se rechaza');

select lives_ok($$
  insert into public.catalog_items (business_id, kind, name, price, price_unit, security_deposit, late_fee_per_day)
  values ('00000000-0000-0000-0000-00000000000a', 'rental', 'Rotomartillo', 30000, 'day', 100000, 30000) $$,
  'renta con unidad de precio, depósito y recargo se acepta');

select throws_ok($$
  insert into public.catalog_items (business_id, kind, name, price, price_unit, late_fee_per_day)
  values ('00000000-0000-0000-0000-00000000000a', 'rental', 'Sin depósito', 30000, 'day', 30000) $$,
  '23514', null, 'renta sin security_deposit se rechaza');

select throws_ok($$
  insert into public.catalog_items (business_id, kind, name, price, duration_minutes)
  values ('00000000-0000-0000-0000-00000000000a', 'service', 'Precio negativo', -1, 60) $$,
  '23514', null, 'precio negativo se rechaza');

-- ---- aislamiento entre negocios por FK compuesta
insert into public.catalog_items (id, business_id, kind, name, price, duration_minutes)
values ('00000000-0000-0000-0000-0000000000c1', '00000000-0000-0000-0000-00000000000a', 'service', 'Acrílico', 40000, 90);
insert into public.staff (id, business_id, name)
values ('00000000-0000-0000-0000-0000000000d1', '00000000-0000-0000-0000-00000000000b', 'Empleada de B');

select lives_ok($$
  insert into public.catalog_item_images (business_id, item_id, s3_key)
  values ('00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-0000000000c1', 'a/catalog/c1/1.jpg') $$,
  'imagen del mismo negocio que el artículo se acepta');

select throws_ok($$
  insert into public.catalog_item_images (business_id, item_id, s3_key)
  values ('00000000-0000-0000-0000-00000000000b', '00000000-0000-0000-0000-0000000000c1', 'b/catalog/c1/1.jpg') $$,
  '23503', null, 'imagen con business_id distinto al del artículo se rechaza');

select throws_ok($$
  insert into public.staff_services (business_id, staff_id, item_id)
  values ('00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-0000000000d1', '00000000-0000-0000-0000-0000000000c1') $$,
  '23503', null, 'asignar a un servicio una empleada de otro negocio se rechaza');

select throws_ok($$
  insert into public.business_hours (business_id, staff_id, weekday, opens_at, closes_at)
  values ('00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-0000000000d1', 1, '09:00', '18:00') $$,
  '23503', null, 'horario con empleada de otro negocio se rechaza');

-- ---- formatos
select throws_ok($$
  insert into public.businesses (slug, name, vertical) values ('Mi Negocio', 'X', 'nails') $$,
  '23514', null, 'slug con mayúsculas o espacios se rechaza');

select throws_ok($$
  insert into public.businesses (slug, name, vertical) values ('limos-z', 'X', 'limos') $$,
  '23514', null, 'giro desconocido se rechaza');

select throws_ok($$
  insert into public.customers (business_id, name, phone)
  values ('00000000-0000-0000-0000-00000000000a', 'Ana', '614 123 4567') $$,
  '23514', null, 'teléfono fuera de E.164 se rechaza');

select lives_ok($$
  insert into public.customers (business_id, name, phone)
  values ('00000000-0000-0000-0000-00000000000a', 'Ana', '+526141234567') $$,
  'teléfono E.164 se acepta');

select throws_ok($$
  insert into public.business_hours (business_id, weekday, opens_at, closes_at)
  values ('00000000-0000-0000-0000-00000000000a', 1, '18:00', '09:00') $$,
  '23514', null, 'horario que cierra antes de abrir se rechaza');

select * from finish();
rollback;
