-- Borrar un artículo o una unidad: desaparece del catálogo, el inventario y la disponibilidad, pero
-- las reservas pasadas conservan su nombre y su clave (borrar la fila rompería ese historial).
alter table public.catalog_items add column deleted_at timestamptz;
alter table public.item_units add column deleted_at timestamptz;

-- La clave de una unidad borrada se puede volver a usar.
alter table public.item_units drop constraint item_units_item_id_code_key;
create unique index item_units_item_code on public.item_units (item_id, code) where deleted_at is null;
