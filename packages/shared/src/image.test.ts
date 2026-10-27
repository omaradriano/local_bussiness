import { describe, expect, it } from 'vitest';
import { fitWithin } from './image';

describe('fitWithin', () => {
  it('leaves small photos alone', () => expect(fitWithin(1600, 1200)).toBeNull());
  it('caps landscape by width', () => expect(fitWithin(4032, 3024)).toEqual({ width: 2048, height: null }));
  it('caps portrait by height', () => expect(fitWithin(3024, 4032)).toEqual({ width: null, height: 2048 }));
  it('handles exactly the limit', () => expect(fitWithin(2048, 2048)).toBeNull());
});
