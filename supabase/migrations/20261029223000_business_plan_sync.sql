-- businesses.plan copia el plan de la cuenta del dueño (account_plans) para que cada lectura de las
-- funciones activas no tenga que buscarlo. La API la mantiene al crear un negocio y al cambiar el plan.
update public.businesses b set plan = coalesce((
  select ap.plan from public.business_members m join public.account_plans ap on ap.user_id = m.user_id
  where m.business_id = b.id and m.role = 'owner' order by m.created_at limit 1), 'pro');

alter table public.businesses add constraint businesses_plan_known check (plan in ('trial', 'lite', 'pro', 'business'));
