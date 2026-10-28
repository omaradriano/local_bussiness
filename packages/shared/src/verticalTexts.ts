import type { Vertical } from './terminology';

// Visible text that changes with the type of business. Words change ("renta", "cita", "pedido") and
// so does grammatical gender ("Confirmada" vs "Confirmado"), so each vertical writes its own phrases
// instead of filling one template. Text shared by every business stays in `strings`.
// Adding a vertical: TypeScript requires every key below.

export type BookingStatus =
  | 'pending_deposit' | 'confirmed' | 'in_progress' | 'ready' | 'out_for_delivery'
  | 'checked_out' | 'overdue' | 'returned' | 'completed' | 'cancelled' | 'expired' | 'no_show';

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

const TEXTS: Record<Vertical, VerticalTexts> = {
  tool_rental: {
    bookings: {
      plural: 'Rentas',
      searchPlaceholder: 'Cliente, teléfono o herramienta',
      empty: 'No hay rentas con estos filtros.',
      ofCustomer: 'Rentas de este cliente',
      noneThatDay: 'Sin rentas ese día.',
      inactive: 'Esta renta ya no está activa.',
      status: {
        pending_deposit: 'Apartada · falta anticipo',
        confirmed: 'Agendada · por salir',
        in_progress: 'En proceso',
        ready: 'Lista',
        out_for_delivery: 'En camino',
        checked_out: 'Rentada · con el cliente',
        overdue: 'Vencida · no ha regresado',
        returned: 'Devuelta · por cerrar',
        completed: 'Cerrada',
        cancelled: 'Cancelada',
        expired: 'Expirada · no se pagó el anticipo',
        no_show: 'No se presentó',
      },
    },
    home: {
      today: 'Lo que sale y regresa hoy',
      calendar: 'Inventario y rentas por fecha',
      customers: 'Fichas, documentos y aval',
      history: 'Todas las rentas, con búsqueda y filtros',
      catalog: 'Herramientas, precios y unidades',
      settings: 'Depósito, retrasos, revisión y documentos',
    },
    newBooking: {
      itemHint: 'Elige qué se va a rentar y cuántas piezas.',
      customerHint: 'Busca o registra al cliente. Su dirección, aval y documentos se guardan en su ficha.',
      paymentHint: 'Por lo general se cobra al registrar la salida. Registra un pago aquí solo si el cliente deja algo ahora.',
    },
    catalog: { activeHint: 'Si la desactivas ya no aparece al registrar rentas; las rentas existentes no cambian.' },
  },

  nails: {
    bookings: {
      plural: 'Citas',
      searchPlaceholder: 'Cliente, teléfono o servicio',
      empty: 'No hay citas con estos filtros.',
      ofCustomer: 'Citas de esta clienta',
      noneThatDay: 'Sin citas ese día.',
      inactive: 'Esta cita ya no está activa.',
      status: {
        pending_deposit: 'Apartada · falta anticipo',
        confirmed: 'Confirmada · por atender',
        in_progress: 'En atención',
        ready: 'Lista',
        out_for_delivery: 'En camino',
        checked_out: 'En curso',
        overdue: 'Atrasada',
        returned: 'Terminada',
        completed: 'Atendida',
        cancelled: 'Cancelada',
        expired: 'Expirada · no se pagó el anticipo',
        no_show: 'No asistió',
      },
    },
    home: {
      today: 'Las citas de hoy',
      calendar: 'Citas por día',
      customers: 'Fichas y teléfonos',
      history: 'Todas las citas, con búsqueda y filtros',
      catalog: 'Servicios, precios y duración',
      settings: 'Anticipo y apartados',
    },
    newBooking: {
      itemHint: 'Elige el servicio.',
      customerHint: 'Busca o registra a la clienta; su teléfono queda guardado para la próxima.',
      paymentHint: 'Si pides anticipo, regístralo aquí para confirmar la cita.',
    },
    catalog: { activeHint: 'Si lo desactivas ya no aparece al agendar citas; las citas existentes no cambian.' },
  },

  flowers: {
    bookings: {
      plural: 'Pedidos',
      searchPlaceholder: 'Cliente, teléfono o arreglo',
      empty: 'No hay pedidos con estos filtros.',
      ofCustomer: 'Pedidos de este cliente',
      noneThatDay: 'Sin pedidos ese día.',
      inactive: 'Este pedido ya no está activo.',
      status: {
        pending_deposit: 'Apartado · falta anticipo',
        confirmed: 'Confirmado · por preparar',
        in_progress: 'En preparación',
        ready: 'Listo para entregar',
        out_for_delivery: 'En camino',
        checked_out: 'Entregado',
        overdue: 'Atrasado',
        returned: 'Devuelto',
        completed: 'Entregado',
        cancelled: 'Cancelado',
        expired: 'Expirado · no se pagó el anticipo',
        no_show: 'No lo recogieron',
      },
    },
    home: {
      today: 'Lo que se prepara y entrega hoy',
      calendar: 'Pedidos por día de entrega',
      customers: 'Fichas, teléfonos y direcciones',
      history: 'Todos los pedidos, con búsqueda y filtros',
      catalog: 'Arreglos, precios y anticipación',
      settings: 'Anticipo, apartados y zonas de entrega',
    },
    newBooking: {
      itemHint: 'Elige el arreglo y cuántos.',
      customerHint: 'Busca o registra al cliente; su teléfono queda guardado para el próximo pedido.',
      paymentHint: 'Si pides anticipo, regístralo aquí para confirmar el pedido.',
    },
    catalog: { activeHint: 'Si lo desactivas ya no aparece al tomar pedidos; los pedidos existentes no cambian.' },
  },
};

/** The texts of a type of business. */
export function textsFor(vertical: Vertical): VerticalTexts {
  return TEXTS[vertical];
}

// ---------------------------------------------------------------- Ajustes

/**
 * Sections of Ajustes. "general" is shared by every business; the rest belong to one type.
 * - general: advance (% of the total) and how long a hold waits for it
 * - rentals: security deposit, late days and closed days
 * - inspection: time a unit is held after coming back
 * - documents: extra documents asked at check-out
 * - delivery: delivery zones and their fee
 */
export type SettingsSection = 'general' | 'rentals' | 'inspection' | 'documents' | 'delivery';

const SETTINGS_SECTIONS: Record<Vertical, SettingsSection[]> = {
  tool_rental: ['general', 'rentals', 'inspection', 'documents'],
  nails: ['general'],
  flowers: ['general', 'delivery'],
};

/** The Ajustes sections a type of business sees, in order. */
export function settingsSectionsFor(vertical: Vertical): SettingsSection[] {
  return SETTINGS_SECTIONS[vertical];
}
