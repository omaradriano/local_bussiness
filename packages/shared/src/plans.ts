import catalog from '../../../services/api/internal/plans/plans.json';

/**
 * What each plan includes, from services/api/internal/plans/plans.json (the API reads the same file
 * and enforces it; the app only hides what the plan doesn't include).
 */
export type PlanKey = 'trial' | 'lite' | 'pro' | 'business';
export type Capability = 'expenses' | 'recurring_expenses' | 'installments' | 'payroll' | 'any_period' | 'compare_businesses';
type Def = (typeof catalog.plans)[keyof typeof catalog.plans];

export const TRIAL_DAYS: number = catalog.trial.days;

export function planDef(plan: PlanKey): Def {
  return catalog.plans[(plan === 'trial' ? catalog.trial.as : plan) as keyof typeof catalog.plans];
}

export function planCan(plan: PlanKey, capability: Capability): boolean {
  return (planDef(plan).capabilities as string[]).includes(capability);
}

const CAPABILITY_LABELS: Record<Capability, string> = {
  expenses: 'Gastos',
  recurring_expenses: 'Gastos recurrentes',
  installments: 'Compras a plazos',
  payroll: 'Sueldos y comisiones',
  any_period: 'Reportes de cualquier periodo',
  compare_businesses: 'Comparar negocios',
};

const count = (n: number | null, one: string, many: string, unlimited: string) =>
  n === null ? unlimited : n === 1 ? `1 ${one}` : `Hasta ${n} ${many}`;

/** What a plan includes, in the owner's words (for the plans page), from the same catalog the API enforces. */
export function planHighlights(plan: Exclude<PlanKey, 'trial'>): string[] {
  const d = planDef(plan);
  const l = d.limits;
  const excluded = d.excluded_features as string[];
  const out = [
    count(l.businesses, 'negocio', 'negocios', 'Negocios ilimitados'),
    count(l.members, 'usuario', 'usuarios', 'Usuarios ilimitados'),
    count(l.catalog_items, 'artículo en el catálogo', 'artículos en el catálogo', 'Catálogo ilimitado'),
    count(l.rental_units, 'unidad en renta', 'unidades en renta', 'Unidades en renta ilimitadas'),
    `${l.photos_per_item} fotos por artículo`,
  ];
  if (!excluded.includes('staff')) out.push(l.staff === null ? 'Equipo sin límite de personas' : `Equipo: hasta ${l.staff} personas`);
  if (!excluded.includes('productSales')) out.push('Venta de productos con inventario');
  if (!excluded.includes('bookingDocuments')) out.push('Documentos adicionales en cada reserva');
  for (const c of d.capabilities as Capability[]) out.push(CAPABILITY_LABELS[c]);
  return out;
}
