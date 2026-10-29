package settings

import (
	"encoding/json"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/omaradriano/local_bussiness/services/api/internal/privacy"
)

// Privacy: who is responsible for the customers' data in the business's privacy notice. Empty
// fields fall back to the business's name and WhatsApp (see NoticeFrom).
type Privacy struct {
	LegalName string `json:"privacy_legal_name"` // name or company name of the responsible
	Address   string `json:"privacy_address"`
	Email     string `json:"privacy_email"` // where customers send ARCO requests
}

type PrivacyPatch struct {
	LegalName *string `json:"privacy_legal_name"`
	Address   *string `json:"privacy_address"`
	Email     *string `json:"privacy_email"`
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func (p Privacy) Apply(in PrivacyPatch) Privacy {
	if in.LegalName != nil {
		p.LegalName = oneLine(*in.LegalName)
	}
	if in.Address != nil {
		p.Address = oneLine(*in.Address)
	}
	if in.Email != nil {
		p.Email = strings.ToLower(strings.TrimSpace(*in.Email))
	}
	return p
}

func (p Privacy) Validate() map[string]string {
	errs := map[string]string{}
	if utf8.RuneCountInString(p.LegalName) > 120 {
		errs["privacy_legal_name"] = "El nombre o razón social es muy largo (máximo 120)."
	}
	if utf8.RuneCountInString(p.Address) > 300 {
		errs["privacy_address"] = "El domicilio es muy largo (máximo 300)."
	}
	if p.Email != "" {
		if a, err := mail.ParseAddress(p.Email); err != nil || a.Address != p.Email || len(p.Email) > 120 {
			errs["privacy_email"] = "Escribe un correo válido."
		}
	}
	return errs
}

// Notice: what the business's public privacy notice shows.
type Notice struct {
	Name      string `json:"name"`       // business name
	LegalName string `json:"legal_name"` // responsible (the business name if not captured)
	Address   string `json:"address"`
	Email     string `json:"email"`
	Phone     string `json:"phone"` // WhatsApp, also a contact for ARCO requests
	Version   string `json:"version"`
}

func NoticeFrom(name string, phone *string, raw []byte) Notice {
	var p Privacy
	_ = json.Unmarshal(raw, &p)
	n := Notice{Name: name, LegalName: p.LegalName, Address: p.Address, Email: p.Email, Version: privacy.Version}
	if n.LegalName == "" {
		n.LegalName = name
	}
	if phone != nil {
		n.Phone = *phone
	}
	return n
}
