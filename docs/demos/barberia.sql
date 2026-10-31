-- Barbería demo para presentar Khronno (SQL Editor de Supabase, el proyecto donde esté el negocio).
--
-- Antes: en la app, crea el negocio con giro «Barbería», nombre «Barbería Demo» (el nombre debe
-- incluir «Demo») y slug «barberia-demo». Después corre este script completo.
--
-- Llena: 3 barberos (el dueño + Toño + Memo) con horario y servicios, sueldos y comisiones,
-- 12 clientes, citas de las últimas 2 semanas (atendidas y cobradas, algunas «no llegó"), de hoy y
-- de los próximos 6 días, productos con ventas y gastos del mes. Se puede volver a correr: borra
-- y recrea los datos de ESE negocio (solo si su nombre incluye «Demo»).
-- Teléfonos ficticios (+52 614 555 0001…), montos en centavos.

do $$
declare
  v_slug  constant text := 'barberia-demo';
  v_email constant text := 'oadrian38@gmail.com';
  v_biz uuid; v_name text; v_tz text; v_user uuid; v_owner uuid; v_tono uuid; v_memo uuid;
  v_today date; v_off int; v_pct int;
  v_items uuid[]; v_names text[]; v_prices bigint[]; v_mins int[];
  v_custs uuid[];
  v_prods uuid[]; v_pprices bigint[]; v_pcosts bigint[];
  r record; i int; v_start timestamptz; v_end timestamptz; v_status text; v_bk uuid; v_sale uuid; v_qty int;
  v_methods constant text[] := array['cash', 'card', 'transfer'];
begin
  select b.id, b.name, b.timezone, m.user_id, m.staff_id into v_biz, v_name, v_tz, v_user, v_owner
  from public.businesses b
  join public.business_members m on m.business_id = b.id and m.role = 'owner'
  join auth.users u on u.id = m.user_id and u.email = v_email
  where b.slug = v_slug and b.vertical = 'barbershop';
  if v_biz is null then
    raise exception 'No encontré la barbería «%» del dueño %: créala primero en la app.', v_slug, v_email;
  end if;
  if v_name !~* 'demo' then
    raise exception 'Por seguridad el nombre del negocio debe incluir «Demo» (es «%»): este script borra sus datos.', v_name;
  end if;
  if v_owner is null then
    raise exception 'El dueño no aparece como barbero de «%».', v_name;
  end if;
  v_today := (now() at time zone v_tz)::date;

  -- Empezar de cero (solo este negocio).
  delete from public.sales where business_id = v_biz;
  delete from public.bookings where business_id = v_biz;
  delete from public.expenses where business_id = v_biz;
  delete from public.retail_products where business_id = v_biz;
  delete from public.customers where business_id = v_biz;
  delete from public.staff where business_id = v_biz and id <> v_owner;

  -- Barberos: el dueño + dos más, lunes a sábado 10:00–20:00.
  update public.staff set name = 'Omar', color = '#1565C0', active = true where id = v_owner;
  insert into public.staff (business_id, name, color) values (v_biz, 'Toño', '#2E7D32') returning id into v_tono;
  insert into public.staff (business_id, name, color) values (v_biz, 'Memo', '#EF6C00') returning id into v_memo;
  insert into public.business_hours (business_id, staff_id, weekday, opens_at, closes_at)
  select v_biz, s, d, '10:00', '20:00' from unnest(array[v_tono, v_memo]) s, generate_series(1, 6) d;

  select array_agg(id order by sort, name), array_agg(name order by sort, name), array_agg(price order by sort, name),
         array_agg(duration_minutes order by sort, name)
  into v_items, v_names, v_prices, v_mins
  from public.catalog_items where business_id = v_biz and kind = 'service' and active;
  if v_items is null then
    raise exception 'La barbería no tiene servicios activos.';
  end if;
  insert into public.staff_services (business_id, staff_id, item_id)
  select v_biz, s, it from unnest(array[v_owner, v_tono, v_memo]) s, unnest(v_items) it
  on conflict do nothing;

  -- Sueldos: Toño sueldo semanal + comisión; Memo solo comisión.
  insert into public.staff_pay (staff_id, business_id, salary, salary_period, commission_percent) values
    (v_tono, v_biz, 150000, 'weekly', 30),
    (v_memo, v_biz, 0, 'weekly', 50)
  on conflict (staff_id) do update set salary = excluded.salary, salary_period = excluded.salary_period,
    commission_percent = excluded.commission_percent;

  -- Clientes.
  with c(n, name) as (values
    (1, 'Juan Pérez'), (2, 'Carlos Ramírez'), (3, 'Luis Hernández'), (4, 'Miguel Torres'),
    (5, 'Jorge Martínez'), (6, 'Ricardo Flores'), (7, 'Andrés Gómez'), (8, 'Fernando Díaz'),
    (9, 'Roberto Castillo'), (10, 'Diego Morales'), (11, 'Sergio Vargas'), (12, 'Alejandro Ruiz'))
  insert into public.customers (business_id, name, phone, created_at)
  select v_biz, name, '+52614555' || lpad(n::text, 4, '0'), now() - interval '30 days' from c;
  select array_agg(id order by phone) into v_custs from public.customers where business_id = v_biz;

  -- Citas: de hace 13 días a dentro de 6, sin domingos. Horarios separados 70 min (el servicio más
  -- largo dura 60 + 5 de margen), así ningún barbero se empalma.
  for r in
    select d::date as day, s.staff_id, s.k, t.at, abs(hashtext(d::date::text || s.k || t.at::text)) as h
    from generate_series(v_today - 13, v_today + 6, interval '1 day') d
    cross join (values (v_owner, 'a'), (v_tono, 'b'), (v_memo, 'c')) s(staff_id, k)
    cross join unnest(array['10:00', '11:10', '12:20', '13:30', '15:00', '16:10', '17:20', '18:30']::time[]) t(at)
    where extract(dow from d) <> 0
    order by 1, 4, 3
  loop
    v_off := r.day - v_today;
    v_pct := case when v_off < 0 then 70 when v_off = 0 then 75 else greatest(15, 60 - v_off * 8) end;
    continue when r.h % 100 >= v_pct;
    i := (r.h / 100) % array_length(v_items, 1) + 1;
    v_start := (r.day + r.at) at time zone v_tz;
    v_end := v_start + make_interval(mins => v_mins[i]);
    v_status := case when v_end > now() then 'confirmed' when r.h % 19 = 0 then 'no_show' else 'completed' end;

    insert into public.bookings (business_id, customer_id, kind, status, starts_at, ends_at, subtotal, deposit_required, total,
      source, created_by, created_at, updated_at)
    values (v_biz, v_custs[(r.h / 7) % array_length(v_custs, 1) + 1], 'service', v_status, v_start, v_end, v_prices[i], 0, v_prices[i],
      'admin', v_user, least(v_start - interval '1 day', now()), least(v_end, now()))
    returning id into v_bk;
    insert into public.booking_items (business_id, booking_id, item_id, quantity, unit_price, options)
    values (v_biz, v_bk, v_items[i], 1, v_prices[i], jsonb_build_object('item_name', v_names[i], 'periods', 1, 'options_extra', 0,
      'delivery_fee', 0, 'security_deposit', 0, 'answers', '{}'::jsonb));
    if v_status <> 'no_show' then -- «no llegó» libera el horario
      insert into public.booking_resources (business_id, booking_id, staff_id, during)
      values (v_biz, v_bk, r.staff_id, tstzrange(v_start, v_end + interval '5 minutes'));
    end if;
    insert into public.booking_events (business_id, booking_id, from_status, to_status, actor, at, note)
    values (v_biz, v_bk, null, 'confirmed', v_user, least(v_start - interval '1 day', now()), 'Alta (admin)');
    if v_status <> 'confirmed' then
      insert into public.booking_events (business_id, booking_id, from_status, to_status, actor, at)
      values (v_biz, v_bk, 'confirmed', v_status, v_user, v_end);
    end if;
    if v_status = 'completed' then
      insert into public.payments (business_id, booking_id, type, method, amount, paid_at, recorded_by)
      values (v_biz, v_bk, 'balance', v_methods[r.h % 3 + 1], v_prices[i], v_end, v_user);
    end if;
  end loop;

  -- Productos (venta de productos) y algunas ventas en mostrador.
  with p(n, name, price, cost) as (values
    (1, 'Cera mate', 18000, 9000), (2, 'Aceite para barba', 22000, 11000), (3, 'Shampoo anticaspa', 15000, 7000))
  insert into public.retail_products (business_id, name, price, cost, stock)
  select v_biz, name, price, cost, 20 from p;
  select array_agg(id order by name), array_agg(price order by name), array_agg(cost order by name)
  into v_prods, v_pprices, v_pcosts
  from public.retail_products where business_id = v_biz;
  for r in
    select d::date as day, abs(hashtext('venta' || d::date::text)) as h
    from generate_series(v_today - 13, v_today - 1, interval '1 day') d
    where extract(dow from d) <> 0
  loop
    continue when r.h % 100 >= 60;
    i := r.h % 3 + 1;
    v_qty := r.h % 2 + 1;
    insert into public.sales (business_id, sold_at, method, total, cost_total, created_by)
    values (v_biz, (r.day + time '17:00') at time zone v_tz, v_methods[r.h % 3 + 1], v_pprices[i] * v_qty, v_pcosts[i] * v_qty, v_user)
    returning id into v_sale;
    insert into public.sale_lines (business_id, sale_id, product_id, quantity, unit_price, unit_cost)
    values (v_biz, v_sale, v_prods[i], v_qty, v_pprices[i], v_pcosts[i]);
    update public.retail_products set stock = stock - v_qty where id = v_prods[i];
  end loop;

  -- Gastos del mes.
  insert into public.expenses (business_id, spent_on, category, amount, description, created_by) values
    (v_biz, date_trunc('month', v_today)::date, 'rent', 800000, 'Renta del local', v_user),
    (v_biz, v_today - 6, 'utilities', 120000, 'Luz y agua', v_user),
    (v_biz, v_today - 4, 'supplies', 45000, 'Navajas, talco y toallas', v_user);

  raise notice 'Listo: % con % citas.', v_name, (select count(*) from public.bookings where business_id = v_biz);
end $$;
