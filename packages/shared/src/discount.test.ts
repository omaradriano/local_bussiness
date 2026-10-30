import { describe, expect, it } from 'vitest';
import { discountInput } from './discount';

describe('discountInput', () => {
  it('no discount', () => {
    expect(discountInput('none', '50')).toBeNull();
    expect(discountInput('amount', '0')).toBeNull();
    expect(discountInput('percent', '0')).toBeNull();
  });
  it('an amount in pesos becomes centavos', () => {
    expect(discountInput('amount', '$150.50')).toEqual({ type: 'amount', value: 15050 });
    expect(discountInput('amount', '1,200')).toEqual({ type: 'amount', value: 120000 });
  });
  it('a whole percent from 0 to 100', () => {
    expect(discountInput('percent', '15')).toEqual({ type: 'percent', value: 15 });
    expect(discountInput('percent', '10 %')).toEqual({ type: 'percent', value: 10 });
    expect(discountInput('percent', '100')).toEqual({ type: 'percent', value: 100 });
  });
  it('unreadable text', () => {
    expect(discountInput('amount', 'abc')).toBeUndefined();
    expect(discountInput('percent', '101')).toBeUndefined();
    expect(discountInput('percent', '12.5')).toBeUndefined();
    expect(discountInput('percent', '')).toBeUndefined();
  });
});
