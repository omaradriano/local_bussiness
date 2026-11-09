-- Extras of an order can stay on its account (charged with the order, at delivery): no money yet.
alter table public.sales drop constraint sales_method_check;
alter table public.sales add constraint sales_method_check
  check (method in ('cash', 'transfer', 'card', 'on_account') and (method <> 'on_account' or booking_id is not null));
