package admin

import (
	"context"
	"errors"
	"strings"
)

var ErrInvalidImport = errors.New("importação inválida")

var pendingStatuses = []string{"em_aberto", "vencida", "expirada", "falhou", "em_processamento"}
var validStatuses = map[string]bool{"em_aberto": true, "paga": true, "vencida": true, "cancelada": true, "expirada": true, "falhou": true, "em_processamento": true}

type ImportRow struct {
	Name     string   `json:"nome"`
	Phone    string   `json:"telefone"`
	Email    *string  `json:"email"`
	Document *string  `json:"documento"`
	Notes    *string  `json:"observacoes"`
	Original *float64 `json:"valor_original"`
	Discount *float64 `json:"valor_desconto"`
	Status   *string  `json:"status"`
}

type ImportInput struct {
	Rows    []ImportRow `json:"clientes"`
	DueDate string      `json:"vencimento_global"`
}

type ImportResult struct {
	Imported        int      `json:"importados"`
	InvoicesCreated int      `json:"faturasCriadas"`
	InvoicesUpdated int      `json:"faturasAtualizadas"`
	Rejected        []string `json:"rejeitados"`
}

type ClientUpsert struct {
	Name     string  `json:"nome"`
	Phone    string  `json:"telefone"`
	Email    *string `json:"email"`
	Document *string `json:"documento"`
	Notes    *string `json:"observacoes"`
}

type ClientRef struct {
	ID    string `json:"id"`
	Phone string `json:"telefone"`
}
type PendingInvoice struct {
	ID       string `json:"id"`
	ClientID string `json:"cliente_id"`
	DueDate  string `json:"vencimento"`
}
type InvoiceWrite struct {
	ID          string  `json:"id,omitempty"`
	ClientID    string  `json:"cliente_id"`
	Description string  `json:"descricao"`
	Original    float64 `json:"valor_original"`
	Discount    float64 `json:"valor_desconto"`
	DueDate     string  `json:"vencimento"`
	Status      string  `json:"status"`
}

type ImportStore interface {
	UpsertClients(context.Context, []ClientUpsert) ([]ClientRef, error)
	PendingInvoices(context.Context, []string, []string) ([]PendingInvoice, error)
	UpsertInvoices(context.Context, []InvoiceWrite) (int, error)
	InsertInvoices(context.Context, []InvoiceWrite) (int, error)
}

type Importer struct{ store ImportStore }

func NewImporter(store ImportStore) *Importer { return &Importer{store: store} }

func (s *Importer) Import(ctx context.Context, in ImportInput) (ImportResult, error) {
	if len(in.Rows) < 1 || len(in.Rows) > 500 || !dateOnly(in.DueDate) {
		return ImportResult{}, ErrInvalidImport
	}
	type normalized struct {
		client             ClientUpsert
		original, discount float64
		status             string
	}
	seen := map[string]struct{}{}
	rows := make([]normalized, 0, len(in.Rows))
	result := ImportResult{Rejected: []string{}}
	for _, r := range in.Rows {
		phone := normalizePhone(r.Phone)
		if len(phone) < 10 || len(phone) > 11 {
			result.Rejected = append(result.Rejected, r.Phone)
			continue
		}
		if _, ok := seen[phone]; ok {
			result.Rejected = append(result.Rejected, r.Phone)
			continue
		}
		seen[phone] = struct{}{}
		original, discount := numberOrZero(r.Original), numberOrZero(r.Discount)
		if original < 0 || discount < 0 {
			return ImportResult{}, ErrInvalidImport
		}
		if discount > original {
			original = discount
		}
		status := "em_aberto"
		if r.Status != nil && strings.TrimSpace(*r.Status) != "" {
			status = strings.TrimSpace(*r.Status)
			if !validStatuses[status] {
				return ImportResult{}, ErrInvalidImport
			}
		}
		name := strings.TrimSpace(r.Name)
		if name == "" {
			name = phone
		}
		rows = append(rows, normalized{client: ClientUpsert{Name: name, Phone: phone, Email: trimPtr(r.Email), Document: trimPtr(r.Document), Notes: trimPtr(r.Notes)}, original: original, discount: discount, status: status})
	}
	if len(rows) == 0 {
		return result, nil
	}
	clientsIn := make([]ClientUpsert, len(rows))
	for i := range rows {
		clientsIn[i] = rows[i].client
	}
	saved, err := s.store.UpsertClients(ctx, clientsIn)
	if err != nil {
		return ImportResult{}, err
	}
	result.Imported = len(saved)
	ids := make([]string, 0, len(saved))
	byPhone := map[string]string{}
	for _, c := range saved {
		ids = append(ids, c.ID)
		byPhone[c.Phone] = c.ID
	}
	pending, err := s.store.PendingInvoices(ctx, ids, pendingStatuses)
	if err != nil {
		return ImportResult{}, err
	}
	latest := map[string]string{}
	for _, p := range pending {
		if _, ok := latest[p.ClientID]; !ok {
			latest[p.ClientID] = p.ID
		}
	}
	existing := make([]InvoiceWrite, 0)
	fresh := make([]InvoiceWrite, 0)
	for _, r := range rows {
		id, ok := byPhone[r.client.Phone]
		if !ok {
			result.Rejected = append(result.Rejected, r.client.Phone)
			continue
		}
		w := InvoiceWrite{ClientID: id, Description: "Fatura importada", Original: r.original, Discount: r.discount, DueDate: in.DueDate, Status: r.status}
		if invoiceID, ok := latest[id]; ok {
			w.ID = invoiceID
			existing = append(existing, w)
		} else {
			fresh = append(fresh, w)
		}
	}
	if len(existing) > 0 {
		result.InvoicesUpdated, err = s.store.UpsertInvoices(ctx, existing)
		if err != nil {
			return ImportResult{}, err
		}
	}
	if len(fresh) > 0 {
		result.InvoicesCreated, err = s.store.InsertInvoices(ctx, fresh)
		if err != nil {
			return ImportResult{}, err
		}
	}
	return result, nil
}

func normalizePhone(v string) string {
	var b strings.Builder
	for _, r := range v {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	d := b.String()
	if (len(d) == 12 || len(d) == 13) && strings.HasPrefix(d, "55") {
		d = d[2:]
	}
	for len(d) > 11 && strings.HasPrefix(d, "0") {
		d = d[1:]
	}
	return d
}
func numberOrZero(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
func trimPtr(v *string) *string {
	if v == nil {
		return nil
	}
	x := strings.TrimSpace(*v)
	if x == "" {
		return nil
	}
	return &x
}
func dateOnly(v string) bool {
	if len(v) != 10 || v[4] != '-' || v[7] != '-' {
		return false
	}
	for i, r := range v {
		if i == 4 || i == 7 {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
