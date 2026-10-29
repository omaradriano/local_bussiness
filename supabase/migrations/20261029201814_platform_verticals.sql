-- Quién puede abrir un negocio nuevo de cada giro: public (cualquiera al registrarse), admin (solo el
-- superadministrador, para probarlo incluso en producción) u off (nadie). Los negocios que ya existen
-- siguen funcionando en cualquier estado. Un giro sin fila se trata como admin.
create table public.platform_verticals (
  vertical    text primary key,
  status      text not null check (status in ('public', 'admin', 'off')),
  updated_at  timestamptz not null default now()
);
alter table public.platform_verticals enable row level security; -- solo la API (rol privilegiado)

insert into public.platform_verticals (vertical, status) values
  ('tool_rental', 'public'), ('barbershop', 'public'), ('nails', 'admin'), ('flowers', 'admin');
