package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/matspectrum-ai/Claro-Fatura/internal/legacy"
)

type LegacyAPI interface {
	InvoiceByPhone(context.Context, string) (legacy.InvoiceView, bool, error)
	CreateCharge(context.Context, string, string, string) (legacy.ChargeResult, error)
	SetupProPix(context.Context) (map[string]any, error)
	Secrets() map[string]bool
	TestFlow(context.Context) map[string]any
	TestProPixDirect(context.Context) map[string]any
}
func legacyCORS(w http.ResponseWriter,methods string){w.Header().Set("Access-Control-Allow-Origin","*");w.Header().Set("Access-Control-Allow-Methods",methods+", OPTIONS");w.Header().Set("Access-Control-Allow-Headers","Content-Type, Authorization");w.Header().Set("Access-Control-Max-Age","86400")}
func(s *Server)legacyFaturasOptions(w http.ResponseWriter,_ *http.Request){legacyCORS(w,"GET");w.WriteHeader(http.StatusNoContent)}
func(s *Server)legacyCobrancaOptions(w http.ResponseWriter,_ *http.Request){legacyCORS(w,"POST");w.WriteHeader(http.StatusNoContent)}
func(s *Server)legacyFaturas(w http.ResponseWriter,r *http.Request){legacyCORS(w,"GET");if !s.requireLegacyBearer(w,r,true){return};phone:=digitsOnly(r.URL.Query().Get("telefone"));if len(phone)!=11{writeJSON(w,400,map[string]string{"erro":"Informe um telefone válido com DDD (11 dígitos)."});return};if s.deps.Legacy==nil{writeJSON(w,500,map[string]string{"erro":"Não foi possível consultar no momento."});return};row,found,err:=s.deps.Legacy.InvoiceByPhone(r.Context(),phone);if err!=nil{writeJSON(w,500,map[string]string{"erro":"Não foi possível consultar no momento."});return};if !found{writeJSON(w,404,map[string]string{"erro":"Nenhuma fatura encontrada para este telefone."});return};writeJSON(w,200,row)}
func(s *Server)legacyCobranca(w http.ResponseWriter,r *http.Request){legacyCORS(w,"POST");if !s.requireLegacyBearer(w,r,true){return};var raw map[string]any;decoder:=json.NewDecoder(io.LimitReader(r.Body,64<<10));if err:=decoder.Decode(&raw);err!=nil{writeJSON(w,400,map[string]string{"erro":"Corpo inválido."});return};var phone,invoiceID string;if value,exists:=raw["telefone"];exists{text,ok:=value.(string);if !ok{writeJSON(w,400,map[string]string{"erro":"Informe um telefone válido com DDD (11 dígitos)."});return};phone=digitsOnly(text);if len(phone)!=11{writeJSON(w,400,map[string]string{"erro":"Informe um telefone válido com DDD (11 dígitos)."});return}};if value,exists:=raw["fatura_id"];exists{text,ok:=value.(string);if !ok||!isUUID(text){writeJSON(w,400,map[string]string{"erro":"Invalid uuid"});return};invoiceID=text};if phone==""&&invoiceID==""{writeJSON(w,400,map[string]string{"erro":"Informe telefone ou fatura_id."});return};if s.deps.Legacy==nil{writeJSON(w,503,map[string]string{"erro":"Não foi possível gerar a cobrança agora."});return};out,err:=s.deps.Legacy.CreateCharge(r.Context(),phone,invoiceID,requestOrigin(r));if err!=nil{switch{case errors.Is(err,legacy.ErrNoPending):writeJSON(w,404,map[string]string{"erro":"Nenhuma fatura pendente para este telefone."});case errors.Is(err,legacy.ErrNotFound):if invoiceID==""{writeJSON(w,404,map[string]string{"erro":"Nenhuma fatura encontrada para este telefone."})}else{writeJSON(w,404,map[string]string{"erro":"Fatura não encontrada."})};case errors.Is(err,legacy.ErrNotPending):writeJSON(w,409,map[string]string{"erro":"Esta fatura não está pendente de pagamento."});case errors.Is(err,legacy.ErrInvalidAmount):writeJSON(w,422,map[string]string{"erro":"A fatura não possui um valor com desconto válido."});case errors.Is(err,legacy.ErrChargeFailed):writeJSON(w,503,map[string]string{"erro":"Não foi possível gerar a cobrança agora."});default:s.logger.Error("legacy charge failed","error",err);writeJSON(w,500,map[string]string{"erro":"Não foi possível gerar a cobrança agora."})};return};writeJSON(w,200,out)}
func(s *Server)legacySetupProPix(w http.ResponseWriter,r *http.Request){if !s.requireLegacyBearer(w,r,false){return};if s.deps.Legacy==nil{writeJSON(w,500,map[string]string{"error":"Serviço indisponível."});return};out,err:=s.deps.Legacy.SetupProPix(r.Context());if err!=nil{writeJSON(w,500,map[string]string{"error":err.Error()});return};writeJSON(w,200,out)}
func(s *Server)legacyTestSecrets(w http.ResponseWriter,r *http.Request){if !s.requireLegacyBearer(w,r,false){return};if s.deps.Legacy==nil{writeJSON(w,500,map[string]string{"error":"Serviço indisponível."});return};writeJSON(w,200,s.deps.Legacy.Secrets())}
func(s *Server)legacyTestFlow(w http.ResponseWriter,r *http.Request){if !s.requireLegacyBearer(w,r,false){return};if s.deps.Legacy==nil{writeJSON(w,500,map[string]string{"error":"Serviço indisponível."});return};writeJSON(w,200,s.deps.Legacy.TestFlow(r.Context()))}
func(s *Server)legacyTestProPixDirect(w http.ResponseWriter,r *http.Request){if !s.requireLegacyBearer(w,r,false){return};if s.deps.Legacy==nil{writeJSON(w,500,map[string]string{"error":"Serviço indisponível."});return};writeJSON(w,200,s.deps.Legacy.TestProPixDirect(r.Context()))}
func(s *Server)requireLegacyBearer(w http.ResponseWriter,r *http.Request,cors bool)bool{header:=strings.TrimSpace(r.Header.Get("Authorization"));fields:=strings.Fields(header);if len(fields)!=2||!strings.EqualFold(fields[0],"Bearer")||strings.TrimSpace(fields[1])==""||s.deps.Auth==nil{if cors{legacyCORS(w,legacyMethod(r.URL.Path))};writeJSON(w,401,map[string]string{legacyErrorKey(r.URL.Path):"Não autorizado."});return false};if _,err:=s.deps.Auth.RequireAdmin(r.Context(),fields[1]);err!=nil{if cors{legacyCORS(w,legacyMethod(r.URL.Path))};writeJSON(w,401,map[string]string{legacyErrorKey(r.URL.Path):"Não autorizado."});return false};return true}
func legacyErrorKey(path string)string{if path=="/api/public/faturas"||path=="/api/public/cobranca"{return "erro"};return "error"}
func legacyMethod(path string)string{if path=="/api/public/cobranca"{return "POST"};return "GET"}
func digitsOnly(v string)string{var b strings.Builder;for _,r:=range v{if r>='0'&&r<='9'{b.WriteRune(r)}};return b.String()}
func requestOrigin(r *http.Request)string{scheme:="http";if r.TLS!=nil{scheme="https"};if x:=strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"),",")[0]);x=="http"||x=="https"{scheme=x};return scheme+"://"+r.Host}
