import type { components } from '@lb/api-client';
import { centavosToInput, parsePesos, strings } from '@lb/shared';
import { Stack, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, StyleSheet, View } from 'react-native';
import { api } from '../../../../../lib/api';
import { errorText, fieldErrors } from '../../../../../lib/errors';
import { Button, EmptyState, Field, Screen, Segmented, Text, radius, useToast, useUI } from '../../../../../ui';

type Pay = components['schemas']['StaffPay'];
type PeriodKey = Pay['salary_period'];
const t = strings.finances;

/** One person's pay: fixed salary per period and/or commission. */
function PayCard({ businessId, pay, first }: { businessId: string; pay: Pay; first: boolean }) {
  const ui = useUI();
  const toast = useToast();
  const [salary, setSalary] = useState(pay.salary ? centavosToInput(pay.salary) : '');
  const [period, setPeriod] = useState<PeriodKey>(pay.salary_period);
  const [commission, setCommission] = useState(pay.commission_percent ? String(pay.commission_percent) : '');
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);

  async function save() {
    const cents = salary.trim() ? parsePesos(salary) : 0;
    if (cents === null) return setErrors({ salary: strings.catalog.invalidMoney });
    setBusy(true);
    const { data, error, response } = await api.PUT('/v1/b/{businessId}/finances/pay/{staffId}', {
      params: { path: { businessId, staffId: pay.staff_id } },
      body: { salary: cents, salary_period: period, commission_percent: Number(commission.replace(',', '.')) || 0 },
    });
    setBusy(false);
    if (data) { setErrors({}); return toast(t.paySaved); }
    setErrors(fieldErrors(error));
    toast(errorText(error, response.status), 'error');
  }

  return (
    <View style={{ padding: 16, gap: 4, borderTopWidth: first ? 0 : StyleSheet.hairlineWidth, borderColor: ui.separator, opacity: pay.active ? 1 : 0.6 }}>
      <Text variant="headline" style={{ marginBottom: 8 }}>{pay.name}</Text>
      <View style={{ flexDirection: 'row', gap: 12 }}>
        <View style={{ flex: 1 }}><Field label={t.salary} value={salary} onChangeText={setSalary} keyboardType="decimal-pad" placeholder="0" error={errors.salary} /></View>
        <View style={{ flex: 1 }}><Field label={t.commissionPercent} value={commission} onChangeText={setCommission} keyboardType="decimal-pad" placeholder="0" error={errors.commission_percent} /></View>
      </View>
      <View style={{ marginBottom: 12 }}>
        <Segmented options={(['weekly', 'biweekly', 'monthly'] as PeriodKey[]).map((k) => ({ value: k, label: t.salaryPeriod[k] }))} value={period} onChange={setPeriod} />
      </View>
      <Button size="sm" busy={busy} onPress={save}>{t.savePay}</Button>
    </View>
  );
}

export default function PayScreen() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const ui = useUI();
  const [list, setList] = useState<Pay[] | null>(null);
  useEffect(() => {
    api.GET('/v1/b/{businessId}/finances/pay', { params: { path: { businessId } } }).then(({ data }) => setList(data?.pay ?? []));
  }, [businessId]);

  return (
    <Screen>
      <Stack.Screen options={{ title: t.sections.pay }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 8 }}>{t.sections.pay}</Text>
      <Text variant="subhead" color="labelSecondary" style={{ marginBottom: 20 }}>{t.payHint}</Text>
      {!list && <ActivityIndicator />}
      {list?.length === 0 && <EmptyState icon="people-outline" title={t.noStaff} />}
      {!!list?.length && (
        <View style={{ backgroundColor: ui.surface, borderRadius: radius.sm, overflow: 'hidden' }}>
          {list.map((p, i) => <PayCard key={p.staff_id} businessId={businessId} pay={p} first={i === 0} />)}
        </View>
      )}
    </Screen>
  );
}
