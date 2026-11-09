package products

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPriceSale(t *testing.T) {
	prices := Prices{
		Products: map[string]Product{
			"cera":   {ID: "cera", Name: "Cera", Price: 15000, Stock: 5, Active: true},
			"navaja": {ID: "navaja", Name: "Navaja", Price: 8000, Stock: 1, Active: true},
			"viejo":  {ID: "viejo", Name: "Viejo", Price: 100, Stock: 9, Active: false},
		},
		Costs:  map[string]int64{"cera": 7000, "navaja": 3000},
		Retail: true,
	}
	prod := func(id string, q int) LineInput { return LineInput{ProductID: id, Quantity: q} }
	lines, total, cost, err := PriceSale([]LineInput{prod("cera", 1), prod("navaja", 1), prod("cera", 1)}, prices)
	if err != nil || len(lines) != 2 || lines[0].Quantity != 2 || total != 2*15000+8000 || cost != 2*7000+3000 {
		t.Fatalf("sale: %+v %d %d %v", lines, total, cost, err)
	}
	cases := []struct {
		name string
		in   []LineInput
	}{
		{"sin productos", nil},
		{"más de lo que hay", []LineInput{prod("navaja", 2)}},
		{"suma repetidos sobre existencias", []LineInput{prod("navaja", 1), prod("navaja", 1)}},
		{"inactivo", []LineInput{prod("viejo", 1)}},
		{"no existe", []LineInput{prod("x", 1)}},
		{"cantidad cero", []LineInput{prod("cera", 0)}},
		{"venta rápida apagada: artículo", []LineInput{{ItemID: "ramo", Quantity: 1}}},
		{"venta rápida apagada: importe libre", []LineInput{{Name: "Girasoles", Price: ptr64(35000), Quantity: 1}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var bad Invalid
			if _, _, _, err := PriceSale(tc.in, prices); !errors.As(err, &bad) {
				t.Fatalf("want Invalid, got %v", err)
			}
		})
	}
}

// Quick sale (feature quickSale): catalog items and free amounts, with or without stock products.
func TestPriceQuickSale(t *testing.T) {
	prices := Prices{
		Products: map[string]Product{"globo": {ID: "globo", Name: "Globo", Price: 5000, Stock: 3, Active: true}},
		Costs:    map[string]int64{"globo": 2000},
		Items: map[string]Item{
			"ramo":    {ID: "ramo", Name: "Ramo de 12 rosas", Price: 65000, Active: true},
			"libre":   {ID: "libre", Name: "Arreglo personalizado", CustomPrice: true, Active: true},
			"apagado": {ID: "apagado", Name: "Viejo", Price: 100},
		},
		Quick: true,
	}
	lines, total, cost, err := PriceSale([]LineInput{
		{ItemID: "ramo", Quantity: 2, Price: ptr64(1)}, // a fixed price ignores what's sent
		{ItemID: "libre", Name: " 24 rosas rojas ", Price: ptr64(120000), Quantity: 1},
		{Name: " Ramo de girasoles ", Price: ptr64(35000), Quantity: 1},
	}, prices)
	if err != nil || len(lines) != 3 || total != 2*65000+120000+35000 || cost != 0 {
		t.Fatalf("quick sale: %+v %d %d %v", lines, total, cost, err)
	}
	if *lines[0].ItemID != "ramo" || lines[0].Name != "Ramo de 12 rosas" || lines[0].UnitPrice != 65000 ||
		lines[1].Name != "Arreglo personalizado · 24 rosas rojas" || lines[1].UnitPrice != 120000 ||
		lines[2].ItemID != nil || lines[2].ProductID != nil || lines[2].Name != "Ramo de girasoles" {
		t.Fatalf("lines: %+v", lines)
	}
	cases := []struct {
		name string
		in   LineInput
	}{
		{"productos sin la función", LineInput{ProductID: "globo", Quantity: 1}},
		{"artículo inactivo", LineInput{ItemID: "apagado", Quantity: 1}},
		{"artículo que no existe", LineInput{ItemID: "x", Quantity: 1}},
		{"precio libre sin precio", LineInput{ItemID: "libre", Name: "Rosas", Quantity: 1}},
		{"precio libre sin descripción", LineInput{ItemID: "libre", Price: ptr64(1000), Quantity: 1}},
		{"importe libre sin nombre", LineInput{Name: " ", Price: ptr64(1000), Quantity: 1}},
		{"importe libre sin precio", LineInput{Name: "Girasoles", Quantity: 1}},
		{"importe libre en cero", LineInput{Name: "Girasoles", Price: ptr64(0), Quantity: 1}},
		{"producto y artículo a la vez", LineInput{ProductID: "globo", ItemID: "ramo", Quantity: 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var bad Invalid
			if _, _, _, err := PriceSale([]LineInput{tc.in}, prices); !errors.As(err, &bad) {
				t.Fatalf("want Invalid, got %v", err)
			}
		})
	}
	both := prices
	both.Retail = true
	if _, total, cost, err := PriceSale([]LineInput{{ProductID: "globo", Quantity: 2}, {ItemID: "ramo", Quantity: 1}}, both); err != nil || total != 10000+65000 || cost != 4000 {
		t.Fatalf("mixed: %d %d %v", total, cost, err)
	}
}

func TestSaleCustomer(t *testing.T) {
	name, phone, err := SaleCustomer(" Doña Mari ", "614 123 4567")
	if err != nil || name != "Doña Mari" || phone != "+526141234567" {
		t.Fatalf("customer: %q %q %v", name, phone, err)
	}
	if name, phone, err := SaleCustomer("", ""); err != nil || name != "" || phone != "" {
		t.Fatalf("anonymous: %q %q %v", name, phone, err)
	}
	var bad Invalid
	if _, _, err := SaleCustomer("", "123"); !errors.As(err, &bad) {
		t.Fatalf("bad phone: %v", err)
	}
}

func TestProductInput(t *testing.T) {
	cost, stock := int64(5000), 10
	p := Product{}.Apply(ProductInput{Name: ptr("  Cera   mate "), Price: ptr64(15000), Cost: &cost, Stock: &stock}, true)
	if p.Name != "Cera mate" || p.Stock != 10 || *p.Cost != 5000 || p.Validate() != nil {
		t.Fatalf("create: %+v", p)
	}
	// Stock only moves with sales and purchases once the product exists.
	if q := p.Apply(ProductInput{Stock: &stock}, false); q.Stock != 10 {
		t.Fatal("stock set on edit")
	}
	if (Product{Name: ""}).Validate() == nil {
		t.Fatal("empty name accepted")
	}
}

func ptr(s string) *string { return &s }
func ptr64(v int64) *int64 { return &v }

// Only an order's extras can stay on its account (checked before touching the database).
func TestOnAccountNeedsAnOrder(t *testing.T) {
	_, err := Repo{}.Sell(context.Background(), "b", SaleInput{Method: OnAccount}, "", time.Time{})
	if bad, ok := err.(Invalid); !ok || bad["method"] == "" {
		t.Fatalf("on account without an order: %v", err)
	}
}
