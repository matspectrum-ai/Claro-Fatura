package supabase

import (
	"context"
	"net/url"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/admin"
	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
)

const gatewayAdminColumns = "id,slug,rotulo,adapter,ativo,prioridade,api_url,ambiente,limite_diario,webhook_url,secret_names,observacoes"

func (c *Client) ListGatewayRecords(ctx context.Context) ([]gateway.Record, error) { var rows []gateway.Record; q:=url.Values{"select":{gatewayAdminColumns},"order":{"prioridade.asc"}}; if err:=c.Select(ctx,"gateways_config",q,&rows);err!=nil{return nil,err};return rows,nil }
func (c *Client) ReadGatewayRouting(ctx context.Context) (admin.RoutingConfig,bool,error) { var rows []admin.RoutingConfig;q:=url.Values{"select":{"estrategia,gateway_fixa,novo_pix_por_acesso"},"id":{"eq.true"},"limit":{"1"}};if err:=c.Select(ctx,"roteamento_config",q,&rows);err!=nil{return admin.RoutingConfig{},false,err};if len(rows)==0{return admin.RoutingConfig{},false,nil};return rows[0],true,nil }
func (c *Client) SaveGatewayRouting(ctx context.Context,in admin.RoutingInput)error{body:=map[string]any{"estrategia":in.Strategy,"gateway_fixa":nil};if in.Strategy=="fixa"&&in.FixedGateway!=nil{body["gateway_fixa"]=*in.FixedGateway};if in.NewPIXPerAccess!=nil{body["novo_pix_por_acesso"]=*in.NewPIXPerAccess};return c.Update(ctx,"roteamento_config",url.Values{"id":{"eq.true"}},body)}

type gatewayAdminWrite struct{Slug string `json:"slug"`;Label string `json:"rotulo"`;Adapter string `json:"adapter"`;Active bool `json:"ativo"`;Priority int `json:"prioridade"`;APIURL *string `json:"api_url"`;Environment string `json:"ambiente"`;DailyLimit *int `json:"limite_diario"`;WebhookURL *string `json:"webhook_url"`;SecretNames []string `json:"secret_names"`;Observations *string `json:"observacoes"`;UpdatedAt string `json:"updated_at"`}
func gatewayWrite(r gateway.Record,at time.Time)gatewayAdminWrite{return gatewayAdminWrite{Slug:r.Slug,Label:r.Label,Adapter:r.Adapter,Active:r.Active,Priority:r.Priority,APIURL:r.APIURL,Environment:r.Environment,DailyLimit:r.DailyLimit,WebhookURL:r.WebhookURL,SecretNames:r.SecretNames,Observations:r.Observations,UpdatedAt:at.UTC().Format(time.RFC3339Nano)}}
func(c *Client)InsertGatewayRecord(ctx context.Context,r gateway.Record,at time.Time)error{return c.Insert(ctx,"gateways_config",gatewayWrite(r,at))}
func(c *Client)UpdateGatewayRecord(ctx context.Context,id string,r gateway.Record,at time.Time)error{return c.Update(ctx,"gateways_config",url.Values{"id":{"eq."+id}},gatewayWrite(r,at))}
func(c *Client)PatchGatewayRecord(ctx context.Context,id string,active *bool,priority *int,at time.Time)error{body:=map[string]any{"updated_at":at.UTC().Format(time.RFC3339Nano)};if active!=nil{body["ativo"]=*active};if priority!=nil{body["prioridade"]=*priority};return c.Update(ctx,"gateways_config",url.Values{"id":{"eq."+id}},body)}
func(c *Client)DeleteGatewayRecord(ctx context.Context,id string)error{var rows []map[string]any;return c.DeleteReturning(ctx,"gateways_config",url.Values{"id":{"eq."+id}},&rows)}
func(c *Client)UseOnlyGateway(ctx context.Context,id string)error{if err:=c.Update(ctx,"gateways_config",url.Values{"id":{"neq."+id}},map[string]any{"ativo":false});err!=nil{return err};return c.Update(ctx,"gateways_config",url.Values{"id":{"eq."+id}},map[string]any{"ativo":true})}
func(c *Client)ActivateAllGateways(ctx context.Context)error{return c.Update(ctx,"gateways_config",url.Values{"slug":{"neq."}},map[string]any{"ativo":true})}
func(c *Client)WebhookGatewayRows(ctx context.Context,limit int)([]admin.WebhookGatewayRow,error){var rows []admin.WebhookGatewayRow;q:=url.Values{"select":{"gateway_slug,created_at"},"order":{"created_at.desc"},"limit":{itoaAdmin(limit)}};if err:=c.Select(ctx,"webhooks_log",q,&rows);err!=nil{return nil,err};return rows,nil}
func itoaAdmin(n int)string{if n<=0{return "0"};b:=[20]byte{};i:=len(b);for n>0{i--;b[i]=byte('0'+n%10);n/=10};return string(b[i:])}
