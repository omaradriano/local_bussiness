-- Stock mínimo por producto y archivo (tanda C3): el aviso de «pocas existencias» usa min_stock;
-- un producto con historial borrado se archiva para conservar sus ventas y compras.
alter table public.retail_products
  add column min_stock int not null default 3 check (min_stock between 0 and 100000),
  add column deleted_at timestamptz;
