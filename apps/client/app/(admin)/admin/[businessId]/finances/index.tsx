import type { components } from '@lb/api-client';
import { formatMXN, strings } from '@lb/shared';
import { router, Stack, useFocusEffect, useLocalSearchParams, type Href } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator, View } from 'react-native';
import { Donut } from '../../../../../components/Donut';
import { currentMonth, PeriodPicker, type Period } from '../../../../../components/PeriodPicker';
import { api } from '../../../../../lib/api';
import { errorText } from '../../../../../lib/errors';
import { useTexts } from '../../../../../lib/useTexts';
import { Cell, Group, Notice, Row, Screen, Text, radius, useUI } from '../../../../../ui';

type Summary = components['schemas']['FinanceSummary'];
const t = strings.finances;
const day = (iso: string | null) => (iso ? new Date(`${iso.slice(0, 10)}T12:00:00Z`).toLocaleDateString('es-MX', { timeZone: 'UTC', day: 'numeric', month: 'short', year: 'numeric' }) : '');
// Stable color order: a category keeps its color whatever the period.
const INCOME_KEYS = ['bookings', 'products', 'fees'];
const EXPENSE_KEYS = ['payroll', 'commissions', 'merchandise', 'transport', 'supplies', 'asset', 'rent', 'maintenance', 'utilities', 'advertising', 'taxes', 'other'];

/** The owner's money for a period: profit, where income came from, where it went. */
export default function Finances() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const ui = useUI();
  const { texts, kind } = useTexts();
  const [period, setPeriod] = useState<Period>(currentMonth);
  const [s, setS] = useState<Summary | null>(null);
  const [error, setError] = useState<string | null>(null);

  useFocusEffect(useCallback(() => {
    setS(null);
    api.GET('/v1/b/{businessId}/finances/summary', { params: { path: { businessId }, query: { from: period.from, to: period.to } } })
      .then(({ data, error: err, response }) => (data ? (setS(data), setError(null)) : setError(errorText(err, response.status))));
  }, [businessId, period]));

  const go = (path: string) => () => router.push(`/admin/${businessId}/finances/${path}` as Href);

  return (
    <Screen>
      <Stack.Screen options={{ title: t.title }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 16 }}>{t.title}</Text>
      <View style={{ marginBottom: 20 }}><PeriodPicker value={period} onChange={setPeriod} /></View>
      {error && <Notice tone="warning">{error}</Notice>}
      {!s && !error && <ActivityIndicator />}
      {s && (
        <View style={{ gap: 20 }}>
          {/* Hero: the one number that matters, then its two parts. */}
          <View style={{ backgroundColor: ui.surface, borderRadius: radius.md, padding: 16, gap: 10 }}>
            <Text variant="subhead" color="labelSecondary">{s.profit >= 0 ? t.profit : t.loss}</Text>
            <Text variant="largeTitle" style={{ color: s.profit >= 0 ? ui.success : ui.danger, fontVariant: ['tabular-nums'] }}>
              {s.profit < 0 ? '−' : ''}{formatMXN(Math.abs(s.profit))}
            </Text>
            <Row label={t.income} value={formatMXN(s.income.total)} />
            <Row label={t.expenses} value={formatMXN(s.expenses.total)} />
            <Text variant="footnote" color="labelSecondary">{t.incomeHint}</Text>
          </View>

          {s.pending_recurring > 0 && (
            <Group>
              <Cell icon="time" tint={ui.warning} label={t.recurring.toConfirm(s.pending_recurring)} onPress={go('expenses?tab=recurring')} />
            </Group>
          )}

          {(kind === 'rental' || s.overdue.length > 0) && (
            <Group label={t.overdue}>
              {s.overdue.length === 0 ? <Cell icon="checkmark-circle" tint={ui.success} label={t.noOverdue} /> : null}
              {s.overdue.map((o) => (
                <Cell key={o.booking_id} icon="alert-circle" tint={ui.danger} label={o.customer}
                  sublabel={[o.item, t.dueOn(day(o.due))].filter(Boolean).join(' · ')} value={t.daysLate(o.days_late)}
                  onPress={() => router.push(`/admin/${businessId}/bookings/${o.booking_id}` as Href)} />
              ))}
            </Group>
          )}

          <View style={{ backgroundColor: ui.surface, borderRadius: radius.md, padding: 16 }}>
            <Donut title={t.incomeChart} total={s.income.total} empty={t.noIncome} colorKeys={INCOME_KEYS}
              slices={s.income.parts.map((p) => ({ key: p.key, label: t.incomeSources[p.key] ?? p.key, amount: p.amount }))} />
          </View>
          <View style={{ backgroundColor: ui.surface, borderRadius: radius.md, padding: 16 }}>
            <Donut title={t.expensesChart} total={s.expenses.total} empty={t.noExpenses} colorKeys={EXPENSE_KEYS}
              slices={s.expenses.parts.map((p) => ({ key: p.key, label: t.categories[p.key] ?? p.key, amount: p.amount }))} />
          </View>

          <Group label={t.byStatus(texts.bookings.plural)} footer={t.byStatusHint}>
            {s.bookings.length === 0 ? <Cell label={t.noBookings(texts.bookings.plural)} /> : null}
            {s.bookings.map((b) => <Cell key={b.key} label={texts.bookings.status[b.key]} value={String(b.count)} />)}
          </Group>

          {s.plans.length > 0 && (
            <Group label={t.plans}>
              {s.plans.map((p) => (
                <Cell key={p.id} icon={p.status === 'finished' ? 'checkmark-circle' : 'card'} tint={p.status === 'finished' ? ui.success : ui.purple}
                  label={p.description}
                  sublabel={p.status === 'finished' ? `${t.finished} · ${t.planLine(p.paid, p.installments)}`
                    : `${t.planLine(p.paid, p.installments)} · ${t.nextDue}: ${day(p.next_due)}`}
                  value={p.status === 'finished' ? undefined : `${t.remaining} ${formatMXN(p.remaining)}`} />
              ))}
            </Group>
          )}

          {s.payroll.length > 0 && (
            <Group label={t.payroll}>
              {s.payroll.map((p) => (
                <Cell key={p.staff_id} icon="person" tint={ui.indigo} label={p.name}
                  sublabel={[p.salary ? `${t.salary} ${formatMXN(p.salary)}` : '', p.commission ? `${t.commission} ${formatMXN(p.commission)}` : ''].filter(Boolean).join(' · ')}
                  value={formatMXN(p.salary + p.commission)} />
              ))}
            </Group>
          )}

          <Group separatorInset={58}>
            <Cell icon="receipt" tint={ui.warning} label={t.sections.expenses} sublabel={t.sectionHints.expenses} onPress={go('expenses')} />
            <Cell icon="people" tint={ui.indigo} label={t.sections.pay} sublabel={t.sectionHints.pay} onPress={go('pay')} />
          </Group>
        </View>
      )}
    </Screen>
  );
}
