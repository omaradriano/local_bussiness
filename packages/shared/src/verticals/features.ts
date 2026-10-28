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
