-- Negocios demo, uno por giro. IDs fijos + on conflict do nothing: se puede aplicar varias veces.
-- Sin usuarios de auth: las cuentas de dueño se crean con el onboarding (fase 2).
-- Montos en centavos MXN.

-- ---------------------------------------------------------------- uñas

insert into public.businesses (id, slug, name, vertical, phone_whatsapp, settings) values
  ('d0000000-0000-4000-8000-000000000001', 'demo-unas', 'Uñas Bellas Chihuahua', 'nails', '+526141000001',
   '{"deposit_percent": 30, "cancel_hours": 24, "hold_hours": 4}')
on conflict do nothing;

insert into public.staff (id, business_id, name, color) values
  ('d0000000-0000-4000-8001-000000000001', 'd0000000-0000-4000-8000-000000000001', 'Karla', '#E91E63'),
  ('d0000000-0000-4000-8001-000000000002', 'd0000000-0000-4000-8000-000000000001', 'Mayra', '#9C27B0')
on conflict do nothing;

-- Lunes a sábado 10:00–19:00 (general) y cada empleada con su turno.
insert into public.business_hours (business_id, staff_id, weekday, opens_at, closes_at)
select 'd0000000-0000-4000-8000-000000000001'::uuid, s.staff_id, d, s.opens, s.closes
from generate_series(1, 6) d
cross join (values
  (null::uuid, '10:00'::time, '19:00'::time),
  ('d0000000-0000-4000-8001-000000000001', '10:00', '16:00'),
  ('d0000000-0000-4000-8001-000000000002', '13:00', '19:00')
) as s(staff_id, opens, closes)
where not exists (select 1 from public.business_hours where business_id = 'd0000000-0000-4000-8000-000000000001');

insert into public.catalog_items (id, business_id, kind, name, description, price, sort, duration_minutes, buffer_minutes) values
  ('d0000000-0000-4000-8002-000000000001', 'd0000000-0000-4000-8000-000000000001', 'service', 'Gelish en manos', 'Esmaltado semipermanente con limado y cutícula.', 25000, 1, 60, 10),
  ('d0000000-0000-4000-8002-000000000002', 'd0000000-0000-4000-8000-000000000001', 'service', 'Uñas acrílicas', 'Aplicación completa con forma a elegir.', 45000, 2, 120, 15),
  ('d0000000-0000-4000-8002-000000000003', 'd0000000-0000-4000-8000-000000000001', 'service', 'Pedicure spa', 'Exfoliación, hidratación y esmaltado.', 35000, 3, 75, 10)
on conflict do nothing;

insert into public.item_options (id, business_id, item_id, name, type, choices, extra_price, required) values
  ('d0000000-0000-4000-8003-000000000001', 'd0000000-0000-4000-8000-000000000001', 'd0000000-0000-4000-8002-000000000002',
   'Diseño', 'choice', '["Liso", "Francés", "Arte a mano"]', 5000, false)
on conflict do nothing;

insert into public.staff_services (business_id, staff_id, item_id) values
  ('d0000000-0000-4000-8000-000000000001', 'd0000000-0000-4000-8001-000000000001', 'd0000000-0000-4000-8002-000000000001'),
  ('d0000000-0000-4000-8000-000000000001', 'd0000000-0000-4000-8001-000000000001', 'd0000000-0000-4000-8002-000000000002'),
  ('d0000000-0000-4000-8000-000000000001', 'd0000000-0000-4000-8001-000000000002', 'd0000000-0000-4000-8002-000000000001'),
  ('d0000000-0000-4000-8000-000000000001', 'd0000000-0000-4000-8001-000000000002', 'd0000000-0000-4000-8002-000000000003')
on conflict do nothing;

-- ---------------------------------------------------------------- florería

insert into public.businesses (id, slug, name, vertical, phone_whatsapp, settings) values
  ('d0000000-0000-4000-8000-000000000002', 'demo-flores', 'Florería Las Margaritas', 'flowers', '+526141000002',
   '{"deposit_percent": 50, "cancel_hours": 48, "hold_hours": 6, "daily_capacity": 20,
     "delivery_zones": [{"name": "Centro", "fee": 6000}, {"name": "Norte", "fee": 9000}, {"name": "Sur", "fee": 9000}]}')
on conflict do nothing;

insert into public.business_hours (business_id, weekday, opens_at, closes_at)
select 'd0000000-0000-4000-8000-000000000002'::uuid, d, '09:00', '20:00'
from generate_series(0, 6) d
where not exists (select 1 from public.business_hours where business_id = 'd0000000-0000-4000-8000-000000000002');

insert into public.catalog_items (id, business_id, kind, name, description, price, sort, prep_hours, daily_capacity) values
  ('d0000000-0000-4000-8002-000000000011', 'd0000000-0000-4000-8000-000000000002', 'product', 'Ramo de 12 rosas', 'Rosas rojas con follaje y papel coreano.', 65000, 1, 24, 10),
  ('d0000000-0000-4000-8002-000000000012', 'd0000000-0000-4000-8000-000000000002', 'product', 'Caja de girasoles', 'Seis girasoles en caja redonda.', 55000, 2, 24, null),
  ('d0000000-0000-4000-8002-000000000013', 'd0000000-0000-4000-8000-000000000002', 'product', 'Arma tu ramo', 'Elige base, flores y follaje.', 15000, 3, 24, null)
on conflict do nothing;

insert into public.item_options (id, business_id, item_id, name, type, required) values
  ('d0000000-0000-4000-8003-000000000011', 'd0000000-0000-4000-8000-000000000002', 'd0000000-0000-4000-8002-000000000011', 'Mensaje de tarjeta', 'text', false),
  ('d0000000-0000-4000-8003-000000000012', 'd0000000-0000-4000-8000-000000000002', 'd0000000-0000-4000-8002-000000000013', 'Mensaje de tarjeta', 'text', false)
on conflict do nothing;

insert into public.item_component_groups (id, business_id, item_id, name, min_total, max_total, required, sort) values
  ('d0000000-0000-4000-8004-000000000001', 'd0000000-0000-4000-8000-000000000002', 'd0000000-0000-4000-8002-000000000013', 'Base', 1, 1, true, 1),
  ('d0000000-0000-4000-8004-000000000002', 'd0000000-0000-4000-8000-000000000002', 'd0000000-0000-4000-8002-000000000013', 'Flores', 12, 36, true, 2),
  ('d0000000-0000-4000-8004-000000000003', 'd0000000-0000-4000-8000-000000000002', 'd0000000-0000-4000-8002-000000000013', 'Follaje', 0, 10, false, 3)
on conflict do nothing;

insert into public.item_components (id, business_id, group_id, name, unit_price, max_qty) values
  ('d0000000-0000-4000-8005-000000000001', 'd0000000-0000-4000-8000-000000000002', 'd0000000-0000-4000-8004-000000000001', 'Papel coreano', 0, 1),
  ('d0000000-0000-4000-8005-000000000002', 'd0000000-0000-4000-8000-000000000002', 'd0000000-0000-4000-8004-000000000001', 'Caja redonda', 12000, 1),
  ('d0000000-0000-4000-8005-000000000003', 'd0000000-0000-4000-8000-000000000002', 'd0000000-0000-4000-8004-000000000001', 'Florero de vidrio', 18000, 1),
  ('d0000000-0000-4000-8005-000000000004', 'd0000000-0000-4000-8000-000000000002', 'd0000000-0000-4000-8004-000000000002', 'Rosa roja', 4500, 36),
  ('d0000000-0000-4000-8005-000000000005', 'd0000000-0000-4000-8000-000000000002', 'd0000000-0000-4000-8004-000000000002', 'Girasol', 5000, 12),
  ('d0000000-0000-4000-8005-000000000006', 'd0000000-0000-4000-8000-000000000002', 'd0000000-0000-4000-8004-000000000002', 'Lilis', 6000, 12),
  ('d0000000-0000-4000-8005-000000000007', 'd0000000-0000-4000-8000-000000000002', 'd0000000-0000-4000-8004-000000000003', 'Eucalipto', 1500, 10)
on conflict do nothing;

-- ---------------------------------------------------------------- renta de herramienta

insert into public.businesses (id, slug, name, vertical, phone_whatsapp, settings) values
  ('d0000000-0000-4000-8000-000000000003', 'demo-herramienta', 'Rentas El Taller', 'tool_rental', '+526141000003',
   '{"deposit_percent": 0, "cancel_hours": 24, "hold_hours": 24, "inspection_minutes": 60}')
on conflict do nothing;

insert into public.business_hours (business_id, weekday, opens_at, closes_at)
select 'd0000000-0000-4000-8000-000000000003'::uuid, d, '08:00', '18:00'
from generate_series(1, 6) d
where not exists (select 1 from public.business_hours where business_id = 'd0000000-0000-4000-8000-000000000003');

insert into public.catalog_items (id, business_id, kind, name, description, price, sort, price_unit, security_deposit, late_fee_per_day) values
  ('d0000000-0000-4000-8002-000000000021', 'd0000000-0000-4000-8000-000000000003', 'rental', 'Rotomartillo', 'Rotomartillo SDS plus con juego de brocas.', 30000, 1, 'day', 100000, 30000),
  ('d0000000-0000-4000-8002-000000000022', 'd0000000-0000-4000-8000-000000000003', 'rental', 'Revolvedora de concreto', 'Revolvedora de un saco, motor a gasolina.', 60000, 2, 'day', 300000, 60000),
  ('d0000000-0000-4000-8002-000000000023', 'd0000000-0000-4000-8000-000000000003', 'rental', 'Andamio (sección)', 'Sección de 1.5 m con crucetas.', 15000, 3, 'day', 50000, 15000)
on conflict do nothing;

insert into public.item_units (id, business_id, item_id, code) values
  ('d0000000-0000-4000-8006-000000000001', 'd0000000-0000-4000-8000-000000000003', 'd0000000-0000-4000-8002-000000000021', 'RM-01'),
  ('d0000000-0000-4000-8006-000000000002', 'd0000000-0000-4000-8000-000000000003', 'd0000000-0000-4000-8002-000000000021', 'RM-02'),
  ('d0000000-0000-4000-8006-000000000003', 'd0000000-0000-4000-8000-000000000003', 'd0000000-0000-4000-8002-000000000022', 'RV-01'),
  ('d0000000-0000-4000-8006-000000000004', 'd0000000-0000-4000-8000-000000000003', 'd0000000-0000-4000-8002-000000000023', 'AN-01'),
  ('d0000000-0000-4000-8006-000000000005', 'd0000000-0000-4000-8000-000000000003', 'd0000000-0000-4000-8002-000000000023', 'AN-02'),
  ('d0000000-0000-4000-8006-000000000006', 'd0000000-0000-4000-8000-000000000003', 'd0000000-0000-4000-8002-000000000023', 'AN-03'),
  ('d0000000-0000-4000-8006-000000000007', 'd0000000-0000-4000-8000-000000000003', 'd0000000-0000-4000-8002-000000000023', 'AN-04')
on conflict do nothing;

-- ---------------------------------------------------------------- clientes de ejemplo (teléfonos ficticios)

insert into public.customers (id, business_id, name, phone) values
  ('d0000000-0000-4000-8007-000000000001', 'd0000000-0000-4000-8000-000000000001', 'Ana López', '+526140000101'),
  ('d0000000-0000-4000-8007-000000000002', 'd0000000-0000-4000-8000-000000000002', 'Luis Hernández', '+526140000102'),
  ('d0000000-0000-4000-8007-000000000003', 'd0000000-0000-4000-8000-000000000003', 'Construcciones Robles', '+526140000103')
on conflict do nothing;
