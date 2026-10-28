-- Evidencia de un pago (foto de la transferencia o del ticket de la terminal), en el bucket privado.
alter table public.payments
  add column evidence_keys text[] not null default '{}' check (cardinality(evidence_keys) <= 4);
