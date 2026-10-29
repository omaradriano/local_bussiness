import { exportColumns, exportFileName, strings, term, todayISO, toCSV, type ExportKind } from '@lb/shared';
import { useState } from 'react';
import { Platform, View } from 'react-native';
import { api } from '../lib/api';
import { downloadText } from '../lib/download';
import { errorText } from '../lib/errors';
import { useBusiness } from '../lib/useMe';
import { useTexts } from '../lib/useTexts';
import { Cell, Group, useToast, useUI } from '../ui';
import { currentMonth, PeriodPicker, type Period } from './PeriodPicker';

const t = strings.exports;
const DATED: ExportKind[] = ['bookings', 'expenses', 'sales'];

/** Owner's CSV downloads: customers, bookings, expenses, sales, catalog and products (also when expired). */
export function ExportData({ businessId }: { businessId: string }) {
  const ui = useUI();
  const toast = useToast();
  const { business } = useBusiness(businessId);
  const { texts, has } = useTexts();
  const [period, setPeriod] = useState<Period>(currentMonth);
  const [busy, setBusy] = useState<ExportKind | null>(null);
  if (!business) return null;

  const kinds: { kind: ExportKind; label: string; icon: 'people' | 'list' | 'receipt' | 'pricetags' | 'construct' | 'cube' }[] = [
    { kind: 'customers', label: t.kinds.customers, icon: 'people' },
    { kind: 'bookings', label: texts.bookings.plural, icon: 'list' },
    { kind: 'expenses', label: t.kinds.expenses, icon: 'receipt' },
    ...(has('productSales') ? [{ kind: 'sales' as const, label: t.kinds.sales, icon: 'pricetags' as const }] : []),
    { kind: 'catalog', label: term(business.vertical, 'catalog') ?? strings.business.sections.catalog, icon: 'construct' },
    ...(has('productSales') ? [{ kind: 'products' as const, label: t.kinds.products, icon: 'cube' as const }] : []),
  ];

  async function download(kind: ExportKind, label: string) {
    if (Platform.OS !== 'web') return toast(t.onlyWeb, 'info');
    setBusy(kind);
    const { data, error, response } = await api.GET('/v1/b/{businessId}/export/{kind}', {
      params: { path: { businessId, kind }, query: DATED.includes(kind) ? { from: period.from, to: period.to } : {} },
    });
    setBusy(null);
    if (!data) return toast(errorText(error, response.status), 'error');
    if (!data.rows.length) return toast(t.empty, 'info');
    const csv = toCSV(exportColumns(kind, texts, data.timezone), data.rows as never[]);
    downloadText(exportFileName(business!.slug, DATED.includes(kind) ? `${label} ${period.from} ${period.to}` : label, todayISO()), csv);
    toast(t.done);
  }

  return (
    <Group label={t.title} footer={t.hint}>
      <View style={{ padding: 12 }}><PeriodPicker value={period} onChange={setPeriod} /></View>
      {kinds.map((k) => (
        <Cell key={k.kind} icon={k.icon} tint={ui.accent} label={k.label} value={busy === k.kind ? '…' : 'CSV'}
          onPress={busy ? undefined : () => download(k.kind, k.label)} />
      ))}
    </Group>
  );
}
