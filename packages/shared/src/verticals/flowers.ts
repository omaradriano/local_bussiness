import type { VerticalMap } from './types';

// Florería bajo pedido: products with a delivery date. Texts and Ajustes done; detail flow and builder pending.
export const flowers = {
  label: 'Florería bajo pedido',
  kind: 'product',
  terms: { booking: 'Pedido', catalog_item: 'Arreglo', catalog: 'Arreglos', starts_at: 'Fecha de entrega', ends_at: null },
  settings: ['general', 'delivery'],
  features: [],
  optionalFeatures: ['walkIn', 'productSales'],
  texts: {
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
} satisfies VerticalMap;
