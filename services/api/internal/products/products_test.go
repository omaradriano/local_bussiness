package products

import (
	"errors"
	"testing"
)

func TestPriceSale(t *testing.T) {
	catalog := map[string]Product{
		"cera":   {ID: "cera", Name: "Cera", Price: 15000, Stock: 5, Active: true},
		"navaja": {ID: "navaja", Name: "Navaja", Price: 8000, Stock: 1, Active: true},
		"viejo":  {ID: "viejo", Name: "Viejo", Price: 100, Stock: 9, Active: false},
	}
	costs := map[string]int64{"cera": 7000, "navaja": 3000}
	lines, total, cost, err := PriceSale([]LineInput{{"cera", 1}, {"navaja", 1}, {"cera", 1}}, catalog, costs)
	if err != nil || len(lines) != 2 || lines[0].Quantity != 2 || total != 2*15000+8000 || cost != 2*7000+3000 {
		t.Fatalf("sale: %+v %d %d %v", lines, total, cost, err)
	}
	cases := []struct {
		name string
		in   []LineInput
	}{
		{"sin productos", nil},
		{"más de lo que hay", []LineInput{{"navaja", 2}}},
		{"suma repetidos sobre existencias", []LineInput{{"navaja", 1}, {"navaja", 1}}},
		{"inactivo", []LineInput{{"viejo", 1}}},
		{"no existe", []LineInput{{"x", 1}}},
		{"cantidad cero", []LineInput{{"cera", 0}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var bad Invalid
			if _, _, _, err := PriceSale(tc.in, catalog, costs); !errors.As(err, &bad) {
				t.Fatalf("want Invalid, got %v", err)
			}
		})
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
