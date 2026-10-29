// Supabase Auth error codes → message for the user (es-MX).
const AUTH_ERRORS: Record<string, string> = {
  invalid_credentials: 'Correo o contraseña incorrectos.',
  email_not_confirmed: 'Confirma tu correo con el enlace que te enviamos antes de entrar.',
  user_already_exists: 'Ya existe una cuenta con ese correo. Inicia sesión.',
  email_exists: 'Ya existe una cuenta con ese correo. Inicia sesión.',
  weak_password: 'La contraseña es muy débil. Usa al menos 8 caracteres.',
  over_email_send_rate_limit: 'Se enviaron demasiados correos. Intenta de nuevo en unos minutos.',
  over_request_rate_limit: 'Demasiados intentos. Espera un momento y vuelve a intentar.',
  validation_failed: 'Revisa el correo y la contraseña.',
};

export function authErrorMessage(code: string | undefined): string {
  return (code && AUTH_ERRORS[code]) || 'No se pudo completar. Intenta de nuevo.';
}

export function validateEmail(email: string): string | null {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim()) ? null : 'Escribe un correo válido.';
}

export function validateCredentials(email: string, password: string): string | null {
  const bad = validateEmail(email);
  if (bad) return bad;
  if (password.length < 8) return 'La contraseña debe tener al menos 8 caracteres.';
  return null;
}

// New password (recovery): same rule as sign-up, typed twice.
export function validateNewPassword(password: string, confirm: string): string | null {
  if (password.length < 8) return 'La contraseña debe tener al menos 8 caracteres.';
  if (password !== confirm) return 'Las contraseñas no coinciden.';
  return null;
}
