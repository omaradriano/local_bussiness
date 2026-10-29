import { describe, expect, it } from 'vitest';
import { termsSections, TERMS_VERSION } from './terms';

describe('terms and conditions', () => {
  const text = termsSections().flatMap((s) => s.body).join('\n');
  it('says the trial length from the plans catalog and its version', () => {
    expect(text).toContain('14 días de prueba');
    expect(text).toContain(`Versión ${TERMS_VERSION}`);
  });
  it('explains read-only accounts and deleting a business', () => {
    expect(text).toContain('solo lectura');
    expect(text).toContain('borrar tu negocio');
  });
});
