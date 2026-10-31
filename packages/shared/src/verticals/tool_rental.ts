import type { VerticalMap } from './types';

// Renta de herramienta: the first complete map (requirements from the first client, Oct 2026).
export const tool_rental = {
  label: 'Renta de herramienta',
  kind: 'rental',
  terms: { booking: 'Renta', catalog_item: 'Herramienta', catalog: 'Herramientas', starts_at: 'Fecha de salida', ends_at: 'Fecha de regreso' },
  settings: ['general', 'rentals', 'inspection', 'documents'],
  features: ['inventory', 'guarantor', 'customerAddress', 'customerDocuments', 'trustedCustomers', 'bookingDocuments', 'productSales', 'customFields'],
  optionalFeatures: ['staff'],
  texts: {
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
} satisfies VerticalMap;
