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
