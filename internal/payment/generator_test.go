package payment

import (
	"context"
	"testing"
	"time"
)

type fakeGeneratorStore struct {
	invoice  InvoiceForPayment
	found    bool
	newEach  bool
	pending  *Transaction
	customer CustomerForPayment
	expired  string
	synced   bool
	upserted bool
}

func (s *fakeGeneratorStore) PaymentInvoice(context.Context, string) (InvoiceForPayment, bool, error) { return s.invoice, s.found, nil }
func (s *fakeGeneratorStore) NewPIXPerAccess(context.Context) (bool, error) { return s.newEach, nil }
func (s *fakeGeneratorStore) LatestPendingTransaction(context.Context, string) (*Transaction, error) { return s.pending, nil }
func (s *fakeGeneratorStore) ExpireTransaction(_ context.Context, id string) error { s.expired = id; return nil }
func (s *fakeGeneratorStore) PaymentCustomer(context.Context, string) (CustomerForPayment, error) { return s.customer, nil }
func (s *fakeGeneratorStore) SyncInvoicePIX(context.Context, string, Transaction, int) error { s.synced = true; return nil }
func (s *fakeGeneratorStore) UpsertPendingPayment(context.Context, InvoiceForPayment, Transaction, float64) error { s.upserted = true; return nil }

type fakeCreator struct { calls int; tx *Transaction; err error }
func (c *fakeCreator) CreatePIX(context.Context, CreateRequest) (*Transaction, error) { c.calls++; return c.tx, c.err }

func TestGeneratorUsesExactDiscountAmountInCents(t *testing.T) {
	copyPaste := "000201pix"
	gwid := "gw-1"
	store := &fakeGeneratorStore{found: true, newEach: true, invoice: InvoiceForPayment{ID: "inv", CustomerID: "cust", Description: "Fatura", DiscountAmount: 149.90, Status: "em_aberto"}, customer: CustomerForPayment{Name: "A", Phone: "93999999999"}}
	creator := &fakeCreator{tx: &Transaction{ID: "tx", GatewaySlug: "cashinpay", GatewayTransactionID: &gwid, AmountCents: 14990, CopyPaste: &copyPaste, Status: "pendente"}}
	g := NewGenerator(store, creator)
	got, err := g.Generate(context.Background(), GenerateInput{InvoiceID: "inv", RequestKey: "key", BaseURL: "https://x"})
	if err != nil { t.Fatal(err) }
	if got.Value != 149.90 || got.CopyPaste != copyPaste || !got.Available { t.Fatalf("got=%+v", got) }
	if creator.calls != 1 || !store.synced || !store.upserted { t.Fatalf("calls=%d synced=%v upserted=%v", creator.calls, store.synced, store.upserted) }
}

func TestGeneratorReusesValidPendingPIXWhenConfigured(t *testing.T) {
	copyPaste := "000201old"
	expiry := time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)
	store := &fakeGeneratorStore{found: true, newEach: false, invoice: InvoiceForPayment{ID: "inv", CustomerID: "cust", DiscountAmount: 10, Status: "em_aberto"}, pending: &Transaction{ID: "old", GatewaySlug: "cash", AmountCents: 1000, CopyPaste: &copyPaste, ExpiresAt: &expiry, Status: "pendente"}}
	creator := &fakeCreator{}
	g := NewGenerator(store, creator)
	got, err := g.Generate(context.Background(), GenerateInput{InvoiceID: "inv", RequestKey: "key"})
	if err != nil { t.Fatal(err) }
	if creator.calls != 0 || got.TransactionID != "old" || got.CopyPaste != copyPaste { t.Fatalf("calls=%d got=%+v", creator.calls, got) }
}

func TestGeneratorExpiresOldPIXAndCreatesNewOne(t *testing.T) {
	oldCopy := "000201old"
	oldExpiry := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
	newCopy := "000201new"
	store := &fakeGeneratorStore{found: true, newEach: false, invoice: InvoiceForPayment{ID: "inv", CustomerID: "cust", DiscountAmount: 10, Status: "em_aberto"}, pending: &Transaction{ID: "old", AmountCents: 1000, CopyPaste: &oldCopy, ExpiresAt: &oldExpiry, Status: "pendente"}}
	creator := &fakeCreator{tx: &Transaction{ID: "new", GatewaySlug: "cash", AmountCents: 1000, CopyPaste: &newCopy, Status: "pendente"}}
	g := NewGenerator(store, creator)
	got, err := g.Generate(context.Background(), GenerateInput{InvoiceID: "inv", RequestKey: "key"})
	if err != nil { t.Fatal(err) }
	if store.expired != "old" || creator.calls != 1 || got.TransactionID != "new" { t.Fatalf("expired=%s calls=%d got=%+v", store.expired, creator.calls, got) }
}

func TestGeneratorPaidInvoiceDoesNotCreatePIX(t *testing.T) {
	store := &fakeGeneratorStore{found: true, invoice: InvoiceForPayment{ID: "inv", Status: "paga"}}
	creator := &fakeCreator{}
	got, err := NewGenerator(store, creator).Generate(context.Background(), GenerateInput{InvoiceID: "inv"})
	if err != nil { t.Fatal(err) }
	if got.Available || got.Status != "paga" || creator.calls != 0 { t.Fatalf("got=%+v calls=%d", got, creator.calls) }
}
