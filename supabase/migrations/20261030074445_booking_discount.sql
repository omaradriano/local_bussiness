-- Descuento por reserva: una cantidad fija (centavos) o un porcentaje del total. Se guardan el tipo
-- y el valor para recalcularlo (al extender una renta) y el monto que quitó del total.
alter table public.bookings
  add column discount_type  text check (discount_type in ('amount', 'percent')),
  add column discount_value bigint not null default 0 check (discount_value >= 0),
  add column discount       bigint not null default 0 check (discount >= 0),
  add constraint bookings_discount_percent check (discount_type <> 'percent' or discount_value <= 100);
