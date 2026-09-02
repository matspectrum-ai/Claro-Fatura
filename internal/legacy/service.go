package legacy

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
	"github.com/matspectrum-ai/Claro-Fatura/internal/payment"
)

var (
	ErrNotFound      = errors.New("não encontrado")
	ErrNoPending     = errors.New("nenhuma fatura pendente")
	ErrNotPending    = errors.New("fatura não pendente")
	ErrInvalidAmount = errors.New("valor com desconto inválido")
	ErrChargeFailed  = errors.New("não foi possível gerar cobrança")
)

type InvoiceView struct {
	Phone          string  `json:"telefone"`
	Name           string  `json:"nome"`
	InvoiceID      string  `json:"fatura_id"`
	OpenAmount     float64 `json:"valor_em_aberto"`
	DiscountAmount float64 `json:"valor_com_desconto"`
	Status         string  `json:"status"`
	DueDate        string  `json:"data_vencimento"`
	PIXCopyPaste   *string `json:"pix_copia_e_cola"`
	Barcode        *string `json:"boleto_codigo"`
	BarcodeURL     *string `json:"boleto_url"`
	PaidAt         *string `json:"data_pagamento"`
}

type Client struct {
	ID       string
	Name     string
	Phone    string
	Email    *string
	Document *string
}

type Invoice struct {
	ID             string
	CustomerID     string
	DiscountAmount float64
	OriginalAmount float64
	DueDate        string
	Status         string
}

type OpenInvoice struct {
	ID             string
	CustomerID     string
	DiscountAmount float64
}

type Store interface {
	LegacyInvoiceViewByPhone(context.Context, string) (InvoiceView, bool, error)
	LegacyClientByPhone(context.Context, string) (Client, bool, error)
	LegacyClientByID(context.Context, string) (Client, bool, error)
	LegacyPendingInvoiceID(context.Context, string) (string, bool, error)
	LegacyInvoiceByID(context.Context, string) (Invoice, bool, error)
	LegacyFirstOpenInvoice(context.Context) (OpenInvoice, bool, error)
	LegacyProPixExists(context.Context) (bool, error)
	LegacyInsertProPix(context.Context) error
}

type Router interface {
	CreatePIX(context.Context, payment.CreateRequest) (*payment.Transaction, error)
}

type DirectProPix interface {
	CreatePIX(context.Context, gateway.CreateInput) (gateway.CreatedPIX, error)
}

type Service struct {
	store      Store
	router     Router
	propix     DirectProPix
	baseURL    string
	env        func(string) string
	now        func() time.Time
	randomUUID func() (string, error)
}

func New(store Store, router Router, propix DirectProPix, baseURL string) *Service {
	return &Service{
		store: store, router: router, propix: propix, baseURL: strings.TrimRight(baseURL, "/"),
		env: os.Getenv, now: time.Now, randomUUID: uuidV4,
	}
}

func (s *Service) InvoiceByPhone(ctx context.Context, phone string) (InvoiceView, bool, error) {
	return s.store.LegacyInvoiceViewByPhone(ctx, phone)
}

type ChargeResult struct {
	InvoiceID     string  `json:"fatura_id"`
	Amount        float64 `json:"valor_cobrado"`
	DueDate       string  `json:"data_vencimento"`
	PIXCopyPaste  string  `json:"pix_copia_e_cola"`
	TransactionID *string `json:"transaction_id"`
	Gateway       string  `json:"gateway"`
}

func (s *Service) CreateCharge(ctx context.Context, phone, invoiceID, requestBaseURL string) (ChargeResult, error) {
	name := "Cliente"
	if invoiceID == "" {
		client, found, err := s.store.LegacyClientByPhone(ctx, phone)
		if err != nil { return ChargeResult{}, err }
		if !found { return ChargeResult{}, ErrNotFound }
		name, phone = client.Name, client.Phone
		id, found, err := s.store.LegacyPendingInvoiceID(ctx, client.ID)
		if err != nil { return ChargeResult{}, err }
		if !found { return ChargeResult{}, ErrNoPending }
		invoiceID = id
	}
	invoice, found, err := s.store.LegacyInvoiceByID(ctx, invoiceID)
	if err != nil { return ChargeResult{}, err }
	if !found { return ChargeResult{}, ErrNotFound }
	if invoice.Status != "em_aberto" && invoice.Status != "vencida" { return ChargeResult{}, ErrNotPending }
	client, _, err := s.store.LegacyClientByID(ctx, invoice.CustomerID)
	if err != nil { return ChargeResult{}, err }
	if client.Name != "" { name = client.Name }
	if client.Phone != "" { phone = client.Phone }
	value := invoice.DiscountAmount
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) { return ChargeResult{}, ErrInvalidAmount }
	key, err := s.randomUUID(); if err != nil { return ChargeResult{}, err }
	base := strings.TrimRight(requestBaseURL, "/"); if base == "" { base = s.baseURL }
	tx, err := s.router.CreatePIX(ctx, payment.CreateRequest{
		InvoiceID: invoice.ID, CustomerID: &invoice.CustomerID, AmountCents: int(math.Round(value * 100)),
		Name: name, Phone: phone, Email: client.Email, Document: client.Document,
		Description: "Fatura", RequestKey: key, BaseURL: base,
	})
	if err != nil { return ChargeResult{}, err }
	if tx == nil || tx.CopyPaste == nil || strings.TrimSpace(*tx.CopyPaste) == "" { return ChargeResult{}, ErrChargeFailed }
	return ChargeResult{InvoiceID: invoice.ID, Amount: value, DueDate: invoice.DueDate, PIXCopyPaste: *tx.CopyPaste, TransactionID: tx.GatewayTransactionID, Gateway: tx.GatewaySlug}, nil
}

func (s *Service) SetupProPix(ctx context.Context) (map[string]any, error) {
	exists, err := s.store.LegacyProPixExists(ctx); if err != nil { return nil, err }
	if exists { return map[string]any{"success": true, "message": "ProPix já existe no banco."}, nil }
	if err := s.store.LegacyInsertProPix(ctx); err != nil { return nil, err }
	return map[string]any{"success": true, "message": "ProPix inserido com sucesso."}, nil
}

func (s *Service) Secrets() map[string]bool {
	return map[string]bool{
		"PROPIX_CLIENT_ID": s.env("PROPIX_CLIENT_ID") != "",
		"PROPIX_CLIENT_SECRET": s.env("PROPIX_CLIENT_SECRET") != "",
		"CASHINPAY_SECRET_KEY": s.env("CASHINPAY_SECRET_KEY") != "",
	}
}

func (s *Service) TestFlow(ctx context.Context) map[string]any {
	invoice, found, err := s.store.LegacyFirstOpenInvoice(ctx)
	if err != nil { return map[string]any{"success": false, "error": err.Error()} }
	if !found { return map[string]any{"success": false, "error": "Nenhuma fatura em aberto encontrada para teste."} }
	client, found, err := s.store.LegacyClientByID(ctx, invoice.CustomerID)
	if err != nil { return map[string]any{"success": false, "error": err.Error()} }
	if !found { return map[string]any{"success": false, "error": "Cliente da fatura não encontrado."} }
	tx, err := s.router.CreatePIX(ctx, payment.CreateRequest{
		InvoiceID: invoice.ID, CustomerID: &invoice.CustomerID, AmountCents: int(math.Round(invoice.DiscountAmount * 100)),
		Name: client.Name, Phone: client.Phone, Email: client.Email, Document: client.Document,
		Description: "Teste ProPix", BaseURL: "http://localhost:8080", RequestKey: fmt.Sprintf("TESTE-ROUTER-%d", s.now().UnixMilli()),
	})
	if err != nil { return map[string]any{"success": false, "error": err.Error()} }
	return map[string]any{"success": true, "result": tx}
}

func (s *Service) TestProPixDirect(ctx context.Context) map[string]any {
	if s.propix == nil { return map[string]any{"success": false, "error": "ProPix indisponível"} }
	created, err := s.propix.CreatePIX(ctx, gateway.CreateInput{
		AmountCents: 100, Name: "Teste Diagnostico", Phone: "11999999999",
		Document: strptr("12345678909"), Description: "Teste de Diagnostico ProPix",
		Reference: fmt.Sprintf("DIAG-%d", s.now().UnixMilli()),
	})
	if err != nil { return map[string]any{"success": false, "error": err.Error()} }
	return map[string]any{"success": true, "result": map[string]any{
		"id": created.TransactionID, "copia_cola": created.CopyPaste, "qrcode": created.QRCode, "status": created.Status,
	}}
}

func strptr(v string) *string { return &v }

func uuidV4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil { return "", err }
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
