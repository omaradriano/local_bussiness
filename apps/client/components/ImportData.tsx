import { importTemplate, rowsFromCSV, strings, term, type ImportKind } from '@lb/shared';
import { useState } from 'react';
import { Modal, Platform, Pressable, ScrollView, View } from 'react-native';
import { api } from '../lib/api';
import { downloadText, pickTextFile } from '../lib/download';
import { errorText } from '../lib/errors';
import { useBusiness } from '../lib/useMe';
import { useTexts } from '../lib/useTexts';
import { Button, Cell, Group, Text, radius, useToast, useUI } from '../ui';

const t = strings.imports;
type Summary = { results: { row: number; name: string; action: string; errors?: Record<string, string> }[]; created: number; updated: number; failed: number };
type Pending = { kind: ImportKind; label: string; rows: Record<string, unknown>[]; ignored: string[]; summary: Summary };

/**
 * Owner's CSV imports (customers, catalog, products): pick the file, see what would be created,
 * updated or fails (row by row), then import the valid rows in one go.
 */
export function ImportData({ businessId }: { businessId: string }) {
  const ui = useUI();
  const toast = useToast();
  const { business } = useBusiness(businessId);
  const { has } = useTexts();
  const [busy, setBusy] = useState(false);
  const [pending, setPending] = useState<Pending | null>(null);
  if (!business || Platform.OS !== 'web') return null; // file picking is web-only for now (the PWA)

  const kinds: { kind: ImportKind; label: string; icon: 'people' | 'construct' | 'cube' }[] = [
    { kind: 'customers', label: strings.exports.kinds.customers, icon: 'people' },
    { kind: 'catalog', label: term(business.vertical, 'catalog') ?? strings.business.sections.catalog, icon: 'construct' },
    ...(has('productSales') ? [{ kind: 'products' as const, label: strings.exports.kinds.products, icon: 'cube' as const }] : []),
  ];

  const send = (kind: ImportKind, rows: Record<string, unknown>[], apply: boolean) =>
    api.POST('/v1/b/{businessId}/import/{kind}', { params: { path: { businessId, kind }, query: apply ? { apply: true } : {} }, body: { rows } });

  async function start(kind: ImportKind, label: string) {
    const text = await pickTextFile();
    if (text === null) return;
    const parsed = rowsFromCSV(kind, text);
    if (parsed.missing.length) return toast(t.missing(parsed.missing.join(', ')), 'error');
    if (!parsed.rows.length) return toast(t.noRows, 'error');
    setBusy(true);
    const { data, error, response } = await send(kind, parsed.rows, false);
    setBusy(false);
    if (!data) return toast(errorText(error, response.status), 'error');
    setPending({ kind, label, rows: parsed.rows, ignored: parsed.ignored, summary: data });
  }

  async function apply() {
    if (!pending) return;
    setBusy(true);
    const { data, error, response } = await send(pending.kind, pending.rows, true);
    setBusy(false);
    if (!data) return toast(errorText(error, response.status), 'error');
    setPending(null);
    toast(t.done(data.created + data.updated));
  }

  const s = pending?.summary;
  const failures = s?.results.filter((r) => r.action === 'error') ?? [];
  const valid = s ? s.created + s.updated : 0;

  return (
    <>
      <Group label={t.title} footer={t.hint}>
        {kinds.map((k) => (
          <Cell key={k.kind} icon={k.icon} tint={ui.accent} label={t.importOf(k.label)} value={busy ? '…' : 'CSV'}
            onPress={busy ? undefined : () => start(k.kind, k.label)} />
        ))}
      </Group>
      <Group label={t.templates}>
        {kinds.map((k) => (
          <Cell key={k.kind} icon="document-text" tint={ui.gray} label={t.template(k.label)}
            onPress={() => downloadText(`plantilla-${k.kind}.csv`, importTemplate(k.kind))} />
        ))}
      </Group>

      <Modal visible={!!pending} transparent animationType="fade" onRequestClose={() => setPending(null)}>
        <Pressable accessibilityLabel={strings.photos.close} onPress={() => setPending(null)}
          style={{ flex: 1, backgroundColor: 'rgba(0,0,0,0.4)', justifyContent: 'center', alignItems: 'center', padding: 16 }}>
          {/* Stop taps inside the panel from closing it. */}
          <Pressable onPress={() => {}} style={{ width: '100%', maxWidth: 480, maxHeight: '90%', backgroundColor: ui.surface, borderRadius: radius.lg }}>
            {pending && s && (
              <ScrollView contentContainerStyle={{ padding: 20, gap: 10 }}>
                <Text variant="title" accessibilityRole="header">{t.previewTitle(pending.label)}</Text>
                {s.created > 0 && <Text style={{ color: ui.success }}>{t.created(s.created)}</Text>}
                {s.updated > 0 && <Text style={{ color: ui.accent }}>{t.updated(s.updated)}</Text>}
                {s.failed > 0 && <Text style={{ color: ui.danger }}>{t.failed(s.failed)}</Text>}
                {pending.ignored.length > 0 && <Text variant="footnote" color="labelSecondary">{t.ignored(pending.ignored.join(', '))}</Text>}
                {failures.slice(0, 30).map((r) => (
                  <View key={r.row} style={{ backgroundColor: ui.fill, borderRadius: radius.sm, padding: 10 }}>
                    <Text variant="subhead" style={{ fontWeight: '600' }}>{`${t.row(r.row)}${r.name ? ` · ${r.name}` : ''}`}</Text>
                    {Object.values(r.errors ?? {}).map((m) => <Text key={m} variant="footnote" color="labelSecondary">{m}</Text>)}
                  </View>
                ))}
                {failures.length > 30 && <Text variant="footnote" color="labelSecondary">{t.more(failures.length - 30)}</Text>}
                {valid === 0 && <Text color="labelSecondary">{t.nothing}</Text>}
                <View style={{ gap: 8, marginTop: 6 }}>
                  {valid > 0 && <Button variant="primary" busy={busy} onPress={apply}>{t.apply(valid)}</Button>}
                  <Button variant="plain" disabled={busy} onPress={() => setPending(null)}>{strings.confirm.cancel}</Button>
                </View>
              </ScrollView>
            )}
          </Pressable>
        </Pressable>
      </Modal>
    </>
  );
}
