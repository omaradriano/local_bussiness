import type { components } from '@lb/api-client';
import { formatMXN, parsePesos, strings, todayISO } from '@lb/shared';
import { Stack, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, View } from 'react-native';
import { DayNav } from '../../../../../components/DayNav';
import { api } from '../../../../../lib/api';
import { errorText, fieldErrors } from '../../../../../lib/errors';
import { Button, Cell, Field, Group, Notice, Row, Screen, Text, useToast } from '../../../../../ui';

type Report = components['schemas']['CashReport'];
const t = strings.finances.cash;

const when = (iso: string) => new Date(iso).toLocaleString('es-MX', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit', hourCycle: 'h23' });

/** Corte de caja: what came in and went out by payment method on a day, and the cash counted at closing. */
export default function Cash() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const toast = useToast();
  const [day, setDay] = useState(todayISO());
  const [rep, setRep] = useState<Report | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [counted, setCounted] = useState('');
  const [note, setNote] = useState('');
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setRep(null);
    setError(null);
    const { data, error: e, response } = await api.GET('/v1/b/{businessId}/finances/cash', { params: { path: { businessId }, query: { day } } });
    if (data) { setRep(data); setCounted(''); setNote(data.closing?.note ?? ''); }
    else setError(errorText(e, response.status));
  }, [businessId, day]);
  useEffect(() => { load(); }, [load]);

  async function close() {
    const cents = parsePesos(counted);
    if (cents === null) return setErrors({ counted: strings.catalog.invalidMoney });
    setBusy(true);
    const { data, error: e, response } = await api.POST('/v1/b/{businessId}/finances/cash', { params: { path: { businessId } }, body: { day, counted: cents, note } });
    setBusy(false);
    if (!data) { setErrors(fieldErrors(e)); return toast(errorText(e, response.status), 'error'); }
    setErrors({});
    setRep(data);
    setCounted('');
    toast(t.saved);
  }

  const c = rep?.closing;
  return (
    <Screen>
      <Stack.Screen options={{ title: t.title }} />
      <DayNav day={day} onChange={setDay} />
      {error && (<Group padded><Notice tone="warning">{error}</Notice><Button onPress={load} style={{ marginTop: 12 }}>{strings.common.retry}</Button></Group>)}
      {!rep && !error && <ActivityIndicator />}
      {rep && (
        <View style={{ gap: 16 }}>
          {rep.lines.length === 0 && <Group><Cell label={t.nothing} /></Group>}
          {rep.lines.map((l) => (
            <Group key={l.method} label={strings.bookings.methods[l.method]} padded>
              <View style={{ gap: 6 }}>
                {l.orders > 0 && <Row label={t.orders} value={formatMXN(l.orders)} />}
                {l.sales > 0 && <Row label={t.sales} value={formatMXN(l.sales)} />}
                {l.deposits > 0 && <Row label={t.deposits} value={formatMXN(l.deposits)} />}
                {l.refunds > 0 && <Row label={t.refunds} value={`− ${formatMXN(l.refunds)}`} />}
                <Row label={t.net} value={formatMXN(l.net)} strong />
              </View>
            </Group>
          ))}
          {rep.lines.length > 0 && <Group padded><Row label={t.total} value={formatMXN(rep.total)} strong /></Group>}

          <Group label={t.count} footer={t.countHint} padded>
            <View style={{ gap: 12 }}>
              <Row label={t.expected} value={formatMXN(rep.expected_cash)} strong />
              {c && (
                <>
                  <Row label={t.counted} value={formatMXN(c.counted)} />
                  <Row label={c.difference < 0 ? t.missing : c.difference > 0 ? t.over : t.exact} value={formatMXN(Math.abs(c.difference))} strong />
                  <Text variant="footnote" color="labelSecondary">{t.closedBy(c.closed_by, when(c.closed_at))}</Text>
                  {c.expected !== rep.expected_cash && <Notice tone="warning">{t.changed(formatMXN(c.expected))}</Notice>}
                </>
              )}
              <Field label={c ? t.recount : t.countedField} value={counted} onChangeText={(v) => { setCounted(v); setErrors({}); }} keyboardType="decimal-pad" error={errors.counted} />
              <Field label={t.note} value={note} onChangeText={setNote} maxLength={200} error={errors.note} />
              <Button variant="primary" icon="checkmark-circle" busy={busy} disabled={!counted.trim()} onPress={close}>{t.save}</Button>
            </View>
          </Group>
        </View>
      )}
    </Screen>
  );
}
