package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/matspectrum-ai/Claro-Fatura/internal/admin"
)

type AdminGateways interface {
	List(context.Context) ([]admin.GatewayConfig, error)
	Routing(context.Context) (admin.RoutingConfig, error)
	SaveRouting(context.Context, admin.RoutingInput) error
	Save(context.Context, admin.GatewayInput) error
	Patch(context.Context, admin.GatewayPatch) error
	Remove(context.Context, string) error
	UseOnly(context.Context, string) error
	ActivateAll(context.Context) error
	WebhookSummary(context.Context) ([]admin.GatewayWebhookSummary, error)
}

func (s *Server) adminGateways(w http.ResponseWriter,r *http.Request){if _,ok:=s.requireAdmin(w,r);!ok{return};if s.deps.AdminGateways==nil{writeJSON(w,http.StatusServiceUnavailable,map[string]string{"erro":"Gateways indisponíveis."});return};out,err:=s.deps.AdminGateways.List(r.Context());if err!=nil{s.logger.Error("admin gateways failed","error",err);writeJSON(w,500,map[string]string{"erro":"Não foi possível carregar os gateways."});return};writeJSON(w,200,out)}
func (s *Server) adminRouting(w http.ResponseWriter,r *http.Request){if _,ok:=s.requireAdmin(w,r);!ok{return};out,err:=s.deps.AdminGateways.Routing(r.Context());if err!=nil{writeJSON(w,500,map[string]string{"erro":"Não foi possível carregar a estratégia."});return};writeJSON(w,200,out)}
func (s *Server) adminSaveRouting(w http.ResponseWriter,r *http.Request){if _,ok:=s.requireAdmin(w,r);!ok{return};var in admin.RoutingInput;if decodeJSON(r,&in)!=nil{writeJSON(w,400,map[string]string{"erro":"Dados inválidos."});return};if err:=s.deps.AdminGateways.SaveRouting(r.Context(),in);err!=nil{status:=500;if errors.Is(err,admin.ErrInvalidRouting){status=400};writeJSON(w,status,map[string]string{"erro":"Não foi possível salvar a estratégia."});return};writeJSON(w,200,map[string]bool{"ok":true})}
func (s *Server) adminSaveGateway(w http.ResponseWriter,r *http.Request){if _,ok:=s.requireAdmin(w,r);!ok{return};var in admin.GatewayInput;if decodeJSON(r,&in)!=nil{writeJSON(w,400,map[string]string{"erro":"Dados inválidos."});return};if err:=s.deps.AdminGateways.Save(r.Context(),in);err!=nil{status:=500;if errors.Is(err,admin.ErrInvalidGateway){status=400};writeJSON(w,status,map[string]string{"erro":"Não foi possível salvar o gateway."});return};writeJSON(w,200,map[string]bool{"ok":true})}
func (s *Server) adminPatchGateway(w http.ResponseWriter,r *http.Request){if _,ok:=s.requireAdmin(w,r);!ok{return};var in admin.GatewayPatch;if decodeJSON(r,&in)!=nil{writeJSON(w,400,map[string]string{"erro":"Dados inválidos."});return};in.ID=r.PathValue("id");if err:=s.deps.AdminGateways.Patch(r.Context(),in);err!=nil{status:=500;if errors.Is(err,admin.ErrInvalidGateway){status=400};writeJSON(w,status,map[string]string{"erro":"Não foi possível salvar a alteração."});return};writeJSON(w,200,map[string]bool{"ok":true})}
func (s *Server) adminRemoveGateway(w http.ResponseWriter,r *http.Request){if _,ok:=s.requireAdmin(w,r);!ok{return};if err:=s.deps.AdminGateways.Remove(r.Context(),r.PathValue("id"));err!=nil{status:=500;if errors.Is(err,admin.ErrInvalidGateway){status=400};writeJSON(w,status,map[string]string{"erro":"Não foi possível remover o gateway."});return};writeJSON(w,200,map[string]bool{"ok":true})}
func (s *Server) adminUseOnlyGateway(w http.ResponseWriter,r *http.Request){if _,ok:=s.requireAdmin(w,r);!ok{return};if err:=s.deps.AdminGateways.UseOnly(r.Context(),r.PathValue("id"));err!=nil{status:=500;if errors.Is(err,admin.ErrInvalidGateway){status=400};writeJSON(w,status,map[string]string{"erro":"Não foi possível ativar o gateway."});return};writeJSON(w,200,map[string]bool{"ok":true})}
func (s *Server) adminActivateAllGateways(w http.ResponseWriter,r *http.Request){if _,ok:=s.requireAdmin(w,r);!ok{return};if err:=s.deps.AdminGateways.ActivateAll(r.Context());err!=nil{writeJSON(w,500,map[string]string{"erro":"Não foi possível ativar os gateways."});return};writeJSON(w,200,map[string]bool{"ok":true})}
func (s *Server) adminGatewayWebhookSummary(w http.ResponseWriter,r *http.Request){if _,ok:=s.requireAdmin(w,r);!ok{return};out,err:=s.deps.AdminGateways.WebhookSummary(r.Context());if err!=nil{writeJSON(w,500,map[string]string{"erro":"Não foi possível carregar o resumo de webhooks."});return};writeJSON(w,200,out)}
