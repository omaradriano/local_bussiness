import { dateOnly, localDateTime, pesos, type Column } from './csv';
import { strings } from './strings';
import type { BookingStatus, VerticalTexts } from './verticals';

/** Rows as GET /export/{kind} returns them (centavos, UTC instants, status keys). */
export type ExportKind = 'customers' | 'bookings' | 'expenses' | 'sales' | 'catalog' | 'products';
type Customer = { name: string; phone: string; email: string | null; address: string | null; guarantor_name: string | null;
  guarantor_phone: string | null; trusted: boolean; notes: string | null; privacy_accepted_at: string | null; created_at: string };
type Booking = { id: string; status: BookingStatus; source: string; starts_at: string | null; ends_at: string | null; customer: string;
  phone: string; items: string; total: number; paid: number; due: number; extra_charges: number; created_at: string };
type Expense = { spent_on: string; category: string; description: string | null; amount: number; recurring: boolean };
type Sale = { sold_at: string; method: string; products: string; total: number; cost: number };
type CatalogItem = { kind: string; name: string; price: number; price_unit: string | null; duration_minutes: number | null;
  security_deposit: number | null; late_fee_per_day: number | null; active: boolean };
type Product = { name: string; price: number; cost: number; stock: number; active: boolean };

const h = strings.exports.headers;

/** Columns of each export in the business's words (texts of its giro), dates in its time zone. */
export function exportColumns(kind: ExportKind, texts: VerticalTexts, timeZone: string): Column<never>[] {
  const at = (iso: string | null) => localDateTime(iso, timeZone);
  const cols: Record<ExportKind, Column<any>[]> = { // eslint-disable-line @typescript-eslint/no-explicit-any
    customers: [
      { header: h.name, value: (r: Customer) => r.name }, { header: h.phone, value: (r: Customer) => r.phone },
      { header: h.email, value: (r: Customer) => r.email }, { header: h.address, value: (r: Customer) => r.address },
      { header: h.guarantor, value: (r: Customer) => r.guarantor_name }, { header: h.guarantorPhone, value: (r: Customer) => r.guarantor_phone },
      { header: h.trusted, value: (r: Customer) => r.trusted }, { header: h.notes, value: (r: Customer) => r.notes },
      { header: h.privacy, value: (r: Customer) => at(r.privacy_accepted_at) }, { header: h.created, value: (r: Customer) => at(r.created_at) },
    ],
    bookings: [
      { header: h.id, value: (r: Booking) => r.id.slice(0, 8) }, { header: h.status, value: (r: Booking) => texts.bookings.status[r.status] ?? r.status },
      { header: h.start, value: (r: Booking) => at(r.starts_at) }, { header: h.end, value: (r: Booking) => at(r.ends_at) },
      { header: h.customer, value: (r: Booking) => r.customer }, { header: h.phone, value: (r: Booking) => r.phone },
      { header: h.items, value: (r: Booking) => r.items }, { header: h.total, value: (r: Booking) => pesos(r.total) },
      { header: h.paid, value: (r: Booking) => pesos(r.paid) }, { header: h.due, value: (r: Booking) => pesos(r.due) },
      { header: h.extra, value: (r: Booking) => pesos(r.extra_charges) },
      { header: h.source, value: (r: Booking) => strings.exports.sources[r.source] ?? r.source }, { header: h.created, value: (r: Booking) => at(r.created_at) },
    ],
    expenses: [
      { header: h.date, value: (r: Expense) => dateOnly(r.spent_on) }, { header: h.category, value: (r: Expense) => strings.finances.categories[r.category] ?? r.category },
      { header: h.description, value: (r: Expense) => r.description }, { header: h.amount, value: (r: Expense) => pesos(r.amount) },
      { header: h.recurring, value: (r: Expense) => r.recurring },
    ],
    sales: [
      { header: h.date, value: (r: Sale) => at(r.sold_at) }, { header: h.products, value: (r: Sale) => r.products },
      { header: h.total, value: (r: Sale) => pesos(r.total) }, { header: h.cost, value: (r: Sale) => pesos(r.cost) },
      { header: h.method, value: (r: Sale) => strings.bookings.methods[r.method] ?? r.method },
    ],
    catalog: [
      { header: h.kind, value: (r: CatalogItem) => strings.exports.itemKinds[r.kind] ?? r.kind }, { header: h.name, value: (r: CatalogItem) => r.name },
      { header: h.price, value: (r: CatalogItem) => pesos(r.price) },
      { header: h.priceUnit, value: (r: CatalogItem) => (r.price_unit ? strings.catalog.priceUnits[r.price_unit as 'day'] : '') },
      { header: h.duration, value: (r: CatalogItem) => r.duration_minutes }, { header: h.deposit, value: (r: CatalogItem) => pesos(r.security_deposit) },
      { header: h.lateFee, value: (r: CatalogItem) => pesos(r.late_fee_per_day) }, { header: h.active, value: (r: CatalogItem) => r.active },
    ],
    products: [
      { header: h.name, value: (r: Product) => r.name }, { header: h.price, value: (r: Product) => pesos(r.price) },
      { header: h.cost, value: (r: Product) => pesos(r.cost) }, { header: h.stock, value: (r: Product) => r.stock },
      { header: h.active, value: (r: Product) => r.active },
    ],
  };
  return cols[kind];
}

/** File name: "<slug>-<what>-<yyyy-mm-dd>.csv". */
export function exportFileName(slug: string, what: string, today: string): string {
  const clean = what.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
  return `${slug}-${clean}-${today}.csv`;
}
