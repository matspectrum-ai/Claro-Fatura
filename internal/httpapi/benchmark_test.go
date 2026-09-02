package httpapi

import(
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)
func benchmarkHandler()http.Handler{return New(Dependencies{Invoices:invoiceStub{}},slog.New(slog.NewTextHandler(io.Discard,nil)))}
func BenchmarkHealthHTTP(b *testing.B){h:=benchmarkHandler();b.ReportAllocs();b.ResetTimer();for i:=0;i<b.N;i++{r:=httptest.NewRequest(http.MethodGet,"/healthz",nil);w:=httptest.NewRecorder();h.ServeHTTP(w,r);if w.Code!=200{b.Fatal(w.Code)}}}
func BenchmarkInvoiceLookupHTTPParallel(b *testing.B){h:=benchmarkHandler();b.ReportAllocs();b.ResetTimer();b.RunParallel(func(pb *testing.PB){for pb.Next(){r:=httptest.NewRequest(http.MethodGet,"/api/v1/faturas?telefone=11999999999",nil);w:=httptest.NewRecorder();h.ServeHTTP(w,r);if w.Code!=200{b.Fatal(w.Code)}}})}
