package invoice

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Store interface {
	Select(ctx context.Context, table string, query url.Values, dst any) error
	Insert(ctx context.Context, table string, body any) error
}

type Service struct {
	store Store
	now   func() time.Time
}

func New(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

type Customer struct {
	ID       string `json:"id"`
	Name     string `json:"nome"`
	Phone    string `json:"telefone"`
	Email    string `json:"email,omitempty"`
	Document string `json:"documento,omitempty"`
}

type Invoice struct {
	ID             string  `json:"id"`
	Description    string  `json:"descricao"`
	Reference      string  `json:"referencia"`
	OriginalAmount float64 `json:"valor_original"`
	DiscountAmount float64 `json:"valor_desconto"`
	DueDate        string  `json:"vencimento"`
	Status         string  `json:"status"`
}

type QueryResult struct {
	Found    bool      `json:"encontrado"`
	Customer *Customer `json:"cliente,omitempty"`
	Invoices []Invoice `json:"faturas,omitempty"`
}

type customerRow struct {
	ID    string  `json:"id"`
	Name  string  `json:"nome"`
	Phone *string `json:"telefone"`
}

type invoiceRow struct {
	ID             string   `json:"id"`
	Description    *string  `json:"descricao"`
	Reference      *string  `json:"referencia"`
	OriginalAmount float64  `json:"valor_original"`
	DiscountAmount *float64 `json:"valor_desconto"`
	DueDate        string   `json:"vencimento"`
	Status         string   `json:"status"`
}

func (s *Service) QueryByPhone(ctx context.Context, phone string) (QueryResult, error) {
	phone = Digits(phone)
	if len(phone) < 10 || len(phone) > 11 {
		return QueryResult{}, fmt.Errorf("telefone inválido")
	}

	variants := PhoneVariants(phone)
	var customers []customerRow
	q := url.Values{}
	q.Set("select", "id,nome,telefone")
	q.Set("telefone", "in.("+strings.Join(variants, ",")+")")
	q.Set("limit", "1")
	if err := s.store.Select(ctx, "clientes", q, &customers); err != nil {
		return QueryResult{}, fmt.Errorf("não foi possível consultar no momento: %w", err)
	}
	if len(customers) == 0 {
		s.recordAccess(ctx, phone, false, nil, nil)
		return QueryResult{Found: false}, nil
	}

	customer := customers[0]
	start, end := MonthBoundsUTC(s.now())
	var rows []invoiceRow
	q = url.Values{}
	q.Set("select", "id,descricao,referencia,valor_original,valor_desconto,vencimento,status")
	q.Set("cliente_id", "eq."+customer.ID)
	q.Set("status", "in.(em_aberto,vencida,em_processamento,falhou,expirada)")
	q.Set("vencimento", "gte."+start+"&vencimento=lte."+end)
	q.Del("vencimento")
	q.Add("vencimento", "gte."+start)
	q.Add("vencimento", "lte."+end)
	q.Set("order", "vencimento.desc")
	q.Set("limit", "1")
	if err := s.store.Select(ctx, "faturas", q, &rows); err != nil {
		return QueryResult{}, fmt.Errorf("não foi possível consultar no momento: %w", err)
	}

	var invoices []Invoice
	var original, discount *float64
	if len(rows) > 0 {
		r := rows[0]
		description, reference := "", ""
		if r.Description != nil { description = *r.Description }
		if r.Reference != nil { reference = *r.Reference }
		discountAmount := r.OriginalAmount
		if r.DiscountAmount != nil && *r.DiscountAmount != 0 { discountAmount = *r.DiscountAmount }
		invoices = []Invoice{{
			ID: r.ID, Description: description, Reference: reference,
			OriginalAmount: r.OriginalAmount, DiscountAmount: valueOrZero(r.DiscountAmount),
			DueDate: r.DueDate, Status: r.Status,
		}}
		original, discount = &r.OriginalAmount, &discountAmount
	}
	s.recordAccess(ctx, phone, len(rows) > 0, original, discount)

	customerPhone := ""
	if customer.Phone != nil { customerPhone = *customer.Phone }
	return QueryResult{
		Found: true,
		Customer: &Customer{ID: customer.ID, Name: customer.Name, Phone: customerPhone},
		Invoices: invoices,
	}, nil
}

func (s *Service) recordAccess(ctx context.Context, phone string, success bool, original, discount *float64) {
	_ = s.store.Insert(ctx, "acessos", map[string]any{
		"pagina": "/fatura", "telefone_consultado": phone, "sucesso": success,
		"valor_original": original, "valor_desconto": discount,
	})
}

func Digits(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' { b.WriteRune(r) }
	}
	return b.String()
}

func PhoneVariants(phone string) []string {
	set := map[string]struct{}{phone: {}, "55" + phone: {}}
	if len(phone) == 11 && phone[2] == '9' {
		set[phone[:2]+phone[3:]] = struct{}{}
	}
	if len(phone) == 10 {
		set[phone[:2]+"9"+phone[2:]] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for v := range set { out = append(out, v) }
	sort.Strings(out)
	return out
}

func MonthBoundsUTC(now time.Time) (string, string) {
	u := now.UTC()
	start := time.Date(u.Year(), u.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0).AddDate(0, 0, -1)
	return start.Format("2006-01-02"), end.Format("2006-01-02")
}

func valueOrZero(v *float64) float64 {
	if v == nil { return 0 }
	return *v
}
