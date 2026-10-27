// Visible UI text (es-MX), centralized so it can vary by vertical later.
export const strings = {
  appName: 'Local Bussiness',
  common: {
    loading: 'Cargando…',
    retry: 'Reintentar',
    error: 'Algo salió mal. Intenta de nuevo.',
  },
  admin: {
    title: 'Panel',
    yourBusinesses: 'Tus negocios',
    noBusinesses: 'Todavía no tienes un negocio registrado.',
    createBusiness: 'Registrar mi negocio',
    signOut: 'Cerrar sesión',
    role: { owner: 'Dueño', staff: 'Empleado' },
  },
  auth: {
    loginTitle: 'Iniciar sesión',
    signUpTitle: 'Crear cuenta',
    email: 'Correo',
    password: 'Contraseña',
    submitLogin: 'Entrar',
    submitSignUp: 'Crear cuenta',
    toSignUp: '¿No tienes cuenta? Crea una',
    toLogin: '¿Ya tienes cuenta? Inicia sesión',
    checkEmail: 'Te enviamos un correo para confirmar tu cuenta. Ábrelo y después inicia sesión.',
  },
  onboarding: {
    title: 'Registra tu negocio',
    name: 'Nombre del negocio',
    slug: 'Dirección de tu catálogo',
    slugHint: 'Así te encontrarán tus clientes: tudominio.com/',
    vertical: '¿A qué se dedica?',
    verticals: { nails: 'Uñas y belleza', flowers: 'Florería bajo pedido', tool_rental: 'Renta de herramienta' },
    submit: 'Crear negocio',
    templateNote: 'Te dejamos un catálogo y horario de ejemplo; los puedes cambiar después.',
  },
  publicCatalog: { placeholder: 'Catálogo público (fase 6).' },
} as const;
