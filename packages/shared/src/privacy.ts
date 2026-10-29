import notice from '../../../services/api/internal/privacy/privacy.json';
import { formatPhone } from './phone';

/**
 * Privacy notices (Ley Federal de Protección de Datos Personales en Posesión de los Particulares,
 * DOF 20/03/2025): the platform's (owners and staff) and each business's (its customers).
 * The version lives in services/api/internal/privacy/privacy.json, read by the API too, so a
 * recorded acceptance names the text shown. Change it whenever these texts change.
 * Draft to be reviewed by a lawyer before selling.
 */
export const PRIVACY_VERSION: string = notice.version;

export type NoticeSection = { title: string; body: string[] };

// Contacto para dudas, comentarios y mejoras (ventana de bienvenida).
export const PLATFORM_CONTACT = 'oadriandev@gmail.com';

// Pendiente: el dueño de la plataforma da estos datos; mientras, se ven como marcadores.
export const PLATFORM_RESPONSIBLE = {
  legalName: '[Nombre o razón social del responsable]',
  address: '[Domicilio del responsable]',
  email: '[correo de contacto]',
};

const ARCO = (contact: string) =>
  `Puedes pedir acceso a tus datos, corregirlos, cancelarlos u oponerte a su uso (derechos ARCO), así como revocar tu consentimiento, escribiendo a ${contact}. ` +
  'Indica tu nombre, cómo contactarte, qué datos y qué derecho quieres ejercer, y una identificación. Te responderemos en un máximo de 20 días hábiles. ' +
  'Si consideras que tu derecho no fue atendido, puedes acudir a la Secretaría Anticorrupción y Buen Gobierno.';

/** Notice of the platform: people who create an account to run a business. */
export function platformNotice(r = PLATFORM_RESPONSIBLE): NoticeSection[] {
  return [
    { title: 'Responsable', body: [`${r.legalName}, con domicilio en ${r.address}, es responsable de tus datos personales como usuario de la plataforma.`] },
    { title: 'Datos que recabamos', body: [
      'Tu correo y contraseña para entrar, tu nombre, y los datos de tu negocio: nombre, teléfono de WhatsApp, horario, catálogo, precios, equipo y movimientos (citas, rentas, pedidos, ventas, gastos y sueldos).',
      'No te pedimos datos sensibles.',
    ] },
    { title: 'Para qué los usamos', body: [
      'Para darte acceso a tu cuenta, prestar el servicio, guardar la información de tu negocio, darte soporte y avisarte de cambios en el servicio.',
      'No usamos tus datos para publicidad ni para fines distintos a estos.',
    ] },
    { title: 'Datos de los clientes de tu negocio', body: [
      'Los datos que registras de tus clientes (nombre, teléfono, domicilio, identificaciones, aval) son responsabilidad de tu negocio, que debe darles su propio aviso de privacidad (la plataforma lo genera en Ajustes → Privacidad). ' +
      'Nosotros solo los guardamos y tratamos por encargo tuyo, para prestarte el servicio, y no los usamos para nada más.',
    ] },
    { title: 'Con quién los compartimos', body: [
      'Usamos proveedores que guardan y procesan la información por nuestra cuenta (base de datos y autenticación, almacenamiento de archivos y servidores), algunos ubicados fuera de México. Están obligados a protegerla y no pueden usarla para otros fines.',
      'No vendemos ni transferimos tus datos a terceros, salvo que una autoridad lo requiera conforme a la ley.',
    ] },
    { title: 'Tus derechos', body: [ARCO(r.email)] },
    { title: 'Tecnologías de rastreo', body: ['La app guarda tu sesión en tu dispositivo para que no tengas que entrar cada vez. No usamos cookies de publicidad ni de rastreo.'] },
    { title: 'Cambios a este aviso', body: [`Publicaremos cualquier cambio en esta página. Versión ${PRIVACY_VERSION}.`] },
  ];
}

export type BusinessNoticeData = { name: string; legal_name: string; address: string; email: string; phone: string };

/** Notice of a business for its customers, from Ajustes → Privacidad (empty fields: its name and WhatsApp). */
export function businessNotice(b: BusinessNoticeData): NoticeSection[] {
  const where = b.address ? `, con domicilio en ${b.address},` : '';
  const of = b.legal_name === b.name ? '' : ` de ${b.name}`;
  const contact = [b.email, b.phone && `WhatsApp ${formatPhone(b.phone)}`].filter(Boolean).join(' o ') || `${b.name} directamente`;
  return [
    { title: 'Responsable', body: [`${b.legal_name}${where} es responsable de los datos personales que le das como cliente${of}.`] },
    { title: 'Datos que recabamos', body: [
      'Tu nombre, teléfono y, si hace falta para tu servicio, correo y domicilio.',
      'En rentas: identificación oficial, comprobante de domicilio y los datos de una persona que te avale. Si nos das los datos de tu aval, avísale de este aviso.',
      'No te pedimos datos sensibles.',
    ] },
    { title: 'Para qué los usamos', body: [
      'Para apartar, atender y cobrar tus citas, pedidos o rentas; contactarte por WhatsApp o teléfono sobre ellos; y, en rentas, verificar tu identidad y asegurar la devolución de lo rentado.',
      'No usamos tus datos para publicidad ni para fines distintos a estos.',
    ] },
    { title: 'Con quién los compartimos', body: [
      `${b.name} usa una plataforma de reservas que guarda tus datos por su cuenta y no puede usarlos para nada más. No vendemos ni transferimos tus datos a terceros, salvo que una autoridad lo requiera conforme a la ley.`,
    ] },
    { title: 'Tus derechos', body: [ARCO(contact)] },
    { title: 'Cambios a este aviso', body: [`Publicaremos cualquier cambio en esta página. Versión ${PRIVACY_VERSION}.`] },
  ];
}
