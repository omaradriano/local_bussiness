import { describe, expect, it } from 'vitest';
import { authErrorMessage, validateCredentials, validateNewPassword } from './auth';

describe('authErrorMessage', () => {
  it('translates known Supabase codes', () => {
    expect(authErrorMessage('invalid_credentials')).toBe('Correo o contraseña incorrectos.');
    expect(authErrorMessage('email_not_confirmed')).toMatch(/Confirma tu correo/);
  });

  it('falls back to a generic message', () => {
    expect(authErrorMessage('something_new')).toBe('No se pudo completar. Intenta de nuevo.');
    expect(authErrorMessage(undefined)).toBe('No se pudo completar. Intenta de nuevo.');
  });
});

describe('validateCredentials', () => {
  it('accepts a valid email and 8+ char password', () => {
    expect(validateCredentials(' ana@correo.mx ', '12345678')).toBeNull();
  });

  it('rejects bad emails and short passwords', () => {
    expect(validateCredentials('ana@', '12345678')).toBe('Escribe un correo válido.');
    expect(validateCredentials('ana@correo.mx', '1234567')).toBe('La contraseña debe tener al menos 8 caracteres.');
  });
});

describe('validateNewPassword', () => {
  it('asks for 8 characters and the same password twice', () => {
    expect(validateNewPassword('corta', 'corta')).toMatch(/8 caracteres/);
    expect(validateNewPassword('larga-y-buena', 'larga-y-buena!')).toMatch(/no coinciden/);
    expect(validateNewPassword('larga-y-buena', 'larga-y-buena')).toBeNull();
  });
});
