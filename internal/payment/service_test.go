package payment

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
)

type fakeAdapter struct {
	name       string
	configured bool
	createErr  error
	creates    atomic.Int32
	created    gateway.CreatedPIX
}

func (f *fakeAdapter) Name() string                    { return f.name }
func (f *fakeAdapter) Configured(gateway.Record) bool { return f.configured }
func (f *fakeAdapter) CreatePIX(context.Context, gateway.CreateInput) (gateway.CreatedPIX, error) {
	f.creates.Add(1)
	if f.createErr != nil {
		return gateway.CreatedPIX{}, f.createErr
	}
	return f.created, nil
}
func (f *fakeAdapter) Status(context.Context, string, gateway.Record) (*string, error) { return nil, nil }
func (f *fakeAdapter) Paid(*string) bool                                                 { return false }
func (f *fakeAdapter) ReadWebhook(*http.Request, []byte, gateway.Record) (gateway.WebhookRead, error) {
	return gateway.WebhookRead{}, nil
}

type fakeRegistry map[string]gateway.Adapter

func (r fakeRegistry) AdapterFor(g gateway.Record) gateway.Adapter { return r[g.Adapter] }

type fakeStore struct {
	mu            sync.Mutex
	requests      map[string]RequestReservation
	transactions  map[string]Transaction
	invoiceStatus string
	gateways      []gateway.Record
	routing       RoutingConfig
	pointer       int
	logs          []LogEntry
	nextID        int
}

func newFakeStore() *fakeStore {
	return &fakeStore{requests: map[string]RequestReservation{}, transactions: map[string]Transaction{}, invoiceStatus: "em_aberto", routing: RoutingConfig{Strategy: "prioridade"}}
}

func (s *fakeStore) ReserveRequest(_ context.Context, key, _ string) (RequestReservation, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.requests[key]; ok {
		return v, false, nil
	}
	s.nextID++
	v := RequestReservation{ID: "req-" + itoa(s.nextID), Status: "processando"}
	s.requests[key] = v
	return v, true, nil
}
func (s *fakeStore) RequestTransaction(_ context.Context, r RequestReservation) (*Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Status != "concluida" || r.TransactionID == nil {
		return nil, nil
	}
	t, ok := s.transactions[*r.TransactionID]
	if !ok {
		return nil, nil
	}
	copy := t
	return &copy, nil
}
func (s *fakeStore) CompleteRequest(_ context.Context, id, txID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, v := range s.requests {
		if v.ID == id {
			v.Status = "concluida"
			v.TransactionID = &txID
			s.requests[key] = v
		}
	}
	return nil
}
func (s *fakeStore) FailRequest(_ context.Context, id, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, v := range s.requests {
		if v.ID == id {
			v.Status = "falhou"
			s.requests[key] = v
		}
	}
	return nil
}
func (s *fakeStore) InvoiceStatus(context.Context, string) (string, bool, error) { return s.invoiceStatus, true, nil }
func (s *fakeStore) ActiveGateways(context.Context) ([]gateway.Record, error) { return append([]gateway.Record(nil), s.gateways...), nil }
func (s *fakeStore) Routing(context.Context) (RoutingConfig, error) { return s.routing, nil }
func (s *fakeStore) AdvanceGatewayPointer(context.Context, int) (int, error) { s.pointer++; return s.pointer, nil }
func (s *fakeStore) GatewayTransactionsSince(context.Context, string, time.Time) (int, error) { return 0, nil }
func (s *fakeStore) InsertTransaction(_ context.Context, in TransactionCreate) (Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	copyPaste := in.CopyPaste
	t := Transaction{ID: "tx-" + itoa(s.nextID), InvoiceID: in.InvoiceID, CustomerID: in.CustomerID, GatewaySlug: in.GatewaySlug, GatewayID: in.GatewayID, GatewayTransactionID: in.GatewayTransactionID, AmountCents: in.AmountCents, CopyPaste: &copyPaste, QRCode: in.QRCode, Status: in.Status, ExpiresAt: &in.ExpiresAt}
	s.transactions[t.ID] = t
	return t, nil
}
func (s *fakeStore) ReplaceOtherPending(context.Context, string, string, time.Time) error { return nil }
func (s *fakeStore) Log(_ context.Context, e LogEntry) error { s.mu.Lock(); defer s.mu.Unlock(); s.logs = append(s.logs, e); return nil }

func TestCreatePIXPriorityFailover(t *testing.T) {
	store := newFakeStore()
	store.gateways = []gateway.Record{{ID: "g1", Slug: "first", Adapter: "first", Priority: 1}, {ID: "g2", Slug: "second", Adapter: "second", Priority: 2}}
	first := &fakeAdapter{name: "first", configured: true, createErr: errors.New("down")}
	second := &fakeAdapter{name: "second", configured: true, created: gateway.CreatedPIX{TransactionID: "gw-2", CopyPaste: "000201pix", Status: "pending"}}
	svc := New(store, fakeRegistry{"first": first, "second": second}, "Ebook Viver de Vendas")
	tx, err := svc.CreatePIX(context.Background(), CreateRequest{InvoiceID: "inv-1", AmountCents: 12990, RequestKey: "req-key", BaseURL: "https://example.test"})
	if err != nil { t.Fatal(err) }
	if tx == nil || tx.GatewaySlug != "second" { t.Fatalf("transaction=%+v", tx) }
	if first.creates.Load() != 1 || second.creates.Load() != 1 { t.Fatalf("creates first=%d second=%d", first.creates.Load(), second.creates.Load()) }
}

func TestCreatePIXSameRequestKeyNeverDuplicatesGatewayCharge(t *testing.T) {
	store := newFakeStore()
	store.gateways = []gateway.Record{{ID: "g1", Slug: "cash", Adapter: "cash", Priority: 1}}
	adapter := &fakeAdapter{name: "cash", configured: true, created: gateway.CreatedPIX{TransactionID: "gateway-1", CopyPaste: "000201pix", Status: "pending"}}
	svc := New(store, fakeRegistry{"cash": adapter}, "Fatura")
	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	var successes atomic.Int32
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			tx, err := svc.CreatePIX(context.Background(), CreateRequest{InvoiceID: "inv", AmountCents: 1000, RequestKey: "same", BaseURL: "https://x"})
			if err == nil && tx != nil { successes.Add(1) } else if err != nil && !errors.Is(err, ErrAlreadyProcessing) { t.Errorf("unexpected err: %v", err) }
		}()
	}
	wg.Wait()
	if adapter.creates.Load() != 1 { t.Fatalf("gateway creates=%d want 1", adapter.creates.Load()) }
	if successes.Load() < 1 { t.Fatalf("expected at least one successful caller") }
}

func TestCreatePIXFixedStrategyUsesOnlyConfiguredGateway(t *testing.T) {
	fixedID := "g2"
	store := newFakeStore()
	store.routing = RoutingConfig{Strategy: "fixa", FixedGateway: &fixedID}
	store.gateways = []gateway.Record{{ID: "g1", Slug: "a", Adapter: "a", Priority: 1}, {ID: "g2", Slug: "b", Adapter: "b", Priority: 2}}
	a := &fakeAdapter{name: "a", configured: true, created: gateway.CreatedPIX{CopyPaste: "000201a"}}
	b := &fakeAdapter{name: "b", configured: true, created: gateway.CreatedPIX{CopyPaste: "000201b"}}
	svc := New(store, fakeRegistry{"a": a, "b": b}, "Fatura")
	tx, err := svc.CreatePIX(context.Background(), CreateRequest{InvoiceID: "inv", AmountCents: 1000, RequestKey: "k", BaseURL: "https://x"})
	if err != nil { t.Fatal(err) }
	if tx == nil || tx.GatewaySlug != "b" { t.Fatalf("tx=%+v", tx) }
	if a.creates.Load() != 0 || b.creates.Load() != 1 { t.Fatalf("creates a=%d b=%d", a.creates.Load(), b.creates.Load()) }
}

func itoa(v int) string {
	if v == 0 { return "0" }
	var b [20]byte
	i := len(b)
	for v > 0 { i--; b[i] = byte('0' + v%10); v /= 10 }
	return string(b[i:])
}
