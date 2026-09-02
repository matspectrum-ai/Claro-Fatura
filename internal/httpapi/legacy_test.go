package httpapi

import(
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matspectrum-ai/Claro-Fatura/internal/legacy"
)
type legacyStub struct{chargeCalls,flowCalls,directCalls int}
func(l *legacyStub)InvoiceByPhone(context.Context,string)(legacy.InvoiceView,bool,error){return legacy.InvoiceView{Phone:"11999999999",InvoiceID:"f1",OpenAmount:100,DiscountAmount:90},true,nil}
func(l *legacyStub)CreateCharge(context.Context,string,string,string)(legacy.ChargeResult,error){l.chargeCalls++;return legacy.ChargeResult{InvoiceID:"f1",Amount:90,PIXCopyPaste:"000201",Gateway:"propix"},nil}
func(l *legacyStub)SetupProPix(context.Context)(map[string]any,error){return map[string]any{"success":true,"message":"ProPix já existe no banco."},nil}
func(l *legacyStub)Secrets()map[string]bool{return map[string]bool{"PROPIX_CLIENT_ID":true}}
func(l *legacyStub)TestFlow(context.Context)map[string]any{l.flowCalls++;return map[string]any{"success":true}}
func(l *legacyStub)TestProPixDirect(context.Context)map[string]any{l.directCalls++;return map[string]any{"success":true}}
func legacyHandler(l LegacyAPI)http.Handler{logger:=slog.New(slog.NewTextHandler(io.Discard,nil));return New(Dependencies{Invoices:invoiceStub{},Auth:&authStub{},Legacy:l},logger)}
func TestLegacyFaturasKeepsBearerAndCORSContract(t *testing.T){h:=legacyHandler(&legacyStub{});r:=httptest.NewRequest(http.MethodGet,"/api/public/faturas?telefone=(11)%2099999-9999",nil);r.Header.Set("Authorization","Bearer ok");w:=httptest.NewRecorder();h.ServeHTTP(w,r);if w.Code!=200||w.Header().Get("Access-Control-Allow-Origin")!="*"||!strings.Contains(w.Body.String(),"11999999999"){t.Fatalf("status=%d cors=%q body=%s",w.Code,w.Header().Get("Access-Control-Allow-Origin"),w.Body.String())}}
func TestLegacyFaturasRejectsMissingBearer(t *testing.T){h:=legacyHandler(&legacyStub{});r:=httptest.NewRequest(http.MethodGet,"/api/public/faturas?telefone=11999999999",nil);w:=httptest.NewRecorder();h.ServeHTTP(w,r);if w.Code!=401||!strings.Contains(w.Body.String(),"Não autorizado"){t.Fatalf("status=%d body=%s",w.Code,w.Body.String())}}
func TestLegacyCobrancaCallsFakeRouterLayer(t *testing.T){l:=&legacyStub{};h:=legacyHandler(l);r:=httptest.NewRequest(http.MethodPost,"/api/public/cobranca",strings.NewReader(`{"fatura_id":"11111111-1111-1111-1111-111111111111"}`));r.Header.Set("Authorization","Bearer ok");w:=httptest.NewRecorder();h.ServeHTTP(w,r);if w.Code!=200||l.chargeCalls!=1||!strings.Contains(w.Body.String(),"000201"){t.Fatalf("status=%d calls=%d body=%s",w.Code,l.chargeCalls,w.Body.String())}}
func TestLegacyOptionsNeedsNoAuth(t *testing.T){h:=legacyHandler(&legacyStub{});for _,path:=range []string{"/api/public/faturas","/api/public/cobranca"}{r:=httptest.NewRequest(http.MethodOptions,path,nil);w:=httptest.NewRecorder();h.ServeHTTP(w,r);if w.Code!=204||w.Header().Get("Access-Control-Max-Age")!="86400"{t.Fatalf("%s status=%d headers=%v",path,w.Code,w.Header())}}}
func TestLegacyDiagnosticRoutesUseFakeService(t *testing.T){l:=&legacyStub{};h:=legacyHandler(l);for _,path:=range []string{"/api/public/test-fluxo","/api/public/test-propix-direto"}{r:=httptest.NewRequest(http.MethodGet,path,nil);r.Header.Set("Authorization","Bearer ok");w:=httptest.NewRecorder();h.ServeHTTP(w,r);if w.Code!=200{t.Fatalf("%s status=%d body=%s",path,w.Code,w.Body.String())}};if l.flowCalls!=1||l.directCalls!=1{t.Fatalf("flow=%d direct=%d",l.flowCalls,l.directCalls)}}
func TestLegacyAdminAliases(t *testing.T){h:=legacyHandler(&legacyStub{});r:=httptest.NewRequest(http.MethodGet,"/admin/clientes",nil);w:=httptest.NewRecorder();h.ServeHTTP(w,r);if w.Code!=http.StatusFound||w.Header().Get("Location")!="/admin/faturas"{t.Fatalf("status=%d location=%q",w.Code,w.Header().Get("Location"))};r=httptest.NewRequest(http.MethodGet,"/admin/",nil);w=httptest.NewRecorder();h.ServeHTTP(w,r);if w.Code!=200||!strings.Contains(w.Body.String(),"Dashboard"){t.Fatalf("status=%d",w.Code)}}
