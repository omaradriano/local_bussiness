-- Idempotencia del alta de reservas: el cliente manda Idempotency-Key; un reintento (doble toque, red
-- inestable) devuelve la misma reserva en lugar de crear otra.
alter table public.bookings add column idempotency_key text check (length(idempotency_key) between 8 and 100);
create unique index bookings_idempotency_key on public.bookings (business_id, idempotency_key) where idempotency_key is not null;

-- Quién la capturó (alta manual); null en reservas hechas por el cliente final.
alter table public.bookings add column created_by uuid references auth.users (id) on delete set null;
