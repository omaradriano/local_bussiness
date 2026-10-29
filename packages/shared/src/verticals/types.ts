// The map of a type of business ("giro"): everything that changes between giros lives here, one file
// per giro (see docs/mapa-giros.md). A giro rides on an engine (ItemKind): how things are booked is code;
// how a giro names, configures and shows them is this map.

export type ItemKind = 'service' | 'product' | 'rental';

export type BookingStatus =
  | 'pending_deposit' | 'confirmed' | 'in_progress' | 'ready' | 'out_for_delivery'
  | 'checked_out' | 'overdue' | 'returned' | 'completed' | 'cancelled' | 'expired' | 'no_show';

/**
 * Parts of the app a giro turns on. Screens ask `hasFeature(vertical, 'guarantor')`, never "is it a rental?".
 * The ones the API enforces are mirrored in the giro's template (services/api/internal/businesses/templates).
 * - staff: people who attend, each with services and hours (agenda)
 * - inventory: calendar tab with every unit and its state
 * - guarantor: the customer brings a guarantor (name + phone, required by the API)
 * - customerAddress: the customer's address is required (API)
 * - customerDocuments: ID and proof of address on the customer's profile, reused in each booking
 * - trustedCustomers: the owner can exempt a customer from documents
 * - walkIn: book a «Cliente casual» without registering anyone
 * - homeService: the service can be at the customer's address
 * - startService: an "Iniciar" step before finishing an appointment (some salons track it; barbers don't)
 * - bookingDocuments: attach files with a name to a booking ("Documentos adicionales")
 * - productSales: sell products with stock and cost (wax, screws), with their stats
 */
export type Feature =
  | 'staff' | 'inventory' | 'guarantor' | 'customerAddress' | 'customerDocuments' | 'trustedCustomers'
  | 'walkIn' | 'homeService' | 'startService' | 'bookingDocuments' | 'productSales';

/** Ajustes sections. "general" is shared by every business (advance and holds); "agenda" is the appointment engine's. */
export type SettingsSection = 'general' | 'agenda' | 'rentals' | 'inspection' | 'documents' | 'delivery';

export type Terms = {
  booking: string; // "Renta"
  catalog_item: string; // "Herramienta"
  catalog: string; // "Herramientas"
  starts_at: string; // "Fecha de salida"
  ends_at: string | null; // "Fecha de regreso"; null = the giro has no end
};

/** Visible text that changes with the giro (words and grammatical gender). Shared text stays in `strings`. */
export type VerticalTexts = {
  bookings: {
    plural: string; // "Rentas"
    searchPlaceholder: string;
    empty: string; // history with no results
    ofCustomer: string; // section in the customer's profile
    noneThatDay: string;
    inactive: string; // a cancelled / expired one
    /** Descriptive state: where it is and what's next. */
    status: Record<BookingStatus, string>;
  };
  home: { today: string; calendar: string; customers: string; history: string; catalog: string; settings: string };
  newBooking: { itemHint: string; customerHint: string; paymentHint: string };
  catalog: { activeHint: string };
};

export type VerticalMap = {
  /** Name in the sign-up list ("Renta de herramienta"). */
  label: string;
  /** Engine: how things are booked. */
  kind: ItemKind;
  terms: Terms;
  texts: VerticalTexts;
  /** Ajustes sections, in order; the first is always "general". */
  settings: SettingsSection[];
  /** On by default. */
  features: Feature[];
  /** Offered but off until the owner turns them on in Ajustes (e.g. home service). */
  optionalFeatures: Feature[];
};
