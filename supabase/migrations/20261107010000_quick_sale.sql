-- Venta rápida (función quickSale): una venta ya no es solo de productos con existencias; cada
-- línea es un producto (product_id), un artículo del catálogo (item_id) o un importe libre (solo
-- name). name congela lo que se vendió. La venta y el pedido de un cliente casual pueden llevar
-- un nombre y un teléfono solo para esa vez, sin crear ficha de cliente.

alter table public.sale_lines
  alter column product_id drop not null,
  add column item_id uuid,
  add column name    text check (length(name) between 1 and 200),
  add foreign key (item_id, business_id) references public.catalog_items (id, business_id),
  add check (num_nonnulls(product_id, item_id) <= 1 and (product_id is not null or name is not null));
create index on public.sale_lines (item_id);

alter table public.sales
  add column customer_name  text check (length(customer_name) <= 80),
  add column customer_phone text;

alter table public.bookings
  add column walk_in_name  text check (length(walk_in_name) <= 80),
  add column walk_in_phone text;
