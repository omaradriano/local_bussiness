import type { components } from '@lb/api-client';
import { centavosToInput, formatMXN, parsePesos, strings, todayISO } from '@lb/shared';
import { router, Stack, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, Switch, View } from 'react-native';
import { api } from '../../../../../lib/api';
import { errorText, fieldErrors } from '../../../../../lib/errors';
import { Button, Cell, Field, Group, Screen, Text, useToast } from '../../../../../ui';

type Product = components['schemas']['RetailProduct'];
const t = strings.products;
const ISO = /^\d{4}-\d{2}-\d{2}$/;

/** A product: name, price, cost (owner), stock to start with; and merchandise purchases afterwards. */
export default function ProductForm() {
  const { businessId, productId } = useLocalSearchParams<{ businessId: string; productId: string }>();
  const toast = useToast();
  const isNew = productId === 'new';
  const [product, setProduct] = useState<Product | null>(null);
  const [f, setF] = useState({ name: '', price: '', cost: '', stock: '0', active: true });
  const [buy, setBuy] = useState({ quantity: '', unitCost: '', on: todayISO() });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [buyErrors, setBuyErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState<'save' | 'buy' | null>(null);

  useEffect(() => {
    if (isNew) return;
    api.GET('/v1/b/{businessId}/products', { params: { path: { businessId } } }).then(({ data }) => {
      const p = data?.products.find((x) => x.id === productId);
      if (!p) return;
      setProduct(p);
      setF({ name: p.name, price: centavosToInput(p.price), cost: p.cost !== null ? centavosToInput(p.cost) : '', stock: String(p.stock), active: p.active });
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
    if (Object.keys(local).length) return setErrors(local);
    setBusy('save');
    const body = { name: f.name, price: price!, cost: cost!, active: f.active, ...(isNew ? { stock: Number(f.stock) } : {}) };
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
  }

  const title = isNew ? t.add : product!.name;
  return (
    <Screen>
      <Stack.Screen options={{ title }} />
      <Text variant="largeTitle" accessibilityRole="header" style={{ marginBottom: 20 }}>{title}</Text>
      <Group padded>
        <Field label={t.name} value={f.name} onChangeText={(name) => setF({ ...f, name })} error={errors.name} />
        <Field label={t.price} value={f.price} onChangeText={(price) => setF({ ...f, price })} keyboardType="decimal-pad" error={errors.price} />
        <Field label={t.cost} value={f.cost} onChangeText={(cost) => setF({ ...f, cost })} keyboardType="decimal-pad" hint={t.costHint} error={errors.cost} />
        {isNew
          ? <Field label={t.initialStock} value={f.stock} onChangeText={(stock) => setF({ ...f, stock })} keyboardType="numeric" hint={t.stockHint} error={errors.stock} />
          : <Text variant="subhead" color="labelSecondary" style={{ marginBottom: 12 }}>{t.stockLeft(product!.stock)} · {formatMXN(product!.price)}</Text>}
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
    </Screen>
  );
}
