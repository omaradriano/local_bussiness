import type { components } from '@lb/api-client';
import { addToCart, cartTotal, changeQuantity, discountInput, discountOff, formatMXN, parsePesos, saleLines, strings, todayISO, type CartLine, type DiscountMode } from '@lb/shared';
import { Ionicons } from '@expo/vector-icons';
import { router, Stack, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, View } from 'react-native';
import { ConfirmDialog } from '../../../../../components/ConfirmDialog';
import { DiscountFields } from '../../../../../components/DiscountForm';
import { api } from '../../../../../lib/api';
import { errorText, fieldErrors } from '../../../../../lib/errors';
import { useBusiness } from '../../../../../lib/useMe';
import { useTexts } from '../../../../../lib/useTexts';
import { Button, Cell, Chip, Chips, Field, Group, Notice, Row, Screen, Segmented, Step, Text, radius, useToast, useUI, type StepState } from '../../../../../ui';

type Item = components['schemas']['CatalogItem'];
type Product = components['schemas']['RetailProduct'];
type Sale = components['schemas']['Sale'];
type Method = Sale['method'];
const t = strings.quickSale;
const hour = (iso: string) => new Date(iso).toLocaleTimeString('es-MX', { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' });

/** Quick sale (feature quickSale): what the customer takes now, paid now; no date, no customer profile. */
export default function QuickSale() {
  // booking: added to an order from its detail (the sale is linked to it and we go back there).
  const { businessId, booking, who } = useLocalSearchParams<{ businessId: string; booking?: string; who?: string }>();
  const { business } = useBusiness(businessId);
  const { has } = useTexts();
  const ui = useUI();
  const toast = useToast();
  const [items, setItems] = useState<Item[] | null>(null);
  const [products, setProducts] = useState<Product[]>([]);
  const [cart, setCart] = useState<CartLine[]>([]);
  const [custom, setCustom] = useState<Item | null>(null); // free-price item being described
  const [freeName, setFreeName] = useState('');
  const [freePrice, setFreePrice] = useState('');
  const [name, setName] = useState('');
  const [phone, setPhone] = useState('');
  const [method, setMethod] = useState<Method>('cash');
  const [discMode, setDiscMode] = useState<DiscountMode>('none');
  const [discText, setDiscText] = useState('');
  const [received, setReceived] = useState(''); // cash handed over, only to show the change (not saved)
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const [sales, setSales] = useState<Sale[] | null>(null);
  const [removing, setRemoving] = useState<Sale | null>(null); // asks before deleting
  const [loadError, setLoadError] = useState<string | null>(null); // shown with a retry, never as an empty catalog
  const today = todayISO();
  const retail = has('productSales');

  const load = useCallback(() => {
    const path = { businessId };
    setLoadError(null);
    api.GET('/v1/b/{businessId}/catalog', { params: { path } }).then(({ data, error, response }) => {
      if (data) setItems(data.items.filter((i) => i.active));
      else setLoadError(`${strings.bookings.catalogError} ${errorText(error, response.status)}`);
    });
    // Always asked (the API answers 403 without productSales): gating on `has` raced the business loading.
    api.GET('/v1/b/{businessId}/products', { params: { path } }).then(({ data, error, response }) => {
      if (data) setProducts(data.products.filter((p) => p.active));
      else setLoadError(`${strings.products.loadError} ${errorText(error, response.status)}`);
    });
    api.GET('/v1/b/{businessId}/sales', { params: { path, query: { from: today, to: today } } }).then(({ data }) => setSales(data?.sales ?? []));
  }, [businessId, today]);
  useFocusEffect(load);

  // Free lines and described free-price items are each their own line.
  const freeKey = () => `free:${Date.now()}:${Math.random().toString(36).slice(2, 6)}`;

  function addFree() {
    const price = parsePesos(freePrice);
    if (!freeName.trim() || !freePrice.trim()) return setErrors({ free: t.missing });
    if (!price || price <= 0) return setErrors({ free: t.badPrice });
    setErrors({});
    setCart((c) => addToCart(c, custom
      ? { key: freeKey(), name: `${custom.name} · ${freeName.trim()}`, price, itemId: custom.id, detail: freeName.trim() }
      : { key: freeKey(), name: freeName.trim(), price }));
    setFreeName(''); setFreePrice(''); setCustom(null);
  }

  async function charge() {
    const discount = discountInput(discMode, discText);
    if (discount === undefined) return setErrors({ discount: strings.bookings.discount.invalid });
    setBusy(true);
    setErrors({});
    const { data, error, response } = await api.POST('/v1/b/{businessId}/sales', {
      params: { path: { businessId } }, body: { lines: saleLines(cart), method, customer_name: name, customer_phone: phone, ...(booking ? { booking_id: booking } : {}), ...(discount ? { discount } : {}) },
    });
    setBusy(false);
    if (!data) { setErrors(fieldErrors(error)); return toast(errorText(error, response.status), 'error'); }
    toast(t.sold);
    if (booking) return router.back();
    setCart([]); setName(''); setPhone(''); setDiscMode('none'); setDiscText(''); setReceived('');
    load();
  }

  async function cancel(id: string) {
    const { error, response } = await api.DELETE('/v1/b/{businessId}/sales/{saleId}', { params: { path: { businessId, saleId: id } } });
    setRemoving(null);
    if (!response.ok) return toast(errorText(error, response.status), 'error');
    toast(t.cancelled);
    load();
  }

  if (loadError) {
    return (
      <Screen>
        <Notice tone="warning">{loadError}</Notice>
        <Button onPress={load} style={{ marginTop: 12 }}>{strings.common.retry}</Button>
      </Screen>
    );
  }
  if (!business || !items) return <ActivityIndicator style={{ marginTop: 24 }} />;
  const subtotal = cartTotal(cart);
  const off = discountOff(discountInput(discMode, discText), subtotal); // shown before charging; the API applies it
  const total = subtotal - off;
  const cash = method === 'cash' ? parsePesos(received) : null;
  const state = (i: number): StepState => (i === 0 ? (cart.length ? 'done' : 'current') : cart.length ? 'current' : 'upcoming');

  return (
    <Screen>
      <Stack.Screen options={{ title: t.title }} />
      <View style={{ gap: 16 }}>
        {booking && <Notice tone="info">{t.forOrder(who ?? '')}</Notice>}
        <Step inline n={1} title={t.what.title} description={t.what.description} state={state(0)}>
          <Text variant="subhead" color="labelSecondary">{t.catalog}</Text>
          <Chips>
            {items.map((i) => (
              <Chip key={i.id} selected={custom?.id === i.id} onPress={() => (i.custom_price
                ? setCustom(custom?.id === i.id ? null : i)
                : setCart((c) => addToCart(c, { key: `item:${i.id}`, name: i.name, price: i.price, itemId: i.id })))}>
                {`${i.name} · ${i.custom_price ? strings.catalog.customPrice : formatMXN(i.price)}`}
              </Chip>
            ))}
          </Chips>
          {retail && products.length > 0 && (
            <>
              <Text variant="subhead" color="labelSecondary">{t.products}</Text>
              <Chips>
                {products.map((p) => (
                  <Chip key={p.id} onPress={() => setCart((c) => addToCart(c, { key: `product:${p.id}`, name: p.name, price: p.price, productId: p.id, max: p.stock }))}>
                    {`${p.name} · ${formatMXN(p.price)} · ${p.stock ? strings.products.stockLeft(p.stock) : strings.products.noStock}`}
                  </Chip>
                ))}
              </Chips>
            </>
          )}
          <Text variant="subhead" color="labelSecondary">{custom ? t.detail(custom.name) : t.free}</Text>
          <View style={{ flexDirection: 'row', gap: 12 }}>
            <View style={{ flex: 2 }}><Field label={t.freeName} placeholder={custom ? undefined : t.freeNamePlaceholder} value={freeName} onChangeText={setFreeName} /></View>
            <View style={{ flex: 1 }}><Field label={t.price} value={freePrice} onChangeText={setFreePrice} keyboardType="numeric" /></View>
          </View>
          {errors.free && <Notice tone="warning">{errors.free}</Notice>}
          <Button icon="add-circle" onPress={addFree}>{t.add}</Button>

          <View style={{ backgroundColor: ui.fill, borderRadius: radius.sm, overflow: 'hidden' }}>
            {cart.length === 0 && <Text color="labelSecondary" style={{ padding: 14 }}>{t.empty}</Text>}
            {cart.map((l, i) => (
              <View key={l.key} style={{ flexDirection: 'row', alignItems: 'center', gap: 10, padding: 14, borderTopWidth: i ? StyleSheet.hairlineWidth : 0, borderColor: ui.separator }}>
                <View style={{ flex: 1 }}>
                  <Text>{l.name}</Text>
                  <Text variant="footnote" color="labelSecondary">{formatMXN(l.price)}</Text>
                </View>
                <Pressable accessibilityRole="button" accessibilityLabel={t.less(l.name)} hitSlop={10} onPress={() => setCart((c) => changeQuantity(c, l.key, -1))}>
                  <Ionicons name="remove-circle" size={26} color={ui.accent} />
                </Pressable>
                <Text variant="headline" style={{ minWidth: 22, textAlign: 'center', fontVariant: ['tabular-nums'] }}>{l.quantity}</Text>
                <Pressable accessibilityRole="button" accessibilityLabel={t.more(l.name)} hitSlop={10} disabled={l.quantity >= (l.max ?? Infinity)}
                  onPress={() => setCart((c) => changeQuantity(c, l.key, 1))}>
                  <Ionicons name="add-circle" size={26} color={l.quantity >= (l.max ?? Infinity) ? ui.labelSecondary : ui.accent} />
                </Pressable>
              </View>
            ))}
          </View>
          {errors.lines && <Notice tone="warning">{errors.lines}</Notice>}
        </Step>

        {!booking && (
          <Step inline n={2} title={t.customer.title} description={t.customer.description} state={state(1)}>
            <Field label={strings.customers.walkInName} value={name} onChangeText={setName} error={errors.customer_name} />
            <Field label={strings.customers.walkInPhone} value={phone} onChangeText={setPhone} keyboardType="phone-pad" error={errors.customer_phone} />
          </Step>
        )}

        <Step inline n={booking ? 2 : 3} title={t.pay.title} description={t.pay.description} state={state(2)}>
          {/* An order's extras can also go on its balance, charged at delivery. */}
          <Segmented options={(['cash', 'transfer', 'card', ...(booking ? ['on_account'] : [])] as Method[]).map((m) => ({ value: m, label: m === 'on_account' ? t.onAccount : strings.bookings.methods[m] }))} value={method} onChange={setMethod} />
          <DiscountFields mode={discMode} text={discText} onMode={setDiscMode} onText={setDiscText}
            error={errors.discount ?? (discountInput(discMode, discText) === undefined && discText.trim() ? strings.bookings.discount.invalid : undefined)} />
          {off > 0 && <Row label={strings.bookings.subtotal} value={formatMXN(subtotal)} />}
          {off > 0 && <Row label={t.discountRow} value={`− ${formatMXN(off)}`} />}
          <Row label={strings.products.total} value={formatMXN(total)} strong />
          {method === 'cash' && cart.length > 0 && (
            <>
              <Field label={t.received} value={received} onChangeText={setReceived} keyboardType="decimal-pad" />
              {cash !== null && cash > 0 && <Row label={cash >= total ? t.change : t.missingCash} value={formatMXN(Math.abs(cash - total))} strong />}
            </>
          )}
          <Button variant="primary" icon="checkmark-circle" busy={busy} disabled={!cart.length} onPress={charge}>{method === 'on_account' ? t.addOnAccount(formatMXN(total)) : t.charge(formatMXN(total))}</Button>
        </Step>

        <Group label={t.today}>
          {sales?.length === 0 ? <Cell label={t.noSales} /> : null}
          {(sales ?? []).map((s) => (
            <Cell key={s.id} label={s.lines.map((l) => `${l.quantity} × ${l.name}`).join(', ')}
              sublabel={`${hour(s.sold_at)} · ${strings.bookings.methods[s.method]}${s.customer_name ? ` · ${s.customer_name}` : ''}`}
              value={formatMXN(s.total)}
              accessory={business.role === 'owner' && !s.locked ? (
                <Pressable accessibilityRole="button" accessibilityLabel={t.cancel} hitSlop={10} onPress={() => setRemoving(s)}>
                  <Ionicons name="trash-outline" size={18} color={ui.danger} />
                </Pressable>
              ) : undefined} />
          ))}
        </Group>
      </View>
      <ConfirmDialog open={!!removing} title={t.cancel} confirmLabel={t.cancelConfirm} onClose={() => setRemoving(null)}
        body={removing ? t.cancelBody(removing.lines.map((l) => `${l.quantity} × ${l.name}`).join(', '), formatMXN(removing.total)) : ''}
        onConfirm={() => removing && cancel(removing.id)} />
    </Screen>
  );
}
