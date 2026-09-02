package admin

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrInvalidInvoice = errors.New("fatura inválida")

type InvoiceRow struct {
	ID           string  `json:"id"`
	ClientID     string  `json:"cliente_id"`
	Description  string  `json:"descricao"`
	Original     float64 `json:"valor_original"`
	Discount     float64 `json:"valor_desconto"`
	DueDate      string  `json:"vencimento"`
	Status       string  `json:"status"`
	PIXCopyPaste *string `json:"pix_copia_cola"`
	Barcode      *string `json:"boleto_codigo"`
	Client       struct {
		Name  string `json:"nome"`
		Phone string `json:"telefone"`
	} `json:"clientes"`
}

type InvoicePage struct {
	Rows  []InvoiceRow `json:"linhas"`
	Total int          `json:"total"`
}
type InvoiceEdit struct {
	Name     string  `json:"nome"`
	Phone    string  `json:"telefone"`
	Original float64 `json:"valor_original"`
	Discount float64 `json:"valor_desconto"`
	DueDate  string  `json:"vencimento"`
	Status   string  `json:"status"`
}
type ManualPaymentInvoice struct {
	ID       string
	ClientID string
	Original float64
	Discount float64
}
type DeleteAllResult struct {
	Payments int `json:"pagamentos"`
	Invoices int `json:"faturas"`
	Clients  int `json:"clientes"`
}

type InvoiceAdminStore interface {
	ListInvoices(context.Context, string, int, int) (InvoicePage, error)
	UpdateClient(context.Context, string, string, string) error
	UpdateInvoice(context.Context, string, InvoiceEdit) error
	ManualPaymentInvoice(context.Context, string) (ManualPaymentInvoice, bool, error)
	InsertManualPayment(context.Context, ManualPaymentInvoice, time.Time) error
	SetInvoiceStatus(context.Context, string, string) error
	DeleteAll(context.Context) (DeleteAllResult, error)
}

type InvoiceService struct {
	store InvoiceAdminStore
	now   func() time.Time
}

func NewInvoiceService(store InvoiceAdminStore) *InvoiceService {
	return &InvoiceService{store: store, now: time.Now}
}
func (s *InvoiceService) List(ctx context.Context, search string, page int) (InvoicePage, error) {
	if page < 0 {
		page = 0
	}
	return s.store.ListInvoices(ctx, strings.TrimSpace(search), page, 50)
}
func (s *InvoiceService) Save(ctx context.Context, id, clientID string, in InvoiceEdit) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Phone = digits(in.Phone)
	in.Status = strings.TrimSpace(in.Status)
	if id == "" || clientID == "" || len(in.Phone) < 10 || len(in.Phone) > 15 || !dateOnly(in.DueDate) || in.Original < 0 || in.Discount < 0 || !validStatuses[in.Status] {
		return ErrInvalidInvoice
	}
	if in.Name == "" {
		in.Name = in.Phone
	}
	if err := s.store.UpdateClient(ctx, clientID, in.Name, in.Phone); err != nil {
		return err
	}
	return s.store.UpdateInvoice(ctx, id, in)
}
func (s *InvoiceService) SetStatus(ctx context.Context, id, status string) error {
	if id == "" || !validStatuses[status] {
		return ErrInvalidInvoice
	}
	if err := s.store.SetInvoiceStatus(ctx, id, status); err != nil {
		return err
	}
	if status != "paga" {
		return nil
	}
	inv, ok, err := s.store.ManualPaymentInvoice(ctx, id)
	if err != nil || !ok {
		return err
	}
	return s.store.InsertManualPayment(ctx, inv, s.now())
}
func (s *InvoiceService) DeleteAll(ctx context.Context, confirmation string) (DeleteAllResult, error) {
	if confirmation != "APAGAR" {
		return DeleteAllResult{}, ErrInvalidInvoice
	}
	return s.store.DeleteAll(ctx)
}
func digits(v string) string {
	var b strings.Builder
	for _, r := range v {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
