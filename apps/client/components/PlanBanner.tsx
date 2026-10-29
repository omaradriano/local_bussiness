import { strings } from '@lb/shared';
import { useBusiness } from '../lib/useMe';
import { Notice } from '../ui';

/** The trial's days left, or that the account is read-only because its trial or plan ended. */
export function PlanBanner({ businessId }: { businessId: string }) {
  const a = useBusiness(businessId).business?.account;
  if (!a) return null;
  if (a.status === 'expired') return <Notice tone="warning">{a.plan === 'trial' ? strings.plans.trialEnded : strings.plans.planEnded}</Notice>;
  if (a.status === 'trial') return <Notice>{strings.plans.trialLeft(a.days_left)}</Notice>;
  return null;
}
