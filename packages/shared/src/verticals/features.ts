import catalog from '../../../../services/api/internal/features/features.json';
import type { Feature, ItemKind } from './types';

// What each feature is, in the owner's words. Which engines support it comes from the API's catalog
// (services/api/internal/features/features.json), so both sides agree.
const INFO: Record<Feature, { label: string; hint: string }> = {
  staff: { label: 'Equipo', hint: 'Personas que atienden, cada una con sus servicios y su horario.' },
  inventory: { label: 'Inventario', hint: 'Pestaña del calendario con cada unidad y si está libre, rentada o agendada.' },
  guarantor: { label: 'Aval del cliente', hint: 'El cliente registra a una persona que responde por él (nombre y teléfono obligatorios).' },
  customerAddress: { label: 'Dirección obligatoria', hint: 'No se registra un cliente sin su dirección.' },
  customerDocuments: { label: 'Documentos del cliente', hint: 'INE y comprobante de domicilio en su ficha; se reutilizan en cada reserva.' },
  trustedCustomers: { label: 'Clientes de confianza', hint: 'El dueño puede marcar a un cliente para no pedirle documentos.' },
  walkIn: { label: 'Clientes casuales', hint: 'Agendar sin registrar al cliente, como «Cliente casual».' },
  homeService: { label: 'Servicio a domicilio', hint: 'Una cita puede ser en la dirección del cliente.' },
  startService: { label: 'Iniciar servicio', hint: 'Marcar cuándo empieza la atención antes de terminarla. Sin esto, la cita se da por terminada directamente.' },
  productSales: { label: 'Venta de productos', hint: 'Inventario con costo y existencias, ventas en mostrador y sus estadísticas.' },
  bookingDocuments: { label: 'Documentos adicionales', hint: 'Adjuntar archivos con nombre a una reserva (contratos, pagarés…).' },
};

/** Name and explanation of a feature for Ajustes and the platform admin. */
export function featureInfo(feature: Feature): { label: string; hint: string } {
  return INFO[feature];
}

/** Whether a feature works with an engine (same rule as the API). */
export function featureSupports(feature: Feature, kind: ItemKind): boolean {
  return (catalog as Record<string, { engines: string[] }>)[feature]?.engines.includes(kind) ?? false;
}

/** Every feature, as the API knows them. */
export const FEATURES = Object.keys(catalog) as Feature[];
