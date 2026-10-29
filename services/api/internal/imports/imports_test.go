package imports

import (
	"testing"
	"time"

	"github.com/omaradriano/local_bussiness/services/api/internal/catalog"
	"github.com/omaradriano/local_bussiness/services/api/internal/customers"
	"github.com/omaradriano/local_bussiness/services/api/internal/products"
)

func s(v string) *string { return &v }
func i64(v int64) *int64 { return &v }
func n(v int) *int       { return &v }

var now = time.Date(2026, 10, 29, 18, 0, 0, 0, time.UTC)

func TestPlanCustomers(t *testing.T) {
	existing := map[string]customers.Customer{"+526141234567": {ID: "c1", Name: "Ana", Phone: "+526141234567"}}
	rows := []customers.Input{
		{Name: s("Ana López"), Phone: s("614 123 4567"), Email: s("ana@correo.mx")}, // same phone: update
		{Name: s("Luis"), Phone: s("6149998877")},                                   // new
		{Name: s("Luis bis"), Phone: s("+52 614 999 8877")},                         // repeated in the file
		{Name: s(""), Phone: s("6140001111")},                                       // invalid
	}
	got := PlanCustomers(rows, existing, customers.Rules{}, now)
	want := []string{Update, Create, Fail, Fail}
	for i, p := range got {
		if p.Result.Action != want[i] {
			t.Fatalf("row %d: %+v (want %s)", i+2, p.Result, want[i])
		}
	}
	if got[0].Customer.ID != "c1" || got[0].Customer.Name != "Ana López" || got[0].Result.Row != 2 || got[2].Result.Errors["phone"] == "" {
		t.Fatalf("plan: %+v / %+v", got[0], got[2].Result)
	}
	// The giro's rules apply: a rental business needs the address.
	if p := PlanCustomers([]customers.Input{{Name: s("Eva"), Phone: s("6145550000")}}, nil, customers.Rules{RequireAddress: true}, now); p[0].Result.Errors["address"] == "" {
		t.Fatalf("address rule: %+v", p[0].Result)
	}
}

func TestPlanCatalog(t *testing.T) {
	existing := []catalog.Item{{ID: "i1", Kind: "service", Name: "Corte", Price: 15000, DurationMinutes: n(30), Active: true}}
	rows := []CatalogRow{
		{Name: " corte ", Price: i64(18000)},                       // update by name, any case
		{Name: "Barba", Price: i64(10000), DurationMinutes: n(20)}, // new
		{Name: "Tinte", Price: i64(30000), DurationMinutes: n(60)}, // over the plan's limit
		{Name: "Sin precio"}, // invalid
	}
	limit := 2 // 1 active + 1 new
	got := PlanCatalog(rows, existing, "service", &limit, "lite")
	want := []string{Update, Create, Fail, Fail}
	for i, p := range got {
		if p.Result.Action != want[i] {
			t.Fatalf("row %d: %+v (want %s)", i+2, p.Result, want[i])
		}
	}
	if got[0].Item.Price != 18000 || got[0].Item.Name != "Corte" || got[0].Item.DurationMinutes == nil || got[1].Item.Kind != "service" || got[2].Result.Errors["plan"] == "" {
		t.Fatalf("plan: %+v %+v %+v", got[0].Item, got[1].Item, got[2].Result)
	}
}

func TestPlanProducts(t *testing.T) {
	cost := int64(7000)
	existing := []products.Product{{ID: "p1", Name: "Cera", Price: 15000, Cost: &cost, Stock: 3, Active: true}}
	rows := []products.ProductInput{
		{Name: s("CERA"), Price: i64(16000), Stock: n(50)}, // update: price yes, stock no (it moves with sales)
		{Name: s("Shampoo"), Price: i64(12000), Cost: i64(6000), Stock: n(10)},
		{Name: s("Gel"), Price: i64(-1)},
	}
	got := PlanProducts(rows, existing)
	if got[0].Result.Action != Update || got[0].Product.Price != 16000 || got[0].Product.Stock != 3 || got[0].Product.Name != "Cera" ||
		got[1].Result.Action != Create || got[1].Product.Stock != 10 || !got[1].Product.Active || got[2].Result.Action != Fail {
		t.Fatalf("plan: %+v %+v %+v", got[0], got[1], got[2].Result)
	}
}
