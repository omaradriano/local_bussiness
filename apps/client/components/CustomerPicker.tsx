import { Ionicons } from '@expo/vector-icons';
import { formatPhone, strings } from '@lb/shared';
import { useEffect, useState } from 'react';
import { ScrollView, StyleSheet, View } from 'react-native';
import { Button, Cell, Field, Notice, Text, radius, useUI } from '../ui';
import { api } from '../lib/api';
import { CustomerForm, type Customer } from './CustomerForm';

const t = strings.customers;

/** Missing documents of a customer (none if trusted). */
export function missingDocs(c: Customer): string[] {
  if (c.trusted) return [];
  return (['ine', 'proof_of_address'] as const).filter((d) => !c.documents.some((x) => x.doc_type === d)).map((d) => t.docTypes[d]);
}

/** Pick a saved customer by name/phone, or register a new one inline. */
export function CustomerPicker({ businessId, isOwner, value, onChange, error }: {
  businessId: string; isOwner: boolean; value: Customer | null; onChange: (c: Customer | null) => void; error?: string;
}) {
  const ui = useUI();
  const [q, setQ] = useState('');
  const [results, setResults] = useState<Customer[]>([]);
  const [creating, setCreating] = useState(false);
  const [all, setAll] = useState<Customer[] | null>(null); // the "select" of registered customers, loaded when opened
  const [listOpen, setListOpen] = useState(false);

  async function toggleList() {
    if (!listOpen && !all) {
      const { data } = await api.GET('/v1/b/{businessId}/customers', { params: { path: { businessId }, query: { limit: 100 } } });
      setAll((data?.customers ?? []).sort((a, b) => a.name.localeCompare(b.name, 'es')));
    }
    setListOpen(!listOpen);
  }

  useEffect(() => {
    if (value || creating || !q.trim()) return setResults([]);
    const id = setTimeout(async () => {
      const { data } = await api.GET('/v1/b/{businessId}/customers', { params: { path: { businessId }, query: { q, limit: 8 } } });
      setResults(data?.customers ?? []);
    }, 250);
    return () => clearTimeout(id);
  }, [businessId, q, value, creating]);

  // Lists inside a step sit on a grey fill (the step itself is the white surface).
  const list = (cs: Customer[]) => (
    <View style={{ backgroundColor: ui.fill, borderRadius: radius.sm, overflow: 'hidden' }}>
      {cs.map((c, i) => (
        <View key={c.id}>
          {i > 0 && <View style={{ height: StyleSheet.hairlineWidth, backgroundColor: ui.separator, marginLeft: 16 }} />}
          <Cell label={c.name} sublabel={formatPhone(c.phone)} onPress={() => { setListOpen(false); onChange(c); }} />
        </View>
      ))}
    </View>
  );

  if (value) {
    const missing = missingDocs(value);
    return (
      <View style={{ backgroundColor: ui.fill, borderRadius: radius.sm, padding: 14, gap: 4 }}>
        <View style={{ flexDirection: 'row', alignItems: 'center', gap: 8 }}>
          <Ionicons name={value.trusted ? 'shield-checkmark' : 'person-circle'} size={22} color={value.trusted ? ui.success : ui.accent} />
          <Text variant="headline" style={{ flex: 1 }}>{value.name}</Text>
          <Button size="sm" variant="plain" onPress={() => { onChange(null); setQ(''); }}>{t.change}</Button>
        </View>
        <Text variant="subhead" color="labelSecondary">{formatPhone(value.phone)}{value.trusted ? ` · ${t.trustedBadge}` : ''}</Text>
        {value.address && <Text variant="subhead" color="labelSecondary">{value.address}</Text>}
        {missing.length > 0 && <View style={{ marginTop: 6 }}><Notice tone="warning">{t.missingDocs(missing.join(', '))}</Notice></View>}
      </View>
    );
  }

  if (creating) {
    return (
      <View style={{ gap: 8 }}>
        <CustomerForm businessId={businessId} isOwner={isOwner} onSaved={(c) => { setCreating(false); onChange(c); }}
          onExisting={async (id) => {
            const { data } = await api.GET('/v1/b/{businessId}/customers/{customerId}/', { params: { path: { businessId, customerId: id } } });
            if (data) { setCreating(false); onChange(data); }
          }} />
        <Button variant="plain" onPress={() => setCreating(false)}>{strings.common.cancel}</Button>
      </View>
    );
  }

  return (
    <View style={{ gap: 10 }}>
      <Field label={t.search} placeholder={t.searchPlaceholder} value={q} onChangeText={setQ} autoCorrect={false} error={error} />
      {results.length > 0 && list(results)}
      <Button icon={listOpen ? 'chevron-up' : 'chevron-down'} onPress={toggleList}>{t.registered}</Button>
      {listOpen && (all?.length === 0 ? <Text color="labelSecondary">{t.empty}</Text> : all && (
        <ScrollView style={{ maxHeight: 300 }} nestedScrollEnabled>{list(all)}</ScrollView>
      ))}
      <Button variant="plain" icon="person-add" onPress={() => setCreating(true)}>{t.new}</Button>
    </View>
  );
}
