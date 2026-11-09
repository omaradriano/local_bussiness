-- Zona de entrega del pedido (antes solo fijaba el costo): la hoja de ruta agrupa por ella.
-- Los pedidos anteriores quedan sin zona.
alter table public.bookings
  add column delivery_zone text check (length(delivery_zone) <= 80);
