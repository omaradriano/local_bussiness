import { barbershop } from './barbershop';
import { flowers } from './flowers';
import { nails } from './nails';
import { tool_rental } from './tool_rental';
import type { Feature, ItemKind, SettingsSection, Terms, VerticalMap, VerticalTexts } from './types';

export * from './types';
export * from './features';

// Every giro, in sign-up order. Adding a giro: a file like ./tool_rental.ts, its entry here, its API
// template (services/api/internal/businesses/templates/<giro>.json) and the vertical in the database check.
const MAPS = { barbershop, nails, flowers, tool_rental } satisfies Record<string, VerticalMap>;

export type Vertical = keyof typeof MAPS;
export const VERTICALS = Object.keys(MAPS) as Vertical[];

/** The whole map of a giro. */
export function verticalMap(vertical: Vertical): VerticalMap {
  return MAPS[vertical];
}

/** Whether a giro turns on a part of the app. */
export function hasFeature(vertical: Vertical, feature: Feature): boolean {
  return (MAPS[vertical].features as Feature[]).includes(feature);
}

/** The engine a giro books with (same rule as the API). */
export function kindFor(vertical: Vertical): ItemKind {
  return MAPS[vertical].kind;
}

export type Term = keyof Terms;

/** A giro's word for a concept; null = it doesn't apply (e.g. ends_at for flowers). */
export function term(vertical: Vertical, concept: Term): string | null {
  return MAPS[vertical].terms[concept];
}

/** Texts of a giro. */
export function textsFor(vertical: Vertical): VerticalTexts {
  return MAPS[vertical].texts;
}

/** The Ajustes sections a giro sees, in order. */
export function settingsSectionsFor(vertical: Vertical): SettingsSection[] {
  return MAPS[vertical].settings;
}
