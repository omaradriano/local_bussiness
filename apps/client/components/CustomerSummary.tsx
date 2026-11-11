import type { components } from '@lb/api-client';
import { formatMXN, formatPhone, strings } from '@lb/shared';
import { useEffect, useState } from 'react';
import { View } from 'react-native';
import { api } from '../lib/api';
import { useTexts } from '../lib/useTexts';
import { Cell, Group, Row } from '../ui';

type Summary = components['schemas']['CustomerSummary'];
const t = strings.customers.summary;

const day = (iso: string) => new Date(iso).toLocaleDateString('es-MX', { day: 'numeric', month: 'short', year: 'numeric' });

/** Top of a customer's profile: what they've spent and owe, their orders, and who they send them to. */
export function CustomerSummary({ businessId, customerId }: { businessId: string; customerId: string }) {
  const { texts, has } = useTexts();
  const [s, setS] = useState<Summary | null>(null);

  useEffect(() => {
    api.GET('/v1/b/{businessId}/customers/{customerId}/summary', { params: { path: { businessId, customerId } } }).then(({ data }) => setS(data ?? null));
  }, [businessId, customerId]);

  if (!s || s.orders === 0) return null;
  return (
    <View style={{ gap: 16, marginBottom: 20 }}>
      <Group padded>
        <View style={{ gap: 6 }}>
          <Row label={t.spent} value={formatMXN(s.spent)} strong />
          <Row label={texts.bookings.plural} value={String(s.orders)} />
          {s.due > 0 && <Row label={t.due} value={formatMXN(s.due)} />}
          {s.first_at && <Row label={t.first} value={day(s.first_at)} />}
          {s.last_at && <Row label={t.last} value={day(s.last_at)} />}
        </View>
      </Group>
      {has('giftOrder') && s.recipients.length > 0 && (
        <Group label={t.recipients}>
          {s.recipients.map((r) => (
            <Cell key={r.name} icon="gift" label={r.name}
              sublabel={`${t.times(r.count)} · ${t.lastTime(day(r.last_at))}${r.phone ? ` · ${formatPhone(r.phone)}` : ''}`} />
          ))}
        </Group>
      )}
    </View>
  );
}
