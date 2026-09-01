package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matspectrum-ai/Claro-Fatura/internal/invoice"
	"github.com/matspectrum-ai/Claro-Fatura/internal/payment"
)

const testUUID = "550e8400-e29b-41d4-a716-446655440000"
const requestUUID = "550e8400-e29b-41d4-a716-446655440001"

type invoiceStub struct{}

func (invoiceStub) QueryByPhone(context.Context, string) (invoice.QueryResult, error) {
	return invoice.QueryResult{Found: true}, nil
}

type pixStub struct{ input payment.GenerateInput }

func (s *pixStub) Generate(_ context.Context, in payment.GenerateInput) (payment.GeneratedPIX, error) {
	s.input = in
	copy := "000201PIX"
	return payment.GeneratedPIX{Value: 12.34, CopyPaste: copy, TXID: "gw-1", Status: "em_aberto", Available: true, TransactionID: "tx-1", Gateway: "cashinpay"}, nil
}

type statusStub struct{ id string }

func (s *statusStub) Invoice(_ context.Context, id string) (string, error) {
	s.id = id
	return "paga", nil
}

type webhookStub struct {
	slug string
	raw  string
}

func (s *webhookStub) Handle(_ context.Context, _ *http.Request, slug string, raw []byte) payment.WebhookResult {
	s.slug, s.raw = slug, string(raw)
	return payment.WebhookResult{Status: http.StatusOK, OK: true}
}

func newTestHandler(pix PIXGenerator, status InvoiceStatus, webhooks WebhookHandler) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(Dependencies{Invoices: invoiceStub{}, PIX: pix, Status: status, Webhooks: webhooks, SiteURL: "https://clarofatura.app"}, logger)
}

func TestGeneratePIXHTTPContract(t *testing.T) {
	pix := &pixStub{}
	h := newTestHandler(pix, &statusStub{}, &webhookStub{})
	body := `{"request_key":"` + requestUUID + `","forcar":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/faturas/"+testUUID+"/pix", strings.NewReader(body))
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	if pix.input.InvoiceID != testUUID || pix.input.RequestKey != requestUUID || !pix.input.Force || pix.input.BaseURL != "https://clarofatura.app" {
		t.Fatalf("input=%+v", pix.input)
	}
	var got payment.GeneratedPIX
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if !got.Available || got.CopyPaste != "000201PIX" || got.Gateway != "cashinpay" {
		t.Fatalf("got=%+v", got)
	}
}

func TestGeneratePIXRejectsInvalidRequestKey(t *testing.T) {
	h := newTestHandler(&pixStub{}, &statusStub{}, &webhookStub{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/faturas/"+testUUID+"/pix", strings.NewReader(`{"request_key":"not-a-uuid"}`))
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
}

func TestInvoiceStatusHTTPContract(t *testing.T) {
	status := &statusStub{}
	h := newTestHandler(&pixStub{}, status, &webhookStub{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/faturas/"+testUUID+"/status", nil)
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != http.StatusOK || status.id != testUUID {
		t.Fatalf("status=%d id=%s", res.Code, status.id)
	}
	if strings.TrimSpace(res.Body.String()) != `{"status":"paga"}` {
		t.Fatalf("body=%q", res.Body.String())
	}
}

func TestWebhookKeepsOriginalPublicPathAndJSONSuccess(t *testing.T) {
	webhook := &webhookStub{}
	h := newTestHandler(&pixStub{}, &statusStub{}, webhook)
	req := httptest.NewRequest(http.MethodPost, "/api/public/webhooks/propix", strings.NewReader(`{"event":"paid"}`))
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	if webhook.slug != "propix" || webhook.raw != `{"event":"paid"}` {
		t.Fatalf("slug=%q raw=%q", webhook.slug, webhook.raw)
	}
	if strings.TrimSpace(res.Body.String()) != `{"ok":true}` {
		t.Fatalf("body=%q", res.Body.String())
	}
}

type accessStub struct {
	page string
}

func (s *accessStub) LogPublicAccess(_ context.Context, page string) error {
	s.page = page
	return nil
}

func TestEmbeddedHomeAndInvoicePages(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := New(Dependencies{Invoices: invoiceStub{}}, logger)

	for _, tc := range []struct {
		path, want string
	}{
		{"/", "Fatura em Dia"},
		{"/fatura/93999999999", "Sua fatura"},
		{"/assets/app.css", "--primary"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", tc.path, res.Code, res.Body.String())
		}
		if !strings.Contains(res.Body.String(), tc.want) {
			t.Fatalf("%s missing %q", tc.path, tc.want)
		}
	}
}

func TestStaticAssetsAreCacheableButHTMLIsNot(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := New(Dependencies{Invoices: invoiceStub{}}, logger)

	homeReq := httptest.NewRequest(http.MethodGet, "/", nil)
	homeRes := httptest.NewRecorder()
	h.ServeHTTP(homeRes, homeReq)
	if got := homeRes.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("home Cache-Control=%q", got)
	}

	assetReq := httptest.NewRequest(http.MethodGet, "/assets/logo-claro.png", nil)
	assetRes := httptest.NewRecorder()
	h.ServeHTTP(assetRes, assetReq)
	if !strings.HasPrefix(assetRes.Header().Get("Cache-Control"), "public") {
		t.Fatalf("asset Cache-Control=%q", assetRes.Header().Get("Cache-Control"))
	}
}

func TestPublicAccessEndpointKeepsLoggingSilent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	access := &accessStub{}
	h := New(Dependencies{Invoices: invoiceStub{}, Access: access}, logger)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/acessos", strings.NewReader(`{"pagina":"/"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent || access.page != "/" {
		t.Fatalf("status=%d page=%q", res.Code, access.page)
	}
}
