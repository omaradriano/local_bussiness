import { formatPhone, strings } from '@lb/shared';
import { useEffect, useState } from 'react';
import { Button, Pressable, Text, TextInput, View } from 'react-native';
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
  const [q, setQ] = useState('');
  const [results, setResults] = useState<Customer[]>([]);
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    if (value || creating || !q.trim()) return setResults([]);
    const id = setTimeout(async () => {
      const { data } = await api.GET('/v1/b/{businessId}/customers', { params: { path: { businessId }, query: { q, limit: 8 } } });
      setResults(data?.customers ?? []);
    }, 250);
    return () => clearTimeout(id);
  }, [businessId, q, value, creating]);

  if (value) {
    const missing = missingDocs(value);
    return (
      <View style={{ borderWidth: 1, borderRadius: 8, padding: 10, gap: 4 }}>
        <Text style={{ fontWeight: '600' }}>{value.name}{value.trusted ? ` · ${t.trustedBadge}` : ''}</Text>
        <Text>{formatPhone(value.phone)}</Text>
        {value.address && <Text style={{ opacity: 0.7 }}>{value.address}</Text>}
        {missing.length > 0 && <Text>{t.missingDocs(missing.join(', '))}</Text>}
        <Pressable accessibilityRole="button" onPress={() => { onChange(null); setQ(''); }}>
          <Text style={{ textDecorationLine: 'underline' }}>{t.change}</Text>
        </Pressable>
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
        <Button title={strings.common.cancel} onPress={() => setCreating(false)} />
      </View>
    );
  }

  return (
    <View style={{ gap: 6 }}>
      <TextInput accessibilityLabel={t.search} placeholder={t.search} value={q} onChangeText={setQ}
        style={{ borderWidth: 1, borderRadius: 6, padding: 10 }} />
      {results.map((c) => (
        <Pressable key={c.id} accessibilityRole="button" onPress={() => onChange(c)} style={{ borderWidth: 1, borderRadius: 6, padding: 8 }}>
          <Text>{c.name} · {formatPhone(c.phone)}</Text>
        </Pressable>
      ))}
      <Pressable accessibilityRole="button" onPress={() => setCreating(true)}>
        <Text style={{ textDecorationLine: 'underline' }}>+ {t.new}</Text>
      </Pressable>
      {error && <Text accessibilityRole="alert">{error}</Text>}
    </View>
  );
}
