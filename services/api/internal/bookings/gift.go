package bookings

import (
	"slices"
	"strings"
	"unicode/utf8"
)

// giftDetails: who receives the order and the card message (feature giftOrder, e.g. florerías);
// all optional, dropped when the business doesn't have the feature.
func giftDetails(in *CreateInput, active []string) error {
	if !slices.Contains(active, "giftOrder") {
		in.RecipientName, in.RecipientPhone, in.CardMessage = "", "", ""
		return nil
	}
	in.RecipientName = strings.TrimSpace(in.RecipientName)
	in.CardMessage = strings.TrimSpace(in.CardMessage)
	if utf8.RuneCountInString(in.RecipientName) > 80 {
		return Invalid{"recipient_name", "El nombre de quien recibe es muy largo."}
	}
	if raw := strings.TrimSpace(in.RecipientPhone); raw != "" {
		if in.RecipientPhone = NormalizePhone(raw); in.RecipientPhone == "" {
			return Invalid{"recipient_phone", "Escribe un teléfono válido de 10 dígitos."}
		}
	}
	if utf8.RuneCountInString(in.CardMessage) > 500 {
		return Invalid{"card_message", "La dedicatoria es muy larga (máximo 500 caracteres)."}
	}
	return nil
}

// customPrice: an item with a free price (an «arreglo personalizado») takes the price and the
// description written in the order (feature customOrder). A fixed-price item ignores both.
func customPrice(in *CreateInput, it *itemRow, active []string) error {
	if !it.CustomPrice {
		in.CustomPrice, in.CustomDescription = nil, ""
		return nil
	}
	if !slices.Contains(active, "customOrder") {
		return Invalid{"item_id", "Este negocio no toma pedidos personalizados."}
	}
	if in.CustomPrice == nil || *in.CustomPrice < 1 || *in.CustomPrice > maxPayment {
		return Invalid{"custom_price", "Escribe el precio, entre $0.01 y $1,000,000."}
	}
	in.CustomDescription = strings.TrimSpace(in.CustomDescription)
	if n := utf8.RuneCountInString(in.CustomDescription); n == 0 || n > 500 {
		return Invalid{"custom_description", "Describe el pedido (máximo 500 caracteres)."}
	}
	it.Price = *in.CustomPrice
	return nil
}
