package admin

import (
	"context"
	"testing"
	"time"
)

type activityStoreStub struct {
	tx          []TransactionRow
	payments    []PaymentRow
	webhookIDs  map[string]struct{}
	paymentLogs []PaymentLog
	webhookLogs []WebhookLog
	details     []TransactionLookup
	gotStatus   string
	gotLimit    int
	gotGateway  string
}

func (s *activityStoreStub) TransactionRows(_ context.Context, status string, limit int) ([]TransactionRow, error) {
	s.gotStatus = status
	s.gotLimit = limit
	return s.tx, nil
}
func (s *activityStoreStub) PaymentRows(_ context.Context, gateway string, _ time.Time, _ int) ([]PaymentRow, error) {
	s.gotGateway = gateway
	return s.payments, nil
}
func (s *activityStoreStub) WebhookTransactionIDs(context.Context, []string) (map[string]struct{}, error) {
	return s.webhookIDs, nil
}
func (s *activityStoreStub) PaymentLogs(context.Context, int) ([]PaymentLog, error) {
	return s.paymentLogs, nil
}
func (s *activityStoreStub) WebhookLogs(context.Context, int) ([]WebhookLog, error) {
	return s.webhookLogs, nil
}
func (s *activityStoreStub) TransactionsByGatewayIDs(context.Context, []string) ([]TransactionLookup, error) {
	return s.details, nil
}
func str(v string) *string { return &v }

func TestTransactionsGroupedKeepsLatestPerClientAndCountsAttempts(t *testing.T) {
	client := "c1"
	store := &activityStoreStub{tx: []TransactionRow{
		{ID: "new", InvoiceID: "f1", ClientID: &client, Status: "pendente", CreatedAt: "2026-09-01T10:00:00Z"},
		{ID: "old", InvoiceID: "f1", ClientID: &client, Status: "falhou", CreatedAt: "2026-09-01T09:00:00Z"},
		{ID: "other", InvoiceID: "f2", Status: "pago", CreatedAt: "2026-09-01T08:00:00Z"},
	}}
	got, err := NewActivityService(store).Transactions(context.Background(), "todos", true, 100)
	if err != nil {
		t.Fatal(err)
	}
	if store.gotStatus != "" || store.gotLimit != 500 {
		t.Fatalf("query status=%q limit=%d", store.gotStatus, store.gotLimit)
	}
	if len(got) != 2 || got[0].ID != "new" || got[0].Attempts != 2 || got[1].ID != "other" {
		t.Fatalf("got=%+v", got)
	}
}

func TestTransactionsHistoryFiltersInStore(t *testing.T) {
	store := &activityStoreStub{}
	_, err := NewActivityService(store).Transactions(context.Background(), "pago", false, 100)
	if err != nil {
		t.Fatal(err)
	}
	if store.gotStatus != "pago" || store.gotLimit != 100 {
		t.Fatalf("status=%q limit=%d", store.gotStatus, store.gotLimit)
	}
}

func TestPaymentsPreservesWebhookVsConsultation(t *testing.T) {
	client := struct {
		Name  string `json:"nome"`
		Phone string `json:"telefone"`
	}{Name: "Ana", Phone: "93999999999"}
	inv := struct {
		Description string  `json:"descricao"`
		Status      string  `json:"status"`
		Discount    float64 `json:"valor_desconto"`
	}{Description: "Fatura", Status: "paga", Discount: 49.9}
	paid := 4990
	store := &activityStoreStub{payments: []PaymentRow{{ID: "1", GatewaySlug: "propix", GatewayTransactionID: str("gw1"), AmountCents: 5000, PaidAmountCents: &paid, PaidAt: "2026-09-01T10:00:00Z", Client: &client, Invoice: &inv}}, webhookIDs: map[string]struct{}{"gw1": {}}}
	svc := NewActivityService(store)
	svc.now = func() time.Time { return time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC) }
	got, err := svc.Payments(context.Background(), "todas", 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ConfirmedBy != "webhook" || got[0].AmountCents != 4990 || got[0].InvoiceAmountCents == nil || *got[0].InvoiceAmountCents != 4990 {
		t.Fatalf("got=%+v", got)
	}
}

func TestLogsRecognizesWebhookByGatewayTransactionID(t *testing.T) {
	client := struct {
		Name  string `json:"nome"`
		Phone string `json:"telefone"`
	}{Name: "Ana", Phone: "93999999999"}
	store := &activityStoreStub{webhookLogs: []WebhookLog{{ID: "w", Gateway: "propix", GatewayTransactionID: str("gw1"), SignatureValid: true}}, details: []TransactionLookup{{GatewayTransactionID: "gw1", AmountCents: 1234, Status: "pago", Client: &client}}}
	got, err := NewActivityService(store).Logs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Webhooks) != 1 || !got.Webhooks[0].Recognized || got.Webhooks[0].AmountCents == nil || *got.Webhooks[0].AmountCents != 1234 || got.Webhooks[0].ClientName == nil || *got.Webhooks[0].ClientName != "Ana" {
		t.Fatalf("got=%+v", got.Webhooks)
	}
}
