// Quick sale cart (feature quickSale): catalog items, free amounts and, with productSales, products
// with stock. The API re-prices everything; the cart only adds up what the screen shows.

export type CartLine = {
  key: string; // same key = same line (a catalog item or product at its price); free lines are unique
  name: string;
  price: number; // centavos per unit
  quantity: number;
  itemId?: string;
  productId?: string;
  detail?: string; // what a free-price item is («24 rosas rojas»)
  max?: number; // stock left
};

/** Adds a line, or one more of it when it's already there (never above the stock). */
export function addToCart(cart: CartLine[], line: Omit<CartLine, 'quantity'>): CartLine[] {
  const i = cart.findIndex((l) => l.key === line.key);
  if (i < 0) return line.max === 0 ? cart : [...cart, { ...line, quantity: 1 }];
  return changeQuantity(cart, line.key, 1);
}

/** One more or one less; at zero the line goes away. */
export function changeQuantity(cart: CartLine[], key: string, delta: number): CartLine[] {
  return cart
    .map((l) => (l.key === key ? { ...l, quantity: Math.min(l.max ?? Infinity, l.quantity + delta) } : l))
    .filter((l) => l.quantity > 0);
}

export function cartTotal(cart: CartLine[]): number {
  return cart.reduce((n, l) => n + l.price * l.quantity, 0);
}

/** The lines as the API takes them. */
export function saleLines(cart: CartLine[]) {
  return cart.map((l) => (l.productId
    ? { product_id: l.productId, quantity: l.quantity }
    : l.itemId
      ? { item_id: l.itemId, quantity: l.quantity, ...(l.detail ? { name: l.detail, price: l.price } : {}) }
      : { name: l.name, price: l.price, quantity: l.quantity }));
}
