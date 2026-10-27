import { describe, expect, it } from 'vitest';
import { centavosToInput, formatMXN, parsePesos } from './money';

describe('parsePesos', () => {
  it.each([
    ['250', 25000],
    ['250.5', 25050],
    ['250.05', 25005],
    ['$1,250.50', 125050],
    [' 0 ', 0],
  ])('%s -> %i centavos', (input, want) => {
    expect(parsePesos(input)).toBe(want);
  });

  it.each(['', 'abc', '-5', '1.234', '1.2.3', '$'])('rejects %j', (input) => {
    expect(parsePesos(input)).toBeNull();
  });

  it('never produces floating-point centavos', () => {
    expect(parsePesos('0.29')).toBe(29);
    expect(parsePesos('19.99')).toBe(1999);
  });
});

describe('formatting', () => {
  it('formats centavos as MXN', () => {
    expect(formatMXN(125050).replace(/\s/g, ' ')).toBe('$1,250.50');
  });

  it('round-trips through the input format', () => {
    for (const c of [0, 5, 25000, 25050, 125005]) expect(parsePesos(centavosToInput(c))).toBe(c);
  });
});
