-- Fase 1 · parte 3: RLS.
-- El panel solo LEE directo de Supabase; toda escritura pasa por la API Go con la service role
-- (que no está sujeta a RLS). Por eso aquí solo hay políticas de select y se quitan los permisos
-- de escritura a anon y authenticated: aunque alguien olvide una política, no se puede escribir.
-- anon no lee nada: el catálogo público lo sirve la API.

-- ---------------------------------------------------------------- helper

-- Schema no expuesto por la API REST (solo public y graphql_public lo están).
create schema if not exists private;
grant usage on schema private to authenticated;

-- security definer: lee business_members sin pasar por su propia RLS (evita recursión).
create function private.is_member(target_business uuid, roles text[] default null)
returns boolean
language sql
stable
security definer
set search_path = ''
as $$
  select exists (
    select 1
    from public.business_members m
    where m.business_id = target_business
      and m.user_id = (select auth.uid())
      and (roles is null or m.role = any (roles))
  );
$$;
revoke all on function private.is_member(uuid, text[]) from public;
grant execute on function private.is_member(uuid, text[]) to authenticated;

-- ---------------------------------------------------------------- permisos de tabla

revoke all on all tables in schema public from anon;
revoke insert, update, delete, truncate on all tables in schema public from authenticated;
-- Lo mismo para tablas futuras creadas por migraciones (rol postgres).
alter default privileges for role postgres in schema public revoke all on tables from anon;
alter default privileges for role postgres in schema public revoke insert, update, delete, truncate on tables from authenticated;

-- ---------------------------------------------------------------- políticas de lectura

-- Cualquier miembro (owner o staff) del negocio.
create policy members_read on public.businesses            for select to authenticated using (private.is_member(id));
create policy members_read on public.staff                 for select to authenticated using (private.is_member(business_id));
create policy members_read on public.business_hours        for select to authenticated using (private.is_member(business_id));
create policy members_read on public.blackouts             for select to authenticated using (private.is_member(business_id));
create policy members_read on public.catalog_items         for select to authenticated using (private.is_member(business_id));
create policy members_read on public.catalog_item_images   for select to authenticated using (private.is_member(business_id));
create policy members_read on public.item_options          for select to authenticated using (private.is_member(business_id));
create policy members_read on public.staff_services        for select to authenticated using (private.is_member(business_id));
create policy members_read on public.item_units            for select to authenticated using (private.is_member(business_id));
create policy members_read on public.item_component_groups for select to authenticated using (private.is_member(business_id));
create policy members_read on public.item_components       for select to authenticated using (private.is_member(business_id));
create policy members_read on public.customers             for select to authenticated using (private.is_member(business_id));
create policy members_read on public.bookings              for select to authenticated using (private.is_member(business_id));
create policy members_read on public.booking_items         for select to authenticated using (private.is_member(business_id));
create policy members_read on public.booking_resources     for select to authenticated using (private.is_member(business_id));
create policy members_read on public.payments              for select to authenticated using (private.is_member(business_id));
create policy members_read on public.rental_checkouts      for select to authenticated using (private.is_member(business_id));
create policy members_read on public.booking_events        for select to authenticated using (private.is_member(business_id));
create policy members_read on public.workflow_steps        for select to authenticated using (private.is_member(business_id));
create policy members_read on public.booking_steps         for select to authenticated using (private.is_member(business_id));

-- Equipo: el dueño ve a todos; cada miembro se ve a sí mismo (para saber su negocio y rol).
create policy owner_or_self_read on public.business_members for select to authenticated
  using (user_id = (select auth.uid()) or private.is_member(business_id, array['owner']));

-- Ajustes de avisos: solo el dueño. Las plantillas por defecto del giro las ve cualquier usuario.
create policy owner_read on public.notification_rules for select to authenticated
  using (private.is_member(business_id, array['owner']));
create policy owner_read on public.notifications for select to authenticated
  using (private.is_member(business_id, array['owner']));
create policy defaults_or_owner_read on public.notification_templates for select to authenticated
  using (business_id is null or private.is_member(business_id, array['owner']));
