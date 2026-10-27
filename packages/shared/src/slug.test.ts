import { describe, expect, it } from 'vitest';
import { slugify } from './slug';

describe('slugify', () => {
  it.each([
    ['Uñas Bellas, Chihuahua', 'unas-bellas-chihuahua'],
    ['  Florería  Las Margaritas ', 'floreria-las-margaritas'],
    ['Rentas "El Taller" #2', 'rentas-el-taller-2'],
    ['---', ''],
  ])('%s -> %s', (name, slug) => {
    expect(slugify(name)).toBe(slug);
  });

  it('caps at 60 chars without a trailing dash', () => {
    const s = slugify('a'.repeat(59) + ' bc');
    expect(s.length).toBeLessThanOrEqual(60);
    expect(s.endsWith('-')).toBe(false);
  });
});
