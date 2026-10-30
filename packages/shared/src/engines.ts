import modules from '../../../services/api/internal/engines/engines.json';
import type { ItemKind } from './verticals/types';

// The modules each engine is made of, from the API's engines.json (Go checks its tables match it).
// Screens ask a module ("does it have a return date?", "does it go out and come back?"), never the
// engine by name, so a new engine made of existing modules needs no screen changes.
export type EngineModules = {
  timing: 'slot' | 'day' | 'range'; // a time slot, a day, or from–to
  fulfillment: 'atPlaceOrHome' | 'pickupOrDelivery' | 'atPlace';
  resource: 'staff' | 'dailyCapacity' | 'units';
  pricing: 'fixed' | 'perPeriod';
  custody: boolean; // goes with the customer and comes back (check-out, return, deposit, late fees)
};

const ENGINES = modules as Record<ItemKind, EngineModules>;

export function engineFor(kind: ItemKind): EngineModules {
  return ENGINES[kind];
}
