import { describe, expect, it } from 'vitest';
import { parentPath } from './nav';

describe('parentPath', () => {
  it.each([
    ['/admin/b1/catalog/i1/units', '/admin/b1/catalog/i1'],
    ['/admin/b1/catalog/i1', '/admin/b1/catalog'],
    ['/admin/b1/catalog', '/admin/b1'],
    ['/admin/b1', '/admin'],
    ['/admin/b1/', '/admin'],
  ])('%s -> %s', (path, parent) => expect(parentPath(path)).toBe(parent));

  it('has no parent at the panel root or outside it', () => {
    expect(parentPath('/admin')).toBeNull();
    expect(parentPath('/onboarding')).toBeNull();
  });
});
