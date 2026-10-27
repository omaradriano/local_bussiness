import { describe, expect, it } from 'vitest';
import { fitWithin } from './image';

describe('fitWithin', () => {
  it('leaves small photos alone', () => expect(fitWithin(1600, 1200)).toBeNull());
  it('caps landscape by width', () => expect(fitWithin(4032, 3024)).toEqual({ width: 2048, height: 1536 }));
  it('caps portrait by height', () => expect(fitWithin(3024, 4032)).toEqual({ width: 1536, height: 2048 }));
  it('handles exactly the limit', () => expect(fitWithin(2048, 2048)).toBeNull());
  it('always returns whole, positive numbers on both sides', () => {
    const r = fitWithin(10000, 7)!;
    expect(Number.isInteger(r.width) && Number.isInteger(r.height)).toBe(true);
    expect(r).toEqual({ width: 2048, height: 1 });
  });
});
