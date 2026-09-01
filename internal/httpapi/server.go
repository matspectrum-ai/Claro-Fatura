package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/invoice"
	"github.com/matspectrum-ai/Claro-Fatura/internal/payment"
)

type InvoiceLookup interface { QueryByPhone(context.Context, string) (invoice.QueryResult, error) }
type PIXGenerator interface { Generate(context.Context, payment.GenerateInput) (payment.GeneratedPIX, error) }
type InvoiceStatus interface { Invoice(context.Context, string) (string, error) }
type WebhookHandler interface { Handle(context.Context, *http.Request, string, []byte) payment.WebhookResult }
type PublicAccessLogger interface { LogPublicAccess(context.Context, string) error }

type Dependencies struct {
	Invoices InvoiceLookup
	PIX PIXGenerator
	Status InvoiceStatus
	Webhooks WebhookHandler
	Access PublicAccessLogger
	Auth AdminAuth
	AdminInvoices AdminInvoices
	AdminImporter AdminImporter
	AdminMetrics AdminMetrics
	AdminActivity AdminActivity
	SiteURL string
}

type Server struct { deps Dependencies; logger *slog.Logger }

func New(deps Dependencies, logger *slog.Logger) http.Handler {
	s:=&Server{deps:deps,logger:logger}; mux:=http.NewServeMux()
	mux.HandleFunc("GET /",s.home); mux.HandleFunc("GET /fatura/{telefone}",s.invoicePage); mux.HandleFunc("GET /assets/{path...}",s.asset); mux.HandleFunc("GET /favicon.png",s.favicon); mux.HandleFunc("GET /robots.txt",s.robots)
	mux.HandleFunc("GET /auth",s.authPage); mux.HandleFunc("GET /forgot-password",s.forgotPasswordPage); mux.HandleFunc("GET /reset-password",s.resetPasswordPage)
	mux.HandleFunc("GET /admin",s.adminPage); mux.HandleFunc("GET /admin/faturas",s.adminPage); mux.HandleFunc("GET /admin/pagamentos",s.adminActivityPage); mux.HandleFunc("GET /admin/transacoes",s.adminActivityPage); mux.HandleFunc("GET /admin/logs",s.adminActivityPage)
	mux.HandleFunc("POST /api/auth/login",s.login); mux.HandleFunc("POST /api/auth/signup",s.signup); mux.HandleFunc("GET /api/auth/me",s.authMe); mux.HandleFunc("POST /api/auth/logout",s.logout); mux.HandleFunc("POST /api/auth/recover",s.recoverPassword); mux.HandleFunc("POST /api/auth/recovery-session",s.recoverySession); mux.HandleFunc("POST /api/auth/password",s.updatePassword)
	mux.HandleFunc("GET /api/admin/metricas",s.adminMetrics); mux.HandleFunc("DELETE /api/admin/metricas",s.adminClearMetrics); mux.HandleFunc("GET /api/admin/faturas",s.adminInvoices); mux.HandleFunc("PATCH /api/admin/faturas/{id}",s.adminSaveInvoice); mux.HandleFunc("PATCH /api/admin/faturas/{id}/status",s.adminInvoiceStatus); mux.HandleFunc("POST /api/admin/importar",s.adminImport); mux.HandleFunc("DELETE /api/admin/base",s.adminDeleteAll); mux.HandleFunc("GET /api/admin/pagamentos",s.adminPayments); mux.HandleFunc("GET /api/admin/transacoes",s.adminTransactions); mux.HandleFunc("GET /api/admin/logs",s.adminLogs)
	mux.HandleFunc("GET /healthz",s.health); mux.HandleFunc("POST /api/v1/acessos",s.publicAccess); mux.HandleFunc("GET /api/v1/faturas",s.queryInvoices); mux.HandleFunc("POST /api/v1/faturas/{id}/pix",s.generatePIX); mux.HandleFunc("POST /api/v1/faturas/{id}/status",s.invoiceStatus); mux.HandleFunc("POST /api/public/webhooks/{slug}",s.webhook)
	return securityHeaders(accessLog(logger,mux))
}

func(s *Server)health(w http.ResponseWriter,_ *http.Request){writeJSON(w,http.StatusOK,map[string]string{"status":"ok"})}
func(s *Server)publicAccess(w http.ResponseWriter,r *http.Request){if s.deps.Access==nil{w.WriteHeader(http.StatusNoContent);return};var body struct{Pagina string `json:"pagina"`};if err:=decodeJSON(r,&body);err!=nil||body.Pagina!="/"{w.WriteHeader(http.StatusNoContent);return};if err:=s.deps.Access.LogPublicAccess(r.Context(),"/");err!=nil{s.logger.Warn("public access log failed","error",err)};w.WriteHeader(http.StatusNoContent)}
func(s *Server)queryInvoices(w http.ResponseWriter,r *http.Request){phone:=strings.TrimSpace(r.URL.Query().Get("telefone"));result,err:=s.deps.Invoices.QueryByPhone(r.Context(),phone);if err!=nil{status:=http.StatusInternalServerError;if err.Error()=="telefone inválido"{status=http.StatusBadRequest};writeJSON(w,status,map[string]string{"erro":publicError(status)});return};writeJSON(w,http.StatusOK,result)}
func(s *Server)generatePIX(w http.ResponseWriter,r *http.Request){if s.deps.PIX==nil{writeJSON(w,http.StatusServiceUnavailable,map[string]string{"erro":"Pagamento indisponível no momento."});return};invoiceID:=strings.TrimSpace(r.PathValue("id"));if !isUUID(invoiceID){writeJSON(w,http.StatusBadRequest,map[string]string{"erro":"Fatura inválida."});return};var body struct{RequestKey string `json:"request_key"`;Force bool `json:"forcar"`};if err:=decodeJSON(r,&body);err!=nil||!isUUID(body.RequestKey){writeJSON(w,http.StatusBadRequest,map[string]string{"erro":"Dados inválidos."});return};result,err:=s.deps.PIX.Generate(r.Context(),payment.GenerateInput{InvoiceID:invoiceID,RequestKey:body.RequestKey,Force:body.Force,BaseURL:requestBaseURL(r,s.deps.SiteURL)});if err!=nil{switch{case errors.Is(err,payment.ErrInvoiceNotFound):writeJSON(w,http.StatusNotFound,map[string]string{"erro":"Fatura não encontrada."});case errors.Is(err,payment.ErrAlreadyProcessing),errors.Is(err,payment.ErrRequestUsed):writeJSON(w,http.StatusConflict,map[string]string{"erro":err.Error()});default:s.logger.Error("pix generation failed","invoice_id",invoiceID,"error",err);writeJSON(w,http.StatusInternalServerError,map[string]string{"erro":"Não foi possível gerar o PIX agora."})};return};writeJSON(w,http.StatusOK,result)}
func(s *Server)invoiceStatus(w http.ResponseWriter,r *http.Request){if s.deps.Status==nil{writeJSON(w,http.StatusServiceUnavailable,map[string]string{"erro":"Consulta de pagamento indisponível."});return};invoiceID:=strings.TrimSpace(r.PathValue("id"));if !isUUID(invoiceID){writeJSON(w,http.StatusBadRequest,map[string]string{"erro":"Fatura inválida."});return};status,err:=s.deps.Status.Invoice(r.Context(),invoiceID);if err!=nil{s.logger.Error("invoice status failed","invoice_id",invoiceID,"error",err);writeJSON(w,http.StatusInternalServerError,map[string]string{"erro":"Não foi possível consultar no momento."});return};writeJSON(w,http.StatusOK,map[string]string{"status":status})}
func(s *Server)webhook(w http.ResponseWriter,r *http.Request){if s.deps.Webhooks==nil{writeText(w,http.StatusServiceUnavailable,"Webhook indisponível");return};slug:=strings.TrimSpace(r.PathValue("slug"));if slug==""{writeText(w,http.StatusNotFound,"Gateway desconhecida");return};raw,err:=io.ReadAll(io.LimitReader(r.Body,1<<20));if err!=nil{writeText(w,http.StatusBadRequest,"Corpo inválido");return};result:=s.deps.Webhooks.Handle(r.Context(),r,slug,raw);if result.OK{writeJSON(w,result.Status,map[string]bool{"ok":true});return};writeText(w,result.Status,result.Message)}
func decodeJSON(r *http.Request,dst any)error{decoder:=json.NewDecoder(io.LimitReader(r.Body,64<<10));decoder.DisallowUnknownFields();if err:=decoder.Decode(dst);err!=nil{return err};var trailing any;if err:=decoder.Decode(&trailing);err!=io.EOF{return errors.New("multiple JSON values")};return nil}
func requestBaseURL(r *http.Request,fallback string)string{if strings.TrimSpace(fallback)!=""{return strings.TrimRight(fallback,"/")};scheme:="http";if r.TLS!=nil{scheme="https"};if forwarded:=strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"),",")[0]);forwarded=="http"||forwarded=="https"{scheme=forwarded};return scheme+"://"+r.Host}
func isUUID(value string)bool{if len(value)!=36||value[8]!='-'||value[13]!='-'||value[18]!='-'||value[23]!='-'{return false};for i,ch:=range value{if i==8||i==13||i==18||i==23{continue};if !((ch>='0'&&ch<='9')||(ch>='a'&&ch<='f')||(ch>='A'&&ch<='F')){return false}};return true}
func publicError(status int)string{if status==http.StatusBadRequest{return "Telefone inválido"};return "Não foi possível consultar no momento."}
func writeJSON(w http.ResponseWriter,status int,value any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.Header().Set("Cache-Control","no-store");w.WriteHeader(status);_=json.NewEncoder(w).Encode(value)}
func writeText(w http.ResponseWriter,status int,value string){w.Header().Set("Content-Type","text/plain; charset=utf-8");w.Header().Set("Cache-Control","no-store");w.WriteHeader(status);_,_=io.WriteString(w,value)}
func securityHeaders(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Header().Set("X-Content-Type-Options","nosniff");w.Header().Set("X-Frame-Options","DENY");w.Header().Set("Referrer-Policy","strict-origin-when-cross-origin");next.ServeHTTP(w,r)})}
func accessLog(logger *slog.Logger,next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){started:=time.Now();next.ServeHTTP(w,r);logger.Info("http request","method",r.Method,"path",r.URL.Path,"duration_ms",time.Since(started).Milliseconds())})}
