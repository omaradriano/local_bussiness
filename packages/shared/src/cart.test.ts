import { describe, expect, it } from 'vitest';
import { addToCart, cartTotal, changeQuantity, saleLines, type CartLine } from './cart';

const ramo = { key: 'item:ramo', name: 'Ramo', price: 65000, itemId: 'ramo' };
const globo = { key: 'product:globo', name: 'Globo', price: 5000, productId: 'globo', max: 2 };

describe('cart', () => {
  it('adds one more of the same line and adds up the total', () => {
    let cart: CartLine[] = [];
    cart = addToCart(cart, ramo);
    cart = addToCart(cart, ramo);
    cart = addToCart(cart, { key: 'free:1', name: 'Girasoles', price: 35000 });
    expect(cart.map((l) => l.quantity)).toEqual([2, 1]);
    expect(cartTotal(cart)).toBe(2 * 65000 + 35000);
  });

  it('never goes above the stock and removes a line at zero', () => {
    let cart = addToCart(addToCart(addToCart([], globo), globo), globo);
    expect(cart[0].quantity).toBe(2);
    cart = changeQuantity(changeQuantity(cart, globo.key, -1), globo.key, -1);
    expect(cart).toEqual([]);
    expect(addToCart([], { ...globo, max: 0 })).toEqual([]);
  });

  it('turns lines into what the API takes', () => {
    const cart: CartLine[] = [
      { ...ramo, quantity: 1 },
      { key: 'item:libre:1', name: 'Arreglo personalizado', price: 120000, itemId: 'libre', detail: '24 rosas', quantity: 1 },
      { key: 'free:1', name: 'Girasoles', price: 35000, quantity: 2 },
      { ...globo, quantity: 1 },
    ];
    expect(saleLines(cart)).toEqual([
      { item_id: 'ramo', quantity: 1 },
      { item_id: 'libre', quantity: 1, name: '24 rosas', price: 120000 },
      { name: 'Girasoles', price: 35000, quantity: 2 },
      { product_id: 'globo', quantity: 1 },
    ]);
  });
});
