-- Money given back when a booking with payments is cancelled (not the security deposit, that's `refund`).
alter table public.payments drop constraint payments_type_check;
alter table public.payments add constraint payments_type_check
  check (type in ('advance', 'balance', 'security_deposit', 'refund', 'late_fee', 'damage_fee', 'payment_refund'));
