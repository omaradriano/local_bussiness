import { describe, expect, it } from 'vitest';
import { businessNotice, platformNotice, PRIVACY_VERSION } from './privacy';

const text = (sections: { body: string[] }[]) => sections.flatMap((s) => s.body).join('\n');

describe('privacy notices', () => {
  it('carries the version the API records', () => {
    expect(PRIVACY_VERSION).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(text(platformNotice())).toContain(`Versión ${PRIVACY_VERSION}`);
  });

  it('names the business as responsible, with its address and ARCO contacts', () => {
    const t = text(businessNotice({ name: 'Barber360', legal_name: 'Juan Pérez', address: 'Calle 1, Chihuahua', email: 'hola@barber.mx', phone: '+526141234567' }));
    expect(t).toContain('Juan Pérez, con domicilio en Calle 1, Chihuahua, es responsable');
    expect(t).toContain('hola@barber.mx o WhatsApp');
  });

  it('still says how to reach the business when nothing was captured', () => {
    const t = text(businessNotice({ name: 'Barber360', legal_name: 'Barber360', address: '', email: '', phone: '' }));
    expect(t).toContain('Barber360 es responsable');
    expect(t).toContain('escribiendo a Barber360 directamente');
  });
});
