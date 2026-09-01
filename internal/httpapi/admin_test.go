package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matspectrum-ai/Claro-Fatura/internal/admin"
	authsvc "github.com/matspectrum-ai/Claro-Fatura/internal/auth"
)

type authStub struct{ calls int }
func(a *authStub)SignIn(context.Context,string,string)(authsvc.Session,error){return authsvc.Session{},nil}
func(a *authStub)SignUp(context.Context,string,string,string,string)(authsvc.Session,bool,error){return authsvc.Session{},false,nil}
func(a *authStub)RequireAdmin(_ context.Context,t string)(authsvc.User,error){a.calls++;if t!="ok"{return authsvc.User{},authsvc.ErrNoSession};return authsvc.User{ID:"u",Email:"admin@test"},nil}
func(a *authStub)Refresh(context.Context,string)(authsvc.Session,error){return authsvc.Session{},authsvc.ErrNoSession}
func(a *authStub)SendRecovery(context.Context,string,string)error{return nil}
func(a *authStub)UpdatePassword(context.Context,string,string)error{return nil}
func(a *authStub)Logout(context.Context,string)error{return nil}
type importerStub struct{called bool};func(i *importerStub)Import(_ context.Context,in admin.ImportInput)(admin.ImportResult,error){i.called=true;return admin.ImportResult{Imported:len(in.Rows)},nil}
type metricsStub struct{};func(metricsStub)Get(context.Context)(admin.Metrics,error){return admin.Metrics{DatabaseClients:7},nil};func(metricsStub)Clear(context.Context)(int,error){return 3,nil}
type invoicesStub struct{};func(invoicesStub)List(context.Context,string,int)(admin.InvoicePage,error){return admin.InvoicePage{Total:2},nil};func(invoicesStub)Save(context.Context,string,string,admin.InvoiceEdit)error{return nil};func(invoicesStub)SetStatus(context.Context,string,string)error{return nil};func(invoicesStub)DeleteAll(context.Context,string)(admin.DeleteAllResult,error){return admin.DeleteAllResult{Clients:2},nil}
func adminHandler(a AdminAuth,imp AdminImporter)http.Handler{logger:=slog.New(slog.NewTextHandler(io.Discard,nil));return New(Dependencies{Invoices:invoiceStub{},Auth:a,AdminImporter:imp,AdminInvoices:invoicesStub{},AdminMetrics:metricsStub{}},logger)}
func TestAdminAPIRejectsMissingSession(t *testing.T){h:=adminHandler(&authStub{},&importerStub{});r:=httptest.NewRequest(http.MethodGet,"/api/admin/metricas",nil);w:=httptest.NewRecorder();h.ServeHTTP(w,r);if w.Code!=http.StatusUnauthorized{t.Fatalf("status=%d body=%s",w.Code,w.Body.String())}}
func TestAdminImportUsesOneAdminValidationAndCallsImporter(t *testing.T){a:=&authStub{};imp:=&importerStub{};h:=adminHandler(a,imp);r:=httptest.NewRequest(http.MethodPost,"/api/admin/importar",strings.NewReader(`{"clientes":[{"telefone":"11999999999"}],"vencimento_global":"2026-09-30"}`));r.AddCookie(&http.Cookie{Name:accessCookieName,Value:"ok"});w:=httptest.NewRecorder();h.ServeHTTP(w,r);if w.Code!=http.StatusOK||!imp.called{t.Fatalf("status=%d called=%v body=%s",w.Code,imp.called,w.Body.String())};if a.calls!=1{t.Fatalf("admin validations=%d",a.calls)}}
