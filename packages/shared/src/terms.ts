import { TRIAL_DAYS } from './plans';
import { PLATFORM_CONTACT, PLATFORM_RESPONSIBLE, type NoticeSection } from './privacy';
import { APP_NAME } from './strings';

/**
 * Terms and conditions of the platform (owners and their team). Change TERMS_VERSION whenever the
 * text changes; sign-up records the version accepted. Draft to be reviewed by a lawyer before selling.
 */
export const TERMS_VERSION = '2026-10-29';

export function termsSections(r = PLATFORM_RESPONSIBLE): NoticeSection[] {
  return [
    { title: 'Quién presta el servicio', body: [
      `${APP_NAME} es una plataforma para administrar negocios (citas, rentas, pedidos, clientes, cobros, productos y finanzas), prestada por ${r.legalName}, con domicilio en ${r.address}. Al crear una cuenta aceptas estos términos.`,
    ] },
    { title: 'Tu cuenta', body: [
      'Eres responsable de lo que se haga con tu cuenta y de mantener tu contraseña en secreto. Avísanos si crees que alguien más la usa.',
      'Debes ser mayor de edad y dar información verdadera de tu negocio.',
    ] },
    { title: 'Prueba gratis y planes', body: [
      `Al registrar tu primer negocio tienes ${TRIAL_DAYS} días de prueba con las funciones del plan Pro, sin pedirte tarjeta.`,
      'Después puedes contratar un plan (Lite, Pro o Business). Cada plan tiene límites y funciones distintas; te los mostramos antes de contratar. El precio, la forma de pago y el periodo se acuerdan contigo.',
      'Si la prueba o tu plan terminan y no lo renuevas, tu cuenta queda en solo lectura: puedes consultar tu información, pero no registrar nada nuevo. No borramos tus datos por eso.',
    ] },
    { title: 'Tus datos y los de tus clientes', body: [
      'La información que registras (clientes, reservas, documentos, finanzas) es tuya. Nosotros la guardamos y procesamos solo para prestarte el servicio, conforme al aviso de privacidad.',
      'Eres responsable de los datos de tus clientes: de pedirlos con un fin legítimo y de mostrarles el aviso de privacidad de tu negocio (lo generas en Ajustes → Privacidad).',
      'Puedes borrar tu negocio cuando quieras desde Ajustes; se borra toda su información y sus archivos, y no se puede recuperar.',
    ] },
    { title: 'Uso permitido', body: [
      'No uses la plataforma para actividades ilegales, para guardar información que no tengas derecho a tener, ni para intentar acceder a datos de otros negocios o afectar el servicio.',
      'Podemos suspender una cuenta que incumpla estos términos, avisándote cuando sea posible.',
    ] },
    { title: 'Proyecto en desarrollo', body: [
      'La plataforma está en desarrollo: algunas funciones pueden cambiar, mejorar o retirarse. Te avisaremos de los cambios importantes.',
      'Hacemos lo posible por que esté disponible y sin errores, pero puede haber interrupciones o fallas. Te recomendamos revisar la información importante y exportarla cuando lo necesites.',
    ] },
    { title: 'Responsabilidad', body: [
      'No somos responsables de decisiones de tu negocio tomadas con la información de la plataforma, ni de pérdidas causadas por un uso distinto al previsto, por fallas de internet o de tu dispositivo, o por causas fuera de nuestro control.',
      'Nuestra responsabilidad, en cualquier caso, se limita a lo que hayas pagado por el servicio en los últimos tres meses.',
    ] },
    { title: 'Cambios y contacto', body: [
      `Si cambiamos estos términos, publicaremos la versión nueva aquí y te avisaremos. Versión ${TERMS_VERSION}.`,
      `Dudas, comentarios o sugerencias: ${PLATFORM_CONTACT}.`,
      'Estos términos se rigen por las leyes de México.',
    ] },
  ];
}
