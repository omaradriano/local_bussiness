// Package products is the retail side of a business: products with price, cost and stock; sales
// (alone or during a booking) that take stock out; purchases of merchandise that put it back and
// count as an expense; and the period's stats (revenue, cost of what was sold, profit).
package products

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/omaradriano/local_bussiness/services/api/internal/store"
)

var ErrNotFound = errors.New("not found")

// Invalid: field -> message (es-MX).
type Invalid map[string]string

func (e Invalid) Error() string { return "invalid" }

const maxMoney = 100_000_000_00

type Product struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Price  int64  `json:"price"`
	Cost   *int64 `json:"cost"` // owner only (nil for staff)
	Stock  int    `json:"stock"`
	Active bool   `json:"active"`
}

type ProductInput struct {
	Name   *string `json:"name"`
	Price  *int64  `json:"price"`
	Cost   *int64  `json:"cost"`
	Stock  *int    `json:"stock"` // only when creating (afterwards stock moves with sales and purchases)
	Active *bool   `json:"active"`
}

func (p Product) Apply(in ProductInput, creating bool) Product {
	if in.Name != nil {
		p.Name = strings.Join(strings.Fields(*in.Name), " ")
	}
	if in.Price != nil {
		p.Price = *in.Price
	}
	if in.Cost != nil {
		c := *in.Cost
		p.Cost = &c
	}
	if in.Stock != nil && creating {
		p.Stock = *in.Stock
	}
	if in.Active != nil {
		p.Active = *in.Active
	}
	return p
}

func (p Product) Validate() error {
	errs := Invalid{}
	if n := utf8.RuneCountInString(p.Name); n == 0 || n > 80 {
		errs["name"] = "Escribe el nombre del producto."
	}
	if p.Price < 0 || p.Price > maxMoney {
		errs["price"] = "El precio no es válido."
	}
	if p.Cost != nil && (*p.Cost < 0 || *p.Cost > maxMoney) {
		errs["cost"] = "El costo no es válido."
	}
	if p.Stock < 0 || p.Stock > 1_000_000 {
		errs["stock"] = "Las existencias no son válidas."
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// Line of a sale as asked: a product and how many.
type LineInput struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

// Line of a sale as recorded: prices and costs frozen at the moment of the sale.
type Line struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	UnitPrice int64  `json:"unit_price"`
	UnitCost  int64  `json:"-"`
}

// PriceSale checks a sale against the products (exist, active, enough stock) and returns its lines
// and totals. Repeated products are added together.
func PriceSale(in []LineInput, catalog map[string]Product, costs map[string]int64) ([]Line, int64, int64, error) {
	if len(in) == 0 || len(in) > 50 {
		return nil, 0, 0, Invalid{"lines": "Agrega de 1 a 50 productos."}
	}
	qty := map[string]int{}
	var order []string
	for _, l := range in {
		if l.Quantity < 1 || l.Quantity > 1000 {
			return nil, 0, 0, Invalid{"lines": "La cantidad debe ser de 1 a 1000."}
		}
		if _, seen := qty[l.ProductID]; !seen {
			order = append(order, l.ProductID)
		}
		qty[l.ProductID] += l.Quantity
	}
	var lines []Line
	var total, cost int64
	for _, id := range order {
		p, ok := catalog[id]
		if !ok || !p.Active {
			return nil, 0, 0, Invalid{"lines": "Un producto no existe o está inactivo."}
		}
		if qty[id] > p.Stock {
			return nil, 0, 0, Invalid{"lines": fmt.Sprintf("Solo quedan %d de %s.", p.Stock, p.Name)}
		}
		lines = append(lines, Line{ProductID: id, Name: p.Name, Quantity: qty[id], UnitPrice: p.Price, UnitCost: costs[id]})
		total += p.Price * int64(qty[id])
		cost += costs[id] * int64(qty[id])
	}
	return lines, total, cost, nil
}

type Sale struct {
	ID        string    `json:"id"`
	BookingID *string   `json:"booking_id"`
	SoldAt    time.Time `json:"sold_at"`
	Method    string    `json:"method"`
	Total     int64     `json:"total"`
	Lines     []Line    `json:"lines"`
}

type Repo struct{ DB store.DB }

func scanProducts(rows pgx.Rows, withCost bool) ([]Product, map[string]int64, error) {
	defer rows.Close()
	out := []Product{}
	costs := map[string]int64{}
	for rows.Next() {
		var p Product
		var cost int64
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &cost, &p.Stock, &p.Active); err != nil {
			return nil, nil, err
		}
		costs[p.ID] = cost
		if withCost {
			c := cost
			p.Cost = &c
		}
		out = append(out, p)
	}
	return out, costs, rows.Err()
}

const productCols = `id::text, name, price, cost, stock, active`

// Products lists the business's products; the cost only for the owner.
func (r Repo) Products(ctx context.Context, businessID string, withCost bool) ([]Product, error) {
	rows, err := r.DB.Query(ctx, `select `+productCols+` from public.retail_products where business_id = $1 order by active desc, name`, businessID)
	if err != nil {
		return nil, err
	}
	out, _, err := scanProducts(rows, withCost)
	return out, err
}

func (r Repo) Product(ctx context.Context, businessID, id string) (Product, error) {
	rows, err := r.DB.Query(ctx, `select `+productCols+` from public.retail_products where business_id = $1 and id = $2`, businessID, id)
	if err != nil {
		return Product{}, err
	}
	list, _, err := scanProducts(rows, true)
	if err == nil && len(list) == 0 {
		return Product{}, ErrNotFound
	}
	if err != nil {
		return Product{}, err
	}
	return list[0], nil
}

func (r Repo) Create(ctx context.Context, businessID string, p Product) (Product, error) {
	var cost int64
	if p.Cost != nil {
		cost = *p.Cost
	}
	err := r.DB.QueryRow(ctx, `insert into public.retail_products (business_id, name, price, cost, stock, active)
		values ($1, $2, $3, $4, $5, true) returning id::text, active`, businessID, p.Name, p.Price, cost, p.Stock).Scan(&p.ID, &p.Active)
	p.Cost = &cost
	return p, err
}

func (r Repo) Update(ctx context.Context, businessID string, p Product) (Product, error) {
	tag, err := r.DB.Exec(ctx, `update public.retail_products set name = $3, price = $4, cost = $5, active = $6
		where business_id = $1 and id = $2`, businessID, p.ID, p.Name, p.Price, *p.Cost, p.Active)
	if err == nil && tag.RowsAffected() == 0 {
		return p, ErrNotFound
	}
	return p, err
}

// Purchase adds stock (and updates the product's cost to the new unit cost); it counts as an
// expense («Mercancía») on its date.
func (r Repo) Purchase(ctx context.Context, businessID, productID string, quantity int, unitCost int64, on time.Time, actor string) (Product, error) {
	if quantity < 1 || quantity > 100000 {
		return Product{}, Invalid{"quantity": "La cantidad debe ser de 1 a 100,000."}
	}
	if unitCost < 0 || unitCost > maxMoney {
		return Product{}, Invalid{"unit_cost": "El costo no es válido."}
	}
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return Product{}, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `update public.retail_products set stock = stock + $3, cost = $4 where business_id = $1 and id = $2`,
		businessID, productID, quantity, unitCost)
	if err != nil {
		return Product{}, err
	}
	if tag.RowsAffected() == 0 {
		return Product{}, ErrNotFound
	}
	if _, err := tx.Exec(ctx, `insert into public.stock_purchases (business_id, product_id, quantity, unit_cost, purchased_on, created_by)
		values ($1, $2, $3, $4, $5, nullif($6, '')::uuid)`, businessID, productID, quantity, unitCost, on, actor); err != nil {
		return Product{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Product{}, err
	}
	return r.Product(ctx, businessID, productID)
}

// Sell records a sale and takes the stock out, all or nothing.
func (r Repo) Sell(ctx context.Context, businessID string, in []LineInput, method, bookingID, actor string, now time.Time) (Sale, error) {
	if !slices.Contains([]string{"cash", "transfer", "card"}, method) {
		return Sale{}, Invalid{"method": "Elige efectivo, transferencia o tarjeta."}
	}
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return Sale{}, err
	}
	defer tx.Rollback(ctx)
	if bookingID != "" {
		var ok bool
		if err := tx.QueryRow(ctx, `select exists (select 1 from public.bookings where business_id = $1 and id::text = $2)`, businessID, bookingID).Scan(&ok); err != nil {
			return Sale{}, err
		}
		if !ok {
			return Sale{}, Invalid{"booking_id": "La reserva no existe."}
		}
	}
	ids := make([]string, 0, len(in))
	for _, l := range in {
		ids = append(ids, l.ProductID)
	}
	// Lock the products sold so two sales can't take the same last piece.
	rows, err := tx.Query(ctx, `select `+productCols+` from public.retail_products where business_id = $1 and id::text = any($2) for update`, businessID, ids)
	if err != nil {
		return Sale{}, err
	}
	list, costs, err := scanProducts(rows, false)
	if err != nil {
		return Sale{}, err
	}
	catalog := map[string]Product{}
	for _, p := range list {
		catalog[p.ID] = p
	}
	lines, total, cost, err := PriceSale(in, catalog, costs)
	if err != nil {
		return Sale{}, err
	}
	sale := Sale{Method: method, Total: total, Lines: lines, SoldAt: now}
	if bookingID != "" {
		sale.BookingID = &bookingID
	}
	if err := tx.QueryRow(ctx, `insert into public.sales (business_id, booking_id, sold_at, method, total, cost_total, created_by)
		values ($1, nullif($2, '')::uuid, $3, $4, $5, $6, nullif($7, '')::uuid) returning id::text`,
		businessID, bookingID, now, method, total, cost, actor).Scan(&sale.ID); err != nil {
		return Sale{}, err
	}
	for _, l := range lines {
		if _, err := tx.Exec(ctx, `insert into public.sale_lines (business_id, sale_id, product_id, quantity, unit_price, unit_cost)
			values ($1, $2, $3, $4, $5, $6)`, businessID, sale.ID, l.ProductID, l.Quantity, l.UnitPrice, l.UnitCost); err != nil {
			return Sale{}, err
		}
		if _, err := tx.Exec(ctx, `update public.retail_products set stock = stock - $3 where business_id = $1 and id = $2`, businessID, l.ProductID, l.Quantity); err != nil {
			return Sale{}, err
		}
	}
	return sale, tx.Commit(ctx)
}

// CancelSale deletes a sale (a mistake) and puts its stock back.
func (r Repo) CancelSale(ctx context.Context, businessID, saleID string) error {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `update public.retail_products p set stock = p.stock + l.quantity
		from public.sale_lines l where l.sale_id = $2 and l.business_id = $1 and p.id = l.product_id`, businessID, saleID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `delete from public.sales where business_id = $1 and id = $2`, businessID, saleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

// Sales of [start, end), newest first, with their lines.
func (r Repo) Sales(ctx context.Context, businessID string, start, end time.Time) ([]Sale, error) {
	rows, err := r.DB.Query(ctx, `select s.id::text, s.booking_id::text, s.sold_at, s.method, s.total,
		coalesce(jsonb_agg(jsonb_build_object('product_id', l.product_id, 'name', p.name, 'quantity', l.quantity, 'unit_price', l.unit_price)
		         order by p.name) filter (where l.id is not null), '[]')
		from public.sales s left join public.sale_lines l on l.sale_id = s.id left join public.retail_products p on p.id = l.product_id
		where s.business_id = $1 and s.sold_at >= $2 and s.sold_at < $3
		group by s.id order by s.sold_at desc limit 500`, businessID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Sale{}
	for rows.Next() {
		var s Sale
		if err := rows.Scan(&s.ID, &s.BookingID, &s.SoldAt, &s.Method, &s.Total, &s.Lines); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ProductStat: how one product did in the period.
type ProductStat struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	Units     int    `json:"units"`
	Revenue   int64  `json:"revenue"`
	Cost      int64  `json:"cost"`
	Profit    int64  `json:"profit"`
}

type Stats struct {
	Revenue    int64         `json:"revenue"`   // what the sales brought in
	Cost       int64         `json:"cost"`      // cost of what was sold
	Profit     int64         `json:"profit"`    // revenue − cost
	Purchases  int64         `json:"purchases"` // merchandise bought in the period
	Units      int           `json:"units"`
	Sales      int           `json:"sales"`
	ByProduct  []ProductStat `json:"by_product"`  // best sellers first
	LowStock   []Product     `json:"low_stock"`   // active with 3 or fewer left
	StockValue int64         `json:"stock_value"` // what the stock on hand cost
}

// StatsFor [start, end) (instants) and the purchases between the local dates [from, to].
func (r Repo) StatsFor(ctx context.Context, businessID string, start, end, from, to time.Time) (Stats, error) {
	st := Stats{ByProduct: []ProductStat{}, LowStock: []Product{}}
	rows, err := r.DB.Query(ctx, `select p.id::text, p.name, sum(l.quantity)::int, sum(l.quantity * l.unit_price), sum(l.quantity * l.unit_cost)
		from public.sale_lines l join public.sales s on s.id = l.sale_id join public.retail_products p on p.id = l.product_id
		where s.business_id = $1 and s.sold_at >= $2 and s.sold_at < $3
		group by p.id, p.name order by 4 desc, 2`, businessID, start, end)
	if err != nil {
		return st, err
	}
	for rows.Next() {
		var ps ProductStat
		if err := rows.Scan(&ps.ProductID, &ps.Name, &ps.Units, &ps.Revenue, &ps.Cost); err != nil {
			rows.Close()
			return st, err
		}
		ps.Profit = ps.Revenue - ps.Cost
		st.ByProduct = append(st.ByProduct, ps)
		st.Revenue += ps.Revenue
		st.Cost += ps.Cost
		st.Units += ps.Units
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return st, err
	}
	st.Profit = st.Revenue - st.Cost
	if err := r.DB.QueryRow(ctx, `select count(*) from public.sales where business_id = $1 and sold_at >= $2 and sold_at < $3`,
		businessID, start, end).Scan(&st.Sales); err != nil {
		return st, err
	}
	if err := r.DB.QueryRow(ctx, `select coalesce(sum(quantity * unit_cost), 0) from public.stock_purchases
		where business_id = $1 and purchased_on between $2 and $3`, businessID, from, to).Scan(&st.Purchases); err != nil {
		return st, err
	}
	if err := r.DB.QueryRow(ctx, `select coalesce(sum(stock * cost), 0) from public.retail_products where business_id = $1 and active`,
		businessID).Scan(&st.StockValue); err != nil {
		return st, err
	}
	all, err := r.Products(ctx, businessID, true)
	if err != nil {
		return st, err
	}
	for _, p := range all {
		if p.Active && p.Stock <= 3 {
			st.LowStock = append(st.LowStock, p)
		}
	}
	return st, nil
}

// Bounds turns the local dates [from, to] into instants [start, end) in the business's time zone.
func (r Repo) Bounds(ctx context.Context, businessID string, from, to time.Time) (time.Time, time.Time, error) {
	var tz string
	if err := r.DB.QueryRow(ctx, `select timezone from public.businesses where id = $1`, businessID).Scan(&tz); err != nil {
		return time.Time{}, time.Time{}, err
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	start := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, loc)
	return start, time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1), nil
}
