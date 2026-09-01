package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matspectrum-ai/Claro-Fatura/internal/invoice"
	"github.com/matspectrum-ai/Claro-Fatura/internal/payment"
)

type invoiceStub struct{}
func (invoiceStub) QueryByPhone(context.Context,string)(invoice.QueryResult,error){return invoice.QueryResult{},nil}
type pixStub struct{}
func (pixStub) Generate(_ context.Context,in payment.GenerateInput)(payment.GeneratedPIX,error){return payment.GeneratedPIX{InvoiceID:in.InvoiceID,CopyPaste:"000201",Gateway:"propix"},nil}
type statusStub struct{}
func (statusStub) Invoice(context.Context,string)(string,error){return "paga",nil}
type webhookStub struct{}
func (webhookStub) Handle(context.Context,*http.Request,string,[]byte)payment.WebhookResult{return payment.WebhookResult{OK:true,Status:http.StatusOK}}
type accessStub struct{page string}
func(s *accessStub)LogPublicAccess(_ context.Context,page string)error{s.page=page;return nil}

func TestHealth(t *testing.T){logger:=slog.New(slog.NewTextHandler(io.Discard,nil));h:=New(Dependencies{Invoices:invoiceStub{}},logger);req:=httptest.NewRequest(http.MethodGet,"/healthz",nil);res:=httptest.NewRecorder();h.ServeHTTP(res,req);if res.Code!=http.StatusOK{t.Fatalf("status=%d",res.Code)}}
func TestQueryInvoice(t *testing.T){logger:=slog.New(slog.NewTextHandler(io.Discard,nil));h:=New(Dependencies{Invoices:invoiceStub{}},logger);req:=httptest.NewRequest(http.MethodGet,"/api/v1/faturas?telefone=93999999999",nil);res:=httptest.NewRecorder();h.ServeHTTP(res,req);if res.Code!=http.StatusOK{t.Fatalf("status=%d body=%s",res.Code,res.Body.String())}}
func TestGeneratePIXRejectsBadRequestKey(t *testing.T){logger:=slog.New(slog.NewTextHandler(io.Discard,nil));h:=New(Dependencies{Invoices:invoiceStub{},PIX:pixStub{}},logger);req:=httptest.NewRequest(http.MethodPost,"/api/v1/faturas/11111111-1111-1111-1111-111111111111/pix",strings.NewReader(`{"request_key":"bad"}`));res:=httptest.NewRecorder();h.ServeHTTP(res,req);if res.Code!=http.StatusBadRequest{t.Fatalf("status=%d body=%s",res.Code,res.Body.String())}}
func TestGeneratePIX(t *testing.T){logger:=slog.New(slog.NewTextHandler(io.Discard,nil));h:=New(Dependencies{Invoices:invoiceStub{},PIX:pixStub{}},logger);req:=httptest.NewRequest(http.MethodPost,"/api/v1/faturas/11111111-1111-1111-1111-111111111111/pix",strings.NewReader(`{"request_key":"22222222-2222-2222-2222-222222222222"}`));res:=httptest.NewRecorder();h.ServeHTTP(res,req);if res.Code!=http.StatusOK||!strings.Contains(res.Body.String(),"000201"){t.Fatalf("status=%d body=%s",res.Code,res.Body.String())}}
func TestInvoiceStatus(t *testing.T){logger:=slog.New(slog.NewTextHandler(io.Discard,nil));h:=New(Dependencies{Invoices:invoiceStub{},Status:statusStub{}},logger);req:=httptest.NewRequest(http.MethodPost,"/api/v1/faturas/11111111-1111-1111-1111-111111111111/status",nil);res:=httptest.NewRecorder();h.ServeHTTP(res,req);if res.Code!=http.StatusOK||!strings.Contains(res.Body.String(),"paga"){t.Fatalf("status=%d body=%s",res.Code,res.Body.String())}}
func TestWebhook(t *testing.T){logger:=slog.New(slog.NewTextHandler(io.Discard,nil));h:=New(Dependencies{Invoices:invoiceStub{},Webhooks:webhookStub{}},logger);req:=httptest.NewRequest(http.MethodPost,"/api/public/webhooks/propix",strings.NewReader(`{"id":"x"}`));res:=httptest.NewRecorder();h.ServeHTTP(res,req);if res.Code!=http.StatusOK{t.Fatalf("status=%d body=%s",res.Code,res.Body.String())}}
func TestEmbeddedHomeInvoiceAndAdminPages(t *testing.T){logger:=slog.New(slog.NewTextHandler(io.Discard,nil));h:=New(Dependencies{Invoices:invoiceStub{}},logger);for _,tc:=range []struct{path,want string}{{"/","Fatura em Dia"},{"/fatura/93999999999","Sua fatura"},{"/assets/app.css","--primary"},{"/admin/gateways","Gateways de pagamento"},{"/assets/admin-gateways.js","loadWebhookSummary"}}{req:=httptest.NewRequest(http.MethodGet,tc.path,nil);res:=httptest.NewRecorder();h.ServeHTTP(res,req);if res.Code!=http.StatusOK{t.Fatalf("%s status=%d body=%s",tc.path,res.Code,res.Body.String())};if !strings.Contains(res.Body.String(),tc.want){t.Fatalf("%s missing %q",tc.path,tc.want)}}}
func TestStaticAssetsAreCacheableButHTMLIsNot(t *testing.T){logger:=slog.New(slog.NewTextHandler(io.Discard,nil));h:=New(Dependencies{Invoices:invoiceStub{}},logger);homeReq:=httptest.NewRequest(http.MethodGet,"/",nil);homeRes:=httptest.NewRecorder();h.ServeHTTP(homeRes,homeReq);if got:=homeRes.Header().Get("Cache-Control");got!="no-cache"{t.Fatalf("home Cache-Control=%q",got)};assetReq:=httptest.NewRequest(http.MethodGet,"/assets/logo-claro.png",nil);assetRes:=httptest.NewRecorder();h.ServeHTTP(assetRes,assetReq);if !strings.HasPrefix(assetRes.Header().Get("Cache-Control"),"public"){t.Fatalf("asset Cache-Control=%q",assetRes.Header().Get("Cache-Control"))}}
func TestPublicAccessEndpointKeepsLoggingSilent(t *testing.T){logger:=slog.New(slog.NewTextHandler(io.Discard,nil));access:=&accessStub{};h:=New(Dependencies{Invoices:invoiceStub{},Access:access},logger);req:=httptest.NewRequest(http.MethodPost,"/api/v1/acessos",strings.NewReader(`{"pagina":"/"}`));req.Header.Set("Content-Type","application/json");res:=httptest.NewRecorder();h.ServeHTTP(res,req);if res.Code!=http.StatusNoContent||access.page!="/"{t.Fatalf("status=%d page=%q",res.Code,access.page)}}
func TestSecurityHeaders(t *testing.T){logger:=slog.New(slog.NewTextHandler(io.Discard,nil));h:=New(Dependencies{Invoices:invoiceStub{}},logger);req:=httptest.NewRequest(http.MethodGet,"/healthz",nil);res:=httptest.NewRecorder();h.ServeHTTP(res,req);if res.Header().Get("X-Frame-Options")!="DENY"||res.Header().Get("X-Content-Type-Options")!="nosniff"{t.Fatalf("security headers missing: %#v",res.Header())}}
