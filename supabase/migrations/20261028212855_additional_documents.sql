-- "Documentos adicionales" de una renta: pasos con config.section = 'additional'. El primero es el
-- pagaré firmado por el cliente, que se fotografía antes de la salida.
insert into public.workflow_steps (business_id, applies_to_kind, actor, moment, to_status, step_type, label, config, required, sort)
select b.id, 'rental', 'business', 'before_status', 'checked_out', 'photo', 'Pagaré',
       '{"bucket": "private-docs", "max_files": 4, "section": "additional"}'::jsonb, true,
       coalesce((select max(sort) from public.workflow_steps ws where ws.business_id = b.id), 0) + 1
from public.businesses b
where b.vertical = 'tool_rental'
  and not exists (select 1 from public.workflow_steps x where x.business_id = b.id and x.label = 'Pagaré');
