-- Requerimientos de la barbería.

-- «Cliente casual»: un cliente interno por negocio para los servicios que no se registran.
-- No aparece en búsquedas ni en la lista de clientes.
alter table public.customers add column walk_in boolean not null default false;
create unique index customers_one_walk_in on public.customers (business_id) where walk_in;

-- El margen entre citas pasa del servicio al negocio (settings.buffer_minutes): se toma el mayor
-- de sus servicios para no acortar ninguno.
update public.businesses b
set settings = b.settings || jsonb_build_object('buffer_minutes', m.buffer)
from (
  select business_id, max(buffer_minutes) as buffer
  from public.catalog_items where kind = 'service' group by business_id
) m
where m.business_id = b.id and not b.settings ? 'buffer_minutes';

-- Barberías ya creadas: anticipo solo en reservas por internet (como su plantilla).
update public.businesses
set settings = settings || '{"advance_only_online": true}'::jsonb
where vertical = 'barbershop' and not settings ? 'advance_only_online';
