-- Florería: quién recibe el pedido y la dedicatoria (función giftOrder), y artículos con precio
-- libre que se capturan en cada pedido, como el «arreglo personalizado» (función customOrder).
-- La descripción del pedido personalizado queda congelada en booking_items.options.description.

alter table public.bookings
  add column recipient_name  text check (length(recipient_name) <= 80),
  add column recipient_phone text,
  add column card_message    text check (length(card_message) <= 500);

alter table public.catalog_items
  add column custom_price boolean not null default false;
