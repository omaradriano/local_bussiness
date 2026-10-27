begin;
create extension if not exists pgtap with schema extensions;
select plan(8);

select results_eq(
  $$select slug, vertical from public.businesses where slug like 'demo-%' order by slug$$,
  $$values ('demo-flores', 'flowers'), ('demo-herramienta', 'tool_rental'), ('demo-unas', 'nails')$$,
  'seed: un negocio demo por giro');

select is_empty($$
  select b.slug from public.businesses b
  where b.slug like 'demo-%'
    and not exists (select 1 from public.catalog_items i where i.business_id = b.id and i.active) $$,
  'seed: cada negocio demo tiene catálogo activo');

select is_empty($$
  select b.slug from public.businesses b
  where b.slug like 'demo-%'
    and not exists (select 1 from public.business_hours h where h.business_id = b.id) $$,
  'seed: cada negocio demo tiene horario');

select is_empty($$
  select i.name from public.catalog_items i join public.businesses b on b.id = i.business_id
  where b.slug = 'demo-unas' and i.kind = 'service'
    and not exists (select 1 from public.staff_services s where s.item_id = i.id) $$,
  'seed: cada servicio de uñas lo hace al menos una empleada');

select is(
  (select count(*)::int from public.item_units u join public.businesses b on b.id = u.business_id where b.slug = 'demo-herramienta'),
  7, 'seed: renta de herramienta con 7 unidades');

select is_empty($$
  select i.name from public.catalog_items i join public.businesses b on b.id = i.business_id
  where b.slug = 'demo-herramienta'
    and not exists (select 1 from public.item_units u where u.item_id = i.id and u.status = 'available') $$,
  'seed: cada herramienta tiene unidades disponibles');

select results_eq(
  $$select g.name, g.min_total, g.max_total from public.item_component_groups g
    join public.catalog_items i on i.id = g.item_id where i.name = 'Arma tu ramo' order by g.sort$$,
  $$values ('Base'::text, 1, 1), ('Flores', 12, 36), ('Follaje', 0, 10)$$,
  'seed: armador de ramo con base, flores (12 a 36) y follaje');

select ok(
  (select settings ? 'deposit_percent' from public.businesses where slug = 'demo-flores'),
  'seed: políticas del negocio en settings');

select * from finish();
rollback;
