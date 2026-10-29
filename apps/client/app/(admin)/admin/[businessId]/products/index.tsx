import type { components } from '@lb/api-client';
import { formatMXN, strings, todayISO } from '@lb/shared';
import { Ionicons } from '@expo/vector-icons';
import { router, Stack, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, View } from 'react-native';
import { Donut } from '../../../../../components/Donut';
import { currentMonth, PeriodPicker, type Period } from '../../../../../components/PeriodPicker';
import { api } from '../../../../../lib/api';
import { errorText } from '../../../../../lib/errors';
import { useBusiness } from '../../../../../lib/useMe';
import { Button, Cell, EmptyState, Group, Notice, Row, Screen, Segmented, Text, radius, useToast, useUI } from '../../../../../ui';

type Product = components['schemas']['RetailProduct'];
type Sale = components['schemas']['Sale'];
type Stats = components['schemas']['ProductStats'];
type Method = Sale['method'];
type Tab = 'sell' | 'inventory' | 'stats';
const t = strings.products;
const hour = (iso: string) => new Date(iso).toLocaleTimeString('es-MX', { hour: '2-digit', minute: '2-digit' });

/** Counter sale: tap products to add them, pick how it's paid, register. Today's sales below. */
function Sell({ businessId, isOwner, products, reload }: { businessId: string; isOwner: boolean; products: Product[]; reload: () => void }) {
  const ui = useUI();
  const toast = useToast();
  const [cart, setCart] = useState<Record<string, number>>({});
  const [method, setMethod] = useState<Method>('cash');
  const [busy, setBusy] = useState(false);
  const [sales, setSales] = useState<Sale[] | null>(null);
  const today = todayISO();

  const loadSales = useCallback(() => {
    api.GET('/v1/b/{businessId}/sales', { params: { path: { businessId }, query: { from: today, to: today } } }).then(({ data }) => setSales(data?.sales ?? []));
  }, [businessId, today]);
  useFocusEffect(loadSales);

  const active = products.filter((p) => p.active);
  const lines = Object.entries(cart).filter(([, q]) => q > 0);
  const total = lines.reduce((n, [id, q]) => n + (products.find((p) => p.id === id)?.price ?? 0) * q, 0);
  const change = (id: string, delta: number, max: number) => setCart((c) => ({ ...c, [id]: Math.min(max, Math.max(0, (c[id] ?? 0) + delta)) }));

  async function sell() {
    setBusy(true);
    const { data, error, response } = await api.POST('/v1/b/{businessId}/sales', {
      params: { path: { businessId } }, body: { lines: lines.map(([product_id, quantity]) => ({ product_id, quantity })), method },
    });
    setBusy(false);
    if (!data) return toast(errorText(error, response.status), 'error');
    toast(t.sold);
    setCart({});
    reload();
    loadSales();
  }

  async function cancel(id: string) {
    const { error, response } = await api.DELETE('/v1/b/{businessId}/sales/{saleId}', { params: { path: { businessId, saleId: id } } });
    if (!response.ok) return toast(errorText(error, response.status), 'error');
    toast(t.saleCancelled);
    reload();
    loadSales();
  }

  if (!active.length) return <EmptyState icon="pricetags-outline" title={t.empty} />;
  return (
    <View style={{ gap: 20 }}>
      <Text variant="subhead" color="labelSecondary">{t.pickProducts}</Text>
      <View style={{ backgroundColor: ui.surface, borderRadius: radius.sm, overflow: 'hidden' }}>
        {active.map((p, i) => {
          const q = cart[p.id] ?? 0;
          return (
            <View key={p.id} style={{ flexDirection: 'row', alignItems: 'center', gap: 10, padding: 14, borderTopWidth: i ? StyleSheet.hairlineWidth : 0, borderColor: ui.separator, opacity: p.stock ? 1 : 0.5 }}>
              <Pressable accessibilityRole="button" accessibilityLabel={`${p.name}, agregar`} disabled={!p.stock} onPress={() => change(p.id, 1, p.stock)} style={{ flex: 1 }}>
                <Text>{p.name}</Text>
                <Text variant="footnote" color="labelSecondary">{formatMXN(p.price)} · {p.stock ? t.stockLeft(p.stock) : t.noStock}</Text>
              </Pressable>
              {q > 0 && (
                <View style={{ flexDirection: 'row', alignItems: 'center', gap: 10 }}>
                  <Pressable accessibilityRole="button" accessibilityLabel={`Quitar uno de ${p.name}`} hitSlop={10} onPress={() => change(p.id, -1, p.stock)}>
                    <Ionicons name="remove-circle" size={26} color={ui.accent} />
                  </Pressable>
                  <Text variant="headline" style={{ minWidth: 22, textAlign: 'center', fontVariant: ['tabular-nums'] }}>{q}</Text>
                  <Pressable accessibilityRole="button" accessibilityLabel={`Agregar uno de ${p.name}`} hitSlop={10} disabled={q >= p.stock} onPress={() => change(p.id, 1, p.stock)}>
                    <Ionicons name="add-circle" size={26} color={q >= p.stock ? ui.labelSecondary : ui.accent} />
                  </Pressable>
                </View>
              )}
            </View>
          );
        })}
      </View>
      {lines.length > 0 && (
        <View style={{ backgroundColor: ui.surface, borderRadius: radius.md, padding: 16, gap: 12 }}>
          <Text variant="headline">{t.cart}</Text>
          <Segmented options={(['cash', 'transfer', 'card'] as Method[]).map((m) => ({ value: m, label: strings.bookings.methods[m] }))} value={method} onChange={setMethod} />
          <Row label={t.total} value={formatMXN(total)} strong />
          <Button variant="primary" icon="checkmark-circle" busy={busy} onPress={sell}>{t.sell}</Button>
        </View>
      )}
      <Group label={t.todaySales}>
        {sales?.length === 0 ? <Cell label={t.noSales} /> : null}
        {(sales ?? []).map((s) => (
          <Cell key={s.id} label={s.lines.map((l) => `${l.quantity} × ${l.name}`).join(', ')} sublabel={`${hour(s.sold_at)} · ${strings.bookings.methods[s.method]}`}
            value={formatMXN(s.total)}
            accessory={isOwner ? (
              <Pressable accessibilityRole="button" accessibilityLabel={t.cancelSale} hitSlop={10} onPress={() => cancel(s.id)}>
                <Ionicons name="trash-outline" size={18} color={ui.danger} />
              </Pressable>
            ) : undefined} />
        ))}
      </Group>
    </View>
  );
}

function Inventory({ businessId, isOwner, products }: { businessId: string; isOwner: boolean; products: Product[] }) {
  const ui = useUI();
  const open = (productId: string) => router.push({ pathname: '/admin/[businessId]/products/[productId]', params: { businessId, productId } });
  return (
    <View style={{ gap: 16 }}>
      {isOwner && <Button variant="primary" icon="add-circle" onPress={() => open('new')}>{t.add}</Button>}
      {products.length === 0 ? <EmptyState icon="pricetags-outline" title={t.empty} /> : (
        <Group separatorInset={58}>
          {products.map((p) => (
            <Cell key={p.id} icon="pricetag" tint={!p.active ? ui.gray : p.stock <= 3 ? ui.warning : ui.success} label={p.name}
              sublabel={`${p.stock ? t.stockLeft(p.stock) : t.noStock}${p.cost !== null ? ` · ${strings.products.cost.split(' (')[0]} ${formatMXN(p.cost)}` : ''}`}
              value={formatMXN(p.price)} onPress={isOwner ? () => open(p.id) : undefined} />
          ))}
        </Group>
      )}
    </View>
  );
}

function StatsView({ businessId }: { businessId: string }) {
  const ui = useUI();
  const [period, setPeriod] = useState<Period>(currentMonth);
  const [st, setSt] = useState<Stats | null>(null);
  const [error, setError] = useState<string | null>(null);
  useFocusEffect(useCallback(() => {
    setSt(null);
    api.GET('/v1/b/{businessId}/products/stats', { params: { path: { businessId }, query: { from: period.from, to: period.to } } })
      .then(({ data, error: err, response }) => (data ? setSt(data) : setError(errorText(err, response.status))));
  }, [businessId, period]));

  return (
    <View style={{ gap: 20 }}>
      <PeriodPicker value={period} onChange={setPeriod} />
      {error && <Notice tone="warning">{error}</Notice>}
      {!st && !error && <ActivityIndicator />}
      {st && (
        <>
          <View style={{ backgroundColor: ui.surface, borderRadius: radius.md, padding: 16, gap: 10 }}>
            <Text variant="subhead" color="labelSecondary">{t.profit}</Text>
            <Text variant="largeTitle" style={{ color: st.profit >= 0 ? ui.success : ui.danger, fontVariant: ['tabular-nums'] }}>{formatMXN(st.profit)}</Text>
            <Row label={t.revenue} value={formatMXN(st.revenue)} />
            <Row label={t.costOfSold} value={formatMXN(st.cost)} />
            <Text variant="footnote" color="labelSecondary">{`${t.salesCount(st.sales)} · ${t.units(st.units)}`}</Text>
          </View>
          <View style={{ backgroundColor: ui.surface, borderRadius: radius.md, padding: 16 }}>
            <Donut title={t.byProduct} total={st.revenue} empty={t.noSalesPeriod} colorKeys={st.by_product.map((p) => p.product_id)}
              slices={st.by_product.map((p) => ({ key: p.product_id, label: p.name, amount: p.revenue }))} />
          </View>
          {st.by_product.length > 0 && (
            <Group label={t.bestSellers}>
              {st.by_product.map((p) => (
                <Cell key={p.product_id} label={p.name} sublabel={`${t.units(p.units)} · ${t.profit} ${formatMXN(p.profit)}`} value={formatMXN(p.revenue)} />
              ))}
            </Group>
          )}
          {st.low_stock.length > 0 && (
            <Group label={t.lowStock}>
              {st.low_stock.map((p) => <Cell key={p.id} icon="alert-circle" tint={ui.warning} label={p.name} value={p.stock ? t.stockLeft(p.stock) : t.noStock} />)}
            </Group>
          )}
          <Group>
            <Cell label={t.purchasesInPeriod} value={formatMXN(st.purchases)} />
            <Cell label={t.stockValue} value={formatMXN(st.stock_value)} />
          </Group>
        </>
      )}
    </View>
  );
}

/** Products the business sells besides its service: sell at the counter, inventory and stats. */
export default function Products() {
  const { businessId } = useLocalSearchParams<{ businessId: string }>();
  const { business } = useBusiness(businessId);
  const [tab, setTab] = useState<Tab>('sell');
  const [products, setProducts] = useState<Product[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    api.GET('/v1/b/{businessId}/products', { params: { path: { businessId } } })
      .then(({ data, error: err, response }) => (data ? setProducts(data.products) : setError(errorText(err, response.status))));
  }, [businessId]);
  useFocusEffect(load);

  const isOwner = business?.role === 'owner';
  const tabs: Tab[] = isOwner ? ['sell', 'inventory', 'stats'] : ['sell', 'inventory'];
  return (
    <Screen>
      <Stack.Screen options={{ title: t.title }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 16 }}>{t.title}</Text>
      <View style={{ marginBottom: 20 }}>
        <Segmented options={tabs.map((k) => ({ value: k, label: t.tabs[k] }))} value={tab} onChange={setTab} />
      </View>
      {error && <Notice tone="warning">{error}</Notice>}
      {!products && !error && <ActivityIndicator />}
      {products && tab === 'sell' && <Sell businessId={businessId} isOwner={isOwner} products={products} reload={load} />}
      {products && tab === 'inventory' && <Inventory businessId={businessId} isOwner={isOwner} products={products} />}
      {tab === 'stats' && isOwner && <StatsView businessId={businessId} />}
    </Screen>
  );
}
