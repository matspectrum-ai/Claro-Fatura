package admin

import (
	"context"
	"testing"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
)

type gatewayStoreStub struct { rows []gateway.Record; routing RoutingConfig; routingOK bool; saved RoutingInput; inserted gateway.Record; updated gateway.Record; patchedID string; patchedActive *bool; patchedPriority *int; deleted string; only string; activated bool; hooks []WebhookGatewayRow }
func(s *gatewayStoreStub)ListGatewayRecords(context.Context)([]gateway.Record,error){return s.rows,nil}
func(s *gatewayStoreStub)ReadGatewayRouting(context.Context)(RoutingConfig,bool,error){return s.routing,s.routingOK,nil}
func(s *gatewayStoreStub)SaveGatewayRouting(_ context.Context,in RoutingInput)error{s.saved=in;return nil}
func(s *gatewayStoreStub)InsertGatewayRecord(_ context.Context,r gateway.Record,_ time.Time)error{s.inserted=r;return nil}
func(s *gatewayStoreStub)UpdateGatewayRecord(_ context.Context,_ string,r gateway.Record,_ time.Time)error{s.updated=r;return nil}
func(s *gatewayStoreStub)PatchGatewayRecord(_ context.Context,id string,a *bool,p *int,_ time.Time)error{s.patchedID=id;s.patchedActive=a;s.patchedPriority=p;return nil}
func(s *gatewayStoreStub)DeleteGatewayRecord(_ context.Context,id string)error{s.deleted=id;return nil}
func(s *gatewayStoreStub)UseOnlyGateway(_ context.Context,id string)error{s.only=id;return nil}
func(s *gatewayStoreStub)ActivateAllGateways(context.Context)error{s.activated=true;return nil}
func(s *gatewayStoreStub)WebhookGatewayRows(context.Context,int)([]WebhookGatewayRow,error){return s.hooks,nil}

func TestGatewayConfiguredMatchesOriginalEnvRules(t *testing.T){env:=map[string]string{"CASHINPAY_SECRET_KEY":"x","PROPIX_CLIENT_ID":"a","PROPIX_CLIENT_SECRET":"b","GEN_TOKEN":"x"};lookup:=func(k string)string{return env[k]};cases:=[]struct{record gateway.Record;want bool}{{gateway.Record{Adapter:"cashinpay"},true},{gateway.Record{Adapter:"propix"},true},{gateway.Record{Adapter:"m2pay"},false},{gateway.Record{Adapter:"generico",SecretNames:[]string{"GEN_TOKEN"}},true},{gateway.Record{Adapter:"generico",SecretNames:[]string{"GEN_TOKEN","GEN_SECRET"}},false}};for _,tc:=range cases{if got:=gatewayConfigured(tc.record,lookup);got!=tc.want{t.Fatalf("%s got=%v want=%v",tc.record.Adapter,got,tc.want)}}}
func TestRoutingDefaultsAndClearsFixedOutsideFixedStrategy(t *testing.T){store:=&gatewayStoreStub{};s:=NewGatewayAdminService(store,nil);got,err:=s.Routing(context.Background());if err!=nil{t.Fatal(err)};if got.Strategy!="prioridade"||!got.NewPIXPerAccess{t.Fatalf("got=%+v",got)};id:="550e8400-e29b-41d4-a716-446655440000";v:=false;if err:=s.SaveRouting(context.Background(),RoutingInput{Strategy:"rodizio",FixedGateway:&id,NewPIXPerAccess:&v});err!=nil{t.Fatal(err)};if store.saved.FixedGateway!=nil||store.saved.NewPIXPerAccess==nil||*store.saved.NewPIXPerAccess{t.Fatalf("saved=%+v",store.saved)}}
func TestGatewayValidationAndNormalization(t *testing.T){store:=&gatewayStoreStub{};s:=NewGatewayAdminService(store,nil);api:=" https://api.example.com/v1 ";wh:=" https://site.example.com/hook ";obs:=" ok ";in:=GatewayInput{Slug:" Minha-Gateway ",Label:" Minha Gateway ",Adapter:"generico",APIURL:&api,Environment:"producao",Priority:50,WebhookURL:&wh,SecretNames:[]string{" TOKEN_A ","TOKEN_B"},Observations:&obs,Active:true};if err:=s.Save(context.Background(),in);err!=nil{t.Fatal(err)};if store.inserted.Slug!="minha-gateway"||store.inserted.Label!="Minha Gateway"||store.inserted.APIURL==nil||*store.inserted.APIURL!="https://api.example.com/v1"||len(store.inserted.SecretNames)!=2||store.inserted.SecretNames[0]!="TOKEN_A"{t.Fatalf("inserted=%+v",store.inserted)};bad:=in;bad.Slug="INVÁLIDO";if err:=s.Save(context.Background(),bad);err!=ErrInvalidGateway{t.Fatalf("err=%v",err)}}
func TestWebhookSummaryKeepsLatestAndCounts24Hours(t *testing.T){now:=time.Date(2026,9,1,12,0,0,0,time.UTC);store:=&gatewayStoreStub{hooks:[]WebhookGatewayRow{{Gateway:"g1",At:now.Add(-time.Hour).Format(time.RFC3339)},{Gateway:"g1",At:now.Add(-25*time.Hour).Format(time.RFC3339)},{Gateway:"g2",At:now.Add(-2*time.Hour).Format(time.RFC3339)}}};s:=NewGatewayAdminService(store,nil);s.now=func()time.Time{return now};got,err:=s.WebhookSummary(context.Background());if err!=nil{t.Fatal(err)};if len(got)!=2||got[0].Gateway!="g1"||got[0].Last24h!=1||got[0].LastAt==nil||*got[0].LastAt!=store.hooks[0].At{t.Fatalf("got=%+v",got)}}
