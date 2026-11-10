-- Descuento de una venta rápida (tanda B1): lo que se quitó del subtotal; total ya es lo cobrado.
alter table public.sales add column discount bigint not null default 0 check (discount >= 0);
