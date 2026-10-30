import { parsePesos } from './money';

export type DiscountMode = 'none' | 'amount' | 'percent';
export type DiscountInput = { type: 'amount' | 'percent'; value: number };

/**
 * Form -> API discount (format only; the API applies it): an amount in pesos becomes centavos, a
 * percent is a whole number 0–100. `null` = no discount; `undefined` = the text can't be read.
 */
export function discountInput(mode: DiscountMode, text: string): DiscountInput | null | undefined {
  if (mode === 'none') return null;
  if (mode === 'amount') {
    const c = parsePesos(text);
    return c === null ? undefined : c === 0 ? null : { type: 'amount', value: c };
  }
  const s = text.trim().replace(/%$/, '').trim();
  if (!/^\d{1,3}$/.test(s) || Number(s) > 100) return undefined;
  return Number(s) === 0 ? null : { type: 'percent', value: Number(s) };
}
