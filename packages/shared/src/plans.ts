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

export type PaidPlan = Exclude<PlanKey, 'trial'>;
export const PAID_PLANS: PaidPlan[] = ['lite', 'pro', 'business'];
/** A cell: a limit ("30", "Ilimitado") or whether the plan includes it. */
export type PlanCell = string | boolean;
export type PlanRow = { label: string; cells: Record<PaidPlan, PlanCell> };

const limit = (n: number | null): string => (n === null ? 'Ilimitado' : String(n));
const row = (label: string, cell: (p: PaidPlan) => PlanCell): PlanRow =>
  ({ label, cells: Object.fromEntries(PAID_PLANS.map((p) => [p, cell(p)])) as Record<PaidPlan, PlanCell> });

/** Plans side by side (for the plans page), from the same catalog the API enforces. */
export function planComparison(): PlanRow[] {
  const d = (p: PaidPlan) => planDef(p);
  const has = (p: PaidPlan, f: string) => !(d(p).excluded_features as string[]).includes(f);
  const capabilities = [...new Set(PAID_PLANS.flatMap((p) => d(p).capabilities as Capability[]))];
  return [
    row('Negocios', (p) => limit(d(p).limits.businesses)),
    row('Usuarios', (p) => limit(d(p).limits.members)),
    row('Artículos en el catálogo', (p) => limit(d(p).limits.catalog_items)),
    row('Unidades en renta', (p) => limit(d(p).limits.rental_units)),
    row('Fotos por artículo', (p) => limit(d(p).limits.photos_per_item)),
    row('Equipo (personas)', (p) => has(p, 'staff') && limit(d(p).limits.staff)),
    row('Venta de productos', (p) => has(p, 'productSales')),
    row('Documentos adicionales', (p) => has(p, 'bookingDocuments')),
    ...capabilities.map((c) => row(CAPABILITY_LABELS[c], (p) => (d(p).capabilities as string[]).includes(c))),
  ];
}
