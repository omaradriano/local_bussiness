import type { components } from '@lb/api-client';
import { centavosToInput, formatMXN, parsePesos, strings, todayISO } from '@lb/shared';
import { router, Stack, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Switch, View } from 'react-native';
import { ConfirmDialog } from '../../../../../components/ConfirmDialog';
import { api } from '../../../../../lib/api';
import { errorText, fieldErrors } from '../../../../../lib/errors';
import { Button, Cell, Field, Group, Screen, Segmented, Text, useToast } from '../../../../../ui';

type Product = components['schemas']['RetailProduct'];
type Movement = components['schemas']['Movement'];
const t = strings.products;
const ISO = /^\d{4}-\d{2}-\d{2}$/;

/** A product: name, price, cost (owner), stock to start with; and merchandise purchases afterwards. */
export default function ProductForm() {
  const { businessId, productId } = useLocalSearchParams<{ businessId: string; productId: string }>();
  const toast = useToast();
  const isNew = productId === 'new';
  const [product, setProduct] = useState<Product | null>(null);
  const [f, setF] = useState({ name: '', price: '', cost: '', stock: '0', minStock: '3', active: true });
  const [removing, setRemoving] = useState(false);
  const [buy, setBuy] = useState({ quantity: '', unitCost: '', on: todayISO() });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [buyErrors, setBuyErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState<'save' | 'buy' | null>(null);
  const [moves, setMoves] = useState<Movement[] | null>(null);
  const [adj, setAdj] = useState<{ reason: 'waste' | 'count'; amount: string; note: string }>({ reason: 'waste', amount: '', note: '' });
  const [adjErrors, setAdjErrors] = useState<Record<string, string>>({});
  const [adjusting, setAdjusting] = useState(false);
  const loadMoves = useCallback(() => {
    if (isNew) return;
    api.GET('/v1/b/{businessId}/products/{productId}/movements', { params: { path: { businessId, productId } } })
      .then(({ data }) => setMoves(data?.movements ?? []));
  }, [businessId, productId, isNew]);
  useEffect(loadMoves, [loadMoves]);

  useEffect(() => {
    if (isNew) return;
    api.GET('/v1/b/{businessId}/products', { params: { path: { businessId } } }).then(({ data }) => {
      const p = data?.products.find((x) => x.id === productId);
      if (!p) return;
      setProduct(p);
      setF({ name: p.name, price: centavosToInput(p.price), cost: p.cost !== null ? centavosToInput(p.cost) : '', stock: String(p.stock), minStock: String(p.min_stock), active: p.active });
      setBuy((b) => ({ ...b, unitCost: p.cost ? centavosToInput(p.cost) : '' }));
    });
  }, [businessId, productId, isNew]);

  if (!isNew && !product) return <ActivityIndicator style={{ marginTop: 24 }} />;

  async function save() {
    const price = parsePesos(f.price);
    const cost = f.cost.trim() ? parsePesos(f.cost) : 0;
    const local: Record<string, string> = {};
    if (price === null) local.price = strings.catalog.invalidMoney;
    if (cost === null) local.cost = strings.catalog.invalidMoney;
    if (isNew && !/^\d+$/.test(f.stock.trim())) local.stock = strings.catalog.invalidNumber;
    if (!/^\d+$/.test(f.minStock.trim())) local.min_stock = strings.catalog.invalidNumber;
    if (Object.keys(local).length) return setErrors(local);
    setBusy('save');
    const body = { name: f.name, price: price!, cost: cost!, active: f.active, min_stock: Number(f.minStock), ...(isNew ? { stock: Number(f.stock) } : {}) };
    const res = isNew
      ? await api.POST('/v1/b/{businessId}/products', { params: { path: { businessId } }, body })
      : await api.PATCH('/v1/b/{businessId}/products/{productId}', { params: { path: { businessId, productId } }, body });
    setBusy(null);
    if (!res.data) { setErrors(fieldErrors(res.error)); return toast(errorText(res.error, res.response.status), 'error'); }
    toast(t.saved);
    if (isNew) return router.replace({ pathname: '/admin/[businessId]/products/[productId]', params: { businessId, productId: res.data.id } });
    setErrors({});
    setProduct(res.data);
  }

  async function purchase() {
    const unitCost = parsePesos(buy.unitCost || '0');
    const local: Record<string, string> = {};
    if (!/^\d+$/.test(buy.quantity.trim()) || Number(buy.quantity) < 1) local.quantity = strings.catalog.invalidNumber;
    if (unitCost === null) local.unit_cost = strings.catalog.invalidMoney;
    if (!ISO.test(buy.on)) local.purchased_on = strings.finances.invalidDate;
    if (Object.keys(local).length) return setBuyErrors(local);
    setBusy('buy');
    const { data, error, response } = await api.POST('/v1/b/{businessId}/products/{productId}/purchases', {
      params: { path: { businessId, productId } }, body: { quantity: Number(buy.quantity), unit_cost: unitCost!, purchased_on: buy.on },
    });
    setBusy(null);
    if (!data) { setBuyErrors(fieldErrors(error)); return toast(errorText(error, response.status), 'error'); }
    setBuyErrors({});
    setBuy({ ...buy, quantity: '' });
    setProduct(data);
    setF((x) => ({ ...x, cost: data.cost !== null ? centavosToInput(data.cost) : x.cost, stock: String(data.stock) }));
    toast(t.purchased);
    loadMoves();
  }

  // Waste takes pieces out; a count leaves the stock at what was counted.
  async function adjust() {
    const n = adj.amount.trim();
    const key = adj.reason === 'waste' ? 'quantity' : 'counted';
    if (!/^\d+$/.test(n)) return setAdjErrors({ [key]: strings.catalog.invalidNumber });
    setAdjusting(true);
    const { data, error, response } = await api.POST('/v1/b/{businessId}/products/{productId}/adjustments', {
      params: { path: { businessId, productId } }, body: { reason: adj.reason, [key]: Number(n), note: adj.note },
    });
    setAdjusting(false);
    if (!data) { setAdjErrors(fieldErrors(error)); return toast(errorText(error, response.status), 'error'); }
    setAdjErrors({});
    setAdj({ ...adj, amount: '', note: '' });
    setProduct(data);
    toast(t.adjusted);
    loadMoves();
  }

  async function remove() {
    const { data, error, response } = await api.DELETE('/v1/b/{businessId}/products/{productId}', { params: { path: { businessId, productId } } });
    setRemoving(false);
    if (!data) return toast(errorText(error, response.status), 'error');
    toast(data.archived ? t.archived : t.deleted);
    router.back();
  }

  const title = isNew ? t.add : product!.name;
  return (
    <Screen>
      <Stack.Screen options={{ title }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 20 }}>{title}</Text>
      <Group padded>
        <Field label={t.name} value={f.name} onChangeText={(name) => setF({ ...f, name })} error={errors.name} />
        <Field label={t.price} value={f.price} onChangeText={(price) => setF({ ...f, price })} keyboardType="decimal-pad" error={errors.price} />
        <Field label={t.cost} value={f.cost} onChangeText={(cost) => setF({ ...f, cost })} keyboardType="decimal-pad" error={errors.cost}
          hint={(parsePesos(f.cost) ?? 0) > (parsePesos(f.price) ?? Infinity) ? t.costAbovePrice : t.costHint} />
        {isNew
          ? <Field label={t.initialStock} value={f.stock} onChangeText={(stock) => setF({ ...f, stock })} keyboardType="numeric" hint={t.stockHint} error={errors.stock} />
          : <Text variant="subhead" color="labelSecondary" style={{ marginBottom: 12 }}>{t.stockLeft(product!.stock)} · {formatMXN(product!.price)}</Text>}
        <Field label={t.minStock} value={f.minStock} onChangeText={(minStock) => setF({ ...f, minStock })} keyboardType="numeric" hint={t.minStockHint} error={errors.min_stock} />
      </Group>
      {!isNew && (
        <Group>
          <Cell label={t.active} accessory={<Switch accessibilityLabel={t.active} value={f.active} onValueChange={(active) => setF({ ...f, active })} />} />
        </Group>
      )}
      <Button variant="primary" busy={busy === 'save'} onPress={save} style={{ marginBottom: 28 }}>{t.save}</Button>
      {!isNew && (
        <Group label={t.purchase} padded footer={t.purchaseHint}>
          <View style={{ flexDirection: 'row', gap: 12 }}>
            <View style={{ flex: 1 }}><Field label={t.quantity} value={buy.quantity} onChangeText={(quantity) => setBuy({ ...buy, quantity })} keyboardType="numeric" error={buyErrors.quantity} /></View>
            <View style={{ flex: 1 }}><Field label={t.unitCost} value={buy.unitCost} onChangeText={(unitCost) => setBuy({ ...buy, unitCost })} keyboardType="decimal-pad" error={buyErrors.unit_cost} /></View>
          </View>
          <Field label={t.purchasedOn} value={buy.on} onChangeText={(on) => setBuy({ ...buy, on })} error={buyErrors.purchased_on} />
          <Button icon="cart" busy={busy === 'buy'} onPress={purchase}>{t.purchase}</Button>
        </Group>
      )}
      {!isNew && (
        <Group label={t.adjust} padded footer={adj.reason === 'waste' ? t.wasteHint : t.countHint}>
          <Segmented options={(['waste', 'count'] as const).map((r) => ({ value: r, label: t.adjustReasons[r] }))} value={adj.reason}
            onChange={(reason) => { setAdj({ ...adj, reason }); setAdjErrors({}); }} />
          <Field label={adj.reason === 'waste' ? t.wasteQuantity : t.counted} value={adj.amount} onChangeText={(amount) => setAdj({ ...adj, amount })}
            keyboardType="numeric" error={adjErrors.quantity ?? adjErrors.counted} />
          <Field label={t.adjustNote} value={adj.note} onChangeText={(note) => setAdj({ ...adj, note })} maxLength={200} error={adjErrors.note} />
          <Button icon="create" busy={adjusting} disabled={!adj.amount.trim()} onPress={adjust}>{t.adjustSave}</Button>
        </Group>
      )}
      {!isNew && moves && (
        <Group label={t.history}>
          {moves.length === 0 ? <Cell label={t.noHistory} /> : moves.map((m, i) => (
            <Cell key={i} label={t.moveLabel[m.kind](m.quantity)} sublabel={`${new Date(m.at).toLocaleDateString('es-MX', { day: 'numeric', month: 'short', year: 'numeric' })} · ${t.moveAmount[m.kind](formatMXN(m.amount))}${m.note ? ` · ${m.note}` : ''}`} />
          ))}
        </Group>
      )}
      {!isNew && <Button variant="destructive" icon="trash" onPress={() => setRemoving(true)} style={{ marginTop: 12 }}>{t.remove}</Button>}
      <ConfirmDialog open={removing} title={t.removeTitle} body={t.removeBody} confirmLabel={t.remove} onConfirm={remove} onClose={() => setRemoving(false)} />
    </Screen>
  );
}
