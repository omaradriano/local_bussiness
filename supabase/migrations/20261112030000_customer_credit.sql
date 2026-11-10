-- Crédito (tanda B3): la dueña permite entregar a este cliente con saldo pendiente y cobrarle después.
alter table public.customers add column credit boolean not null default false;
