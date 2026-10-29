import type { VerticalMap } from './types';

// Uñas y belleza: agenda by person. Texts and Ajustes done; detail flow and its own features pending.
export const nails = {
  label: 'Uñas y belleza',
  kind: 'service',
  terms: { booking: 'Cita', catalog_item: 'Servicio', catalog: 'Servicios', starts_at: 'Hora de la cita', ends_at: 'Hora de término' },
  settings: ['general', 'agenda'],
  features: ['staff', 'startService'],
  optionalFeatures: ['walkIn', 'homeService', 'productSales'],
  texts: {
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
      settings: 'Anticipo, margen entre citas y funciones',
    },
    newBooking: {
      itemHint: 'Elige el servicio.',
      customerHint: 'Busca o registra a la clienta; su teléfono queda guardado para la próxima.',
      paymentHint: 'Si pides anticipo, regístralo aquí para confirmar la cita.',
    },
    catalog: { activeHint: 'Si lo desactivas ya no aparece al agendar citas; las citas existentes no cambian.' },
  },
} satisfies VerticalMap;
