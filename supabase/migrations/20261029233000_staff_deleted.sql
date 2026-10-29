-- Borrar a una persona del equipo: desaparece de la lista, la agenda y Sueldos, pero las citas y
-- comisiones pasadas conservan su nombre (borrar la fila rompería ese historial).
alter table public.staff add column deleted_at timestamptz;
