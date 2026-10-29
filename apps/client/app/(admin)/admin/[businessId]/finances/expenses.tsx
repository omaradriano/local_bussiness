import type { components } from '@lb/api-client';
import { centavosToInput, formatMXN, parsePesos, strings, todayISO } from '@lb/shared';
import { Stack, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator, Pressable, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { currentMonth, PeriodPicker, type Period } from '../../../../../components/PeriodPicker';
import { api } from '../../../../../lib/api';
import { errorText, fieldErrors } from '../../../../../lib/errors';
import { useTexts } from '../../../../../lib/useTexts';
import { Button, Cell, Chip, Chips, Field, Group, Screen, Segmented, Text, useToast, useUI } from '../../../../../ui';

type Expense = components['schemas']['Expense'];
type Plan = components['schemas']['ExpensePlan'];
type Category = components['schemas']['ExpenseCategory'];
type Recurring = components['schemas']['RecurringExpense'];
type Frequency = components['schemas']['Frequency'];
const TABS = ['once', 'recurring', 'plans'] as const;
type Tab = (typeof TABS)[number];
const t = strings.finances;
const ISO = /^\d{4}-\d{2}-\d{2}$/;
const day = (iso: string) => new Date(`${iso.slice(0, 10)}T12:00:00Z`).toLocaleDateString('es-MX', { timeZone: 'UTC', day: 'numeric', month: 'short', year: 'numeric' });

function CategoryChips({ value, onChange }: { value: Category; onChange: (c: Category) => void }) {
  return (
    <View style={{ gap: 6, marginBottom: 16 }}>
      <Text variant="footnote" color="labelSecondary" style={{ fontWeight: '600', paddingHorizontal: 4 }}>{t.category}</Text>
      <Chips>
        {(t.expenseCategories as readonly Category[]).map((c) => <Chip key={c} selected={value === c} onPress={() => onChange(c)}>{t.categories[c]}</Chip>)}
      </Chips>
    </View>
  );
}

/** Expenses of a period (add / delete) and purchases in installments. */
export default function Expenses() {
  const { businessId, tab: initialTab } = useLocalSearchParams<{ businessId: string; tab?: Tab }>();
  const ui = useUI();
  const toast = useToast();
  const [period, setPeriod] = useState<Period>(currentMonth);
  const [list, setList] = useState<Expense[] | null>(null);
  const [plans, setPlans] = useState<Plan[] | null>(null);
  const [form, setForm] = useState({ spentOn: todayISO(), category: 'transport' as Category, amount: '', description: '' });
  const [plan, setPlan] = useState({ description: '', category: 'asset' as Category, purchasedOn: todayISO(), down: '', amount: '', count: '12', firstDue: todayISO() });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [planErrors, setPlanErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState<'expense' | 'plan' | null>(null);
  const { can } = useTexts();
  // Lite: only one-off expenses (recurring and installments come with Pro).
  const tabs = TABS.filter((k) => (k === 'recurring' ? can('recurring_expenses') : k === 'plans' ? can('installments') : true));
  const [tab, setTab] = useState<Tab>(initialTab ?? 'once');
  const path = { businessId };

  const load = useCallback(() => {
    api.GET('/v1/b/{businessId}/finances/expenses', { params: { path, query: { from: period.from, to: period.to } } }).then(({ data }) => setList(data?.expenses ?? []));
    if (can('installments')) api.GET('/v1/b/{businessId}/finances/plans', { params: { path } }).then(({ data }) => setPlans(data?.plans ?? []));
  }, [businessId, period]); // eslint-disable-line react-hooks/exhaustive-deps
  useFocusEffect(load);

  async function addExpense() {
    const amount = parsePesos(form.amount);
    const local: Record<string, string> = {};
    if (!ISO.test(form.spentOn)) local.spent_on = t.invalidDate;
    if (!amount) local.amount = strings.catalog.invalidMoney;
    if (Object.keys(local).length) return setErrors(local);
    setBusy('expense');
    const { data, error, response } = await api.POST('/v1/b/{businessId}/finances/expenses', {
      params: { path }, body: { spent_on: form.spentOn, category: form.category, amount: amount!, description: form.description },
    });
    setBusy(null);
    if (!data) { setErrors(fieldErrors(error)); return toast(errorText(error, response.status), 'error'); }
    setErrors({});
    setForm({ ...form, amount: '', description: '' });
    toast(t.expenseSaved);
    load();
  }

  async function addPlan() {
    const amount = parsePesos(plan.amount);
    const down = plan.down.trim() ? parsePesos(plan.down) : 0;
    const local: Record<string, string> = {};
    if (!ISO.test(plan.purchasedOn)) local.purchased_on = t.invalidDate;
    if (!ISO.test(plan.firstDue)) local.first_due = t.invalidDate;
    if (!amount) local.installment_amount = strings.catalog.invalidMoney;
    if (down === null) local.down_payment = strings.catalog.invalidMoney;
    if (Object.keys(local).length) return setPlanErrors(local);
    setBusy('plan');
    const { data, error, response } = await api.POST('/v1/b/{businessId}/finances/plans', {
      params: { path }, body: { description: plan.description, category: plan.category, purchased_on: plan.purchasedOn, down_payment: down!,
        installment_amount: amount!, installments: Number(plan.count) || 0, first_due: plan.firstDue },
    });
    setBusy(null);
    if (!data) { setPlanErrors(fieldErrors(error)); return toast(errorText(error, response.status), 'error'); }
    setPlanErrors({});
    setPlan({ ...plan, description: '', down: '', amount: '' });
    toast(t.planSaved);
    load();
  }

  async function remove(kind: 'expense' | 'plan', id: string) {
    const res = kind === 'expense'
      ? await api.DELETE('/v1/b/{businessId}/finances/expenses/{expenseId}', { params: { path: { businessId, expenseId: id } } })
      : await api.DELETE('/v1/b/{businessId}/finances/plans/{planId}', { params: { path: { businessId, planId: id } } });
    if (!res.response.ok) return toast(errorText(res.error, res.response.status), 'error');
    toast(kind === 'expense' ? t.expenseDeleted : t.planDeleted);
    load();
  }

  const trash = (onPress: () => void, label: string) => (
    <Pressable accessibilityRole="button" accessibilityLabel={`${t.remove} ${label}`} hitSlop={10} onPress={onPress}>
      <Ionicons name="trash-outline" size={18} color={ui.danger} />
    </Pressable>
  );
  const total = (list ?? []).reduce((n, e) => n + e.amount, 0);

  return (
    <Screen>
      <Stack.Screen options={{ title: t.sections.expenses }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 16 }}>{t.expenses}</Text>
      <View style={{ marginBottom: 20 }}>
        {tabs.length > 1 && <Segmented options={tabs.map((k) => ({ value: k, label: t.expenseTabs[k] }))} value={tab} onChange={setTab} />}
      </View>

      {tab === 'recurring' && <RecurringTab businessId={businessId} />}

      {tab === 'once' && <>
      <Group label={t.addExpense} padded>
        <Field label={t.date} value={form.spentOn} onChangeText={(spentOn) => setForm({ ...form, spentOn })} error={errors.spent_on} />
        <CategoryChips value={form.category} onChange={(category) => setForm({ ...form, category })} />
        <Field label={t.amount} value={form.amount} onChangeText={(amount) => setForm({ ...form, amount })} keyboardType="decimal-pad" error={errors.amount} />
        <Field label={t.description} placeholder={t.descriptionPlaceholder} value={form.description} onChangeText={(description) => setForm({ ...form, description })} />
        <Button variant="primary" icon="add" busy={busy === 'expense'} onPress={addExpense}>{t.addExpense}</Button>
      </Group>

      <View style={{ marginBottom: 16 }}><PeriodPicker value={period} onChange={setPeriod} /></View>
      {list === null ? <ActivityIndicator /> : (
        <Group label={`${t.expenses} · ${formatMXN(total)}`}>
          {list.length === 0 ? <Cell label={t.emptyExpenses} /> : null}
          {list.map((e) => (
            <Cell key={e.id} label={e.description || t.categories[e.category]} sublabel={`${day(e.spent_on)} · ${t.categories[e.category]}`}
              value={formatMXN(e.amount)} accessory={trash(() => remove('expense', e.id), e.description ?? '')} />
          ))}
        </Group>
      )}
      </>}

      {tab === 'plans' && <>
      <Group label={t.plans} footer={t.planHint}>
        {plans?.length === 0 ? <Cell label={t.emptyPlans} /> : null}
        {(plans ?? []).map((p) => (
          <Cell key={p.id} icon="card" tint={ui.purple} label={p.description}
            sublabel={`${p.installments} × ${formatMXN(p.installment_amount)} · ${t.firstDue} ${day(p.first_due)}`}
            accessory={trash(() => remove('plan', p.id), p.description)} />
        ))}
      </Group>

      <Group label={t.addPlan} padded>
        <Field label={t.planDescription} placeholder={t.planDescriptionPlaceholder} value={plan.description} onChangeText={(description) => setPlan({ ...plan, description })} error={planErrors.description} />
        <CategoryChips value={plan.category} onChange={(category) => setPlan({ ...plan, category })} />
        <Field label={t.purchasedOn} value={plan.purchasedOn} onChangeText={(purchasedOn) => setPlan({ ...plan, purchasedOn })} error={planErrors.purchased_on} />
        <Field label={t.downPayment} value={plan.down} onChangeText={(down) => setPlan({ ...plan, down })} keyboardType="decimal-pad" error={planErrors.down_payment} />
        <View style={{ flexDirection: 'row', gap: 12 }}>
          <View style={{ flex: 1 }}><Field label={t.installmentAmount} value={plan.amount} onChangeText={(amount) => setPlan({ ...plan, amount })} keyboardType="decimal-pad" error={planErrors.installment_amount} /></View>
          <View style={{ flex: 1 }}><Field label={t.installments} value={plan.count} onChangeText={(count) => setPlan({ ...plan, count })} keyboardType="numeric" error={planErrors.installments} /></View>
        </View>
        <Field label={t.firstDue} value={plan.firstDue} onChangeText={(firstDue) => setPlan({ ...plan, firstDue })} error={planErrors.first_due} />
        <Button variant="primary" icon="add" busy={busy === 'plan'} onPress={addPlan}>{t.addPlan}</Button>
      </Group>
      </>}
    </Screen>
  );
}

const rt = t.recurring;
const FREQUENCIES: Frequency[] = ['weekly', 'biweekly', 'monthly', 'yearly'];

/**
 * Recurring expenses (rent, internet, electricity): charges waiting for confirmation first (the real
 * amount, or «No se pagó»), then the list, then the form to add one.
 */
function RecurringTab({ businessId }: { businessId: string }) {
  const ui = useUI();
  const toast = useToast();
  const [list, setList] = useState<Recurring[] | null>(null);
  const [form, setForm] = useState({ description: '', category: 'rent' as Category, amount: '', frequency: 'monthly' as Frequency, startsOn: todayISO(), endsOn: '' });
  const [errors, setErrors] = useState<Record<string, string>>({});
  // The charge being confirmed, with what was really paid.
  const [open, setOpen] = useState<{ rec: Recurring; due: string; amount: string; paidOn: string } | null>(null);
  const [busy, setBusy] = useState<'add' | 'pay' | 'skip' | null>(null);
  const path = { businessId };

  const load = useCallback(() => {
    api.GET('/v1/b/{businessId}/finances/recurring', { params: { path } }).then(({ data }) => setList(data?.recurring ?? []));
  }, [businessId]); // eslint-disable-line react-hooks/exhaustive-deps
  useFocusEffect(load);

  async function add() {
    const amount = parsePesos(form.amount);
    const local: Record<string, string> = {};
    if (!ISO.test(form.startsOn)) local.starts_on = t.invalidDate;
    if (form.endsOn && !ISO.test(form.endsOn)) local.ends_on = t.invalidDate;
    if (!amount) local.amount = strings.catalog.invalidMoney;
    if (Object.keys(local).length) return setErrors(local);
    setBusy('add');
    const { data, error, response } = await api.POST('/v1/b/{businessId}/finances/recurring', {
      params: { path }, body: { description: form.description, category: form.category, amount: amount!, frequency: form.frequency,
        starts_on: form.startsOn, ...(form.endsOn ? { ends_on: form.endsOn } : {}) },
    });
    setBusy(null);
    if (!data) { setErrors(fieldErrors(error)); return toast(errorText(error, response.status), 'error'); }
    setErrors({});
    setForm({ ...form, description: '', amount: '', endsOn: '' });
    toast(rt.saved);
    load();
  }

  async function answer(kind: 'pay' | 'skip') {
    if (!open) return;
    const params = { path: { businessId, recurringId: open.rec.id } };
    let res;
    if (kind === 'pay') {
      const amount = parsePesos(open.amount);
      if (!amount) return setErrors({ paid_amount: strings.catalog.invalidMoney });
      if (!ISO.test(open.paidOn)) return setErrors({ paid_on: t.invalidDate });
      setBusy('pay');
      res = await api.POST('/v1/b/{businessId}/finances/recurring/{recurringId}/charges', { params, body: { due_on: open.due, amount, spent_on: open.paidOn } });
    } else {
      setBusy('skip');
      res = await api.POST('/v1/b/{businessId}/finances/recurring/{recurringId}/skips', { params, body: { due_on: open.due } });
    }
    setBusy(null);
    if (!res.response.ok) return toast(errorText(res.error, res.response.status), 'error');
    setErrors({});
    setOpen(null);
    toast(kind === 'pay' ? rt.confirmed : rt.skipped);
    load();
  }

  async function remove(rec: Recurring) {
    const res = await api.DELETE('/v1/b/{businessId}/finances/recurring/{recurringId}', { params: { path: { businessId, recurringId: rec.id } } });
    if (!res.response.ok) return toast(errorText(res.error, res.response.status), 'error');
    toast(rt.deleted);
    load();
  }

  if (!list) return <ActivityIndicator />;
  const pending = list.flatMap((rec) => rec.pending.map((due) => ({ rec, due: due.slice(0, 10) })));

  return (
    <>
      {pending.length > 0 && (
        <Group label={rt.pending} footer={rt.pendingHint}>
          {pending.map(({ rec, due }) => (
            <Cell key={`${rec.id}-${due}`} icon="time" tint={ui.warning} label={rec.description}
              sublabel={`${day(due)} · ${rt.approx(formatMXN(rec.amount))}`}
              onPress={() => { setErrors({}); setOpen({ rec, due, amount: centavosToInput(rec.amount), paidOn: due }); }} />
          ))}
        </Group>
      )}
      {open && (
        <Group label={`${open.rec.description} · ${day(open.due)}`} padded>
          <Field label={rt.paidAmount} value={open.amount} onChangeText={(amount) => setOpen({ ...open, amount })} keyboardType="decimal-pad" error={errors.paid_amount ?? errors.amount} />
          <Field label={rt.paidOn} value={open.paidOn} onChangeText={(paidOn) => setOpen({ ...open, paidOn })} error={errors.paid_on ?? errors.spent_on} />
          <View style={{ flexDirection: 'row', gap: 8 }}>
            <View style={{ flex: 1 }}><Button variant="primary" icon="checkmark" busy={busy === 'pay'} onPress={() => answer('pay')}>{rt.confirm}</Button></View>
            <View style={{ flex: 1 }}><Button busy={busy === 'skip'} onPress={() => answer('skip')}>{rt.skip}</Button></View>
          </View>
        </Group>
      )}

      <Group label={rt.title} footer={rt.hint}>
        {list.length === 0 ? <Cell label={rt.empty} /> : null}
        {list.map((rec) => (
          <Cell key={rec.id} icon="repeat" tint={ui.indigo} label={rec.description}
            sublabel={`${t.categories[rec.category]} · ${rec.next_due ? rt.next(day(rec.next_due)) : rt.ended}`}
            value={`${formatMXN(rec.amount)} ${rt.per[rec.frequency]}`}
            accessory={
              <Pressable accessibilityRole="button" accessibilityLabel={`${t.remove} ${rec.description}`} hitSlop={10} onPress={() => remove(rec)}>
                <Ionicons name="trash-outline" size={18} color={ui.danger} />
              </Pressable>} />
        ))}
      </Group>

      <Group label={rt.add} padded>
        <Field label={rt.description} placeholder={rt.descriptionPlaceholder} value={form.description} onChangeText={(description) => setForm({ ...form, description })} error={errors.description} />
        <CategoryChips value={form.category} onChange={(category) => setForm({ ...form, category })} />
        <Field label={rt.amount} hint={rt.amountHint} value={form.amount} onChangeText={(amount) => setForm({ ...form, amount })} keyboardType="decimal-pad" error={errors.amount} />
        <View style={{ gap: 6, marginBottom: 16 }}>
          <Text variant="footnote" color="labelSecondary" style={{ fontWeight: '600', paddingHorizontal: 4 }}>{rt.frequency}</Text>
          <Segmented options={FREQUENCIES.map((f) => ({ value: f, label: rt.frequencies[f] }))} value={form.frequency} onChange={(frequency) => setForm({ ...form, frequency })} />
        </View>
        <View style={{ flexDirection: 'row', gap: 12 }}>
          <View style={{ flex: 1 }}><Field label={rt.startsOn} value={form.startsOn} onChangeText={(startsOn) => setForm({ ...form, startsOn })} error={errors.starts_on} /></View>
          <View style={{ flex: 1 }}><Field label={rt.endsOn} value={form.endsOn} onChangeText={(endsOn) => setForm({ ...form, endsOn })} error={errors.ends_on} /></View>
        </View>
        <Button variant="primary" icon="add" busy={busy === 'add'} onPress={add}>{rt.add}</Button>
      </Group>
    </>
  );
}
