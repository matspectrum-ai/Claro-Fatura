package legacy

import (
	"context"
	"errors"
	"testing"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
	"github.com/matspectrum-ai/Claro-Fatura/internal/payment"
)

type storeStub struct{view InvoiceView;viewOK bool;client Client;clientOK bool;invoice Invoice;invoiceOK bool;pendingID string;pendingOK bool;open OpenInvoice;openOK bool;propixExists bool;inserted bool}
func(s *storeStub)LegacyInvoiceViewByPhone(context.Context,string)(InvoiceView,bool,error){return s.view,s.viewOK,nil}
func(s *storeStub)LegacyClientByPhone(context.Context,string)(Client,bool,error){return s.client,s.clientOK,nil}
func(s *storeStub)LegacyClientByID(context.Context,string)(Client,bool,error){return s.client,s.clientOK,nil}
func(s *storeStub)LegacyPendingInvoiceID(context.Context,string)(string,bool,error){return s.pendingID,s.pendingOK,nil}
func(s *storeStub)LegacyInvoiceByID(context.Context,string)(Invoice,bool,error){return s.invoice,s.invoiceOK,nil}
func(s *storeStub)LegacyFirstOpenInvoice(context.Context)(OpenInvoice,bool,error){return s.open,s.openOK,nil}
func(s *storeStub)LegacyProPixExists(context.Context)(bool,error){return s.propixExists,nil}
func(s *storeStub)LegacyInsertProPix(context.Context)error{s.inserted=true;return nil}
type routerStub struct{request payment.CreateRequest;tx *payment.Transaction;err error;calls int}
func(r *routerStub)CreatePIX(_ context.Context,in payment.CreateRequest)(*payment.Transaction,error){r.calls++;r.request=in;return r.tx,r.err}
type directStub struct{input gateway.CreateInput;created gateway.CreatedPIX;err error;calls int}
func(d *directStub)CreatePIX(_ context.Context,in gateway.CreateInput)(gateway.CreatedPIX,error){d.calls++;d.input=in;return d.created,d.err}
func TestCreateChargePreservesDiscountAndRouterContract(t *testing.T){copyPaste:="000201";external:="gw-1";store:=&storeStub{client:Client{ID:"c1",Name:"Ana",Phone:"11999999999"},clientOK:true,invoice:Invoice{ID:"f1",CustomerID:"c1",DiscountAmount:49.90,DueDate:"2026-09-30",Status:"em_aberto"},invoiceOK:true};router:=&routerStub{tx:&payment.Transaction{GatewaySlug:"propix",GatewayTransactionID:&external,CopyPaste:&copyPaste}};s:=New(store,router,nil,"https://configured.example");s.randomUUID=func()(string,error){return "11111111-1111-4111-8111-111111111111",nil};out,err:=s.CreateCharge(context.Background(),"","f1","https://request.example");if err!=nil{t.Fatal(err)};if out.Amount!=49.90||out.DueDate!="2026-09-30"||out.PIXCopyPaste!="000201"||out.Gateway!="propix"{t.Fatalf("result=%+v",out)};if router.calls!=1||router.request.AmountCents!=4990||router.request.RequestKey!="11111111-1111-4111-8111-111111111111"||router.request.BaseURL!="https://request.example"{t.Fatalf("request=%+v calls=%d",router.request,router.calls)}}
func TestCreateChargeRejectsNonPendingWithoutCallingRouter(t *testing.T){store:=&storeStub{invoice:Invoice{ID:"f1",CustomerID:"c1",DiscountAmount:10,Status:"paga"},invoiceOK:true};router:=&routerStub{};_,err:=New(store,router,nil,"").CreateCharge(context.Background(),"","f1","");if !errors.Is(err,ErrNotPending)||router.calls!=0{t.Fatalf("err=%v calls=%d",err,router.calls)}}
func TestSetupProPixIsIdempotent(t *testing.T){store:=&storeStub{propixExists:true};s:=New(store,&routerStub{},nil,"");out,err:=s.SetupProPix(context.Background());if err!=nil{t.Fatal(err)};if store.inserted||out["message"]!="ProPix já existe no banco."{t.Fatalf("out=%v inserted=%v",out,store.inserted)};store.propixExists=false;out,err=s.SetupProPix(context.Background());if err!=nil{t.Fatal(err)};if !store.inserted||out["message"]!="ProPix inserido com sucesso."{t.Fatalf("out=%v inserted=%v",out,store.inserted)}}
func TestDiagnosticEndpointsUseInjectedFakesOnly(t *testing.T){copyPaste:="x";store:=&storeStub{open:OpenInvoice{ID:"f1",CustomerID:"c1",DiscountAmount:1.23},openOK:true,client:Client{ID:"c1",Name:"Teste",Phone:"11999999999"},clientOK:true};router:=&routerStub{tx:&payment.Transaction{ID:"tx",CopyPaste:&copyPaste,GatewaySlug:"propix"}};direct:=&directStub{created:gateway.CreatedPIX{TransactionID:"p1",CopyPaste:"code",Status:"pending"}};s:=New(store,router,direct,"");flow:=s.TestFlow(context.Background());if flow["success"]!=true||router.calls!=1||!stringsHasPrefix(router.request.RequestKey,"TESTE-ROUTER-"){t.Fatalf("flow=%v request=%+v",flow,router.request)};diag:=s.TestProPixDirect(context.Background());if diag["success"]!=true||direct.calls!=1||direct.input.AmountCents!=100||direct.input.Name!="Teste Diagnostico"{t.Fatalf("diag=%v input=%+v",diag,direct.input)}}
func TestSecretsOnlyReturnsPresence(t *testing.T){s:=New(&storeStub{},&routerStub{},nil,"");s.env=func(k string)string{if k=="PROPIX_CLIENT_ID"{return "secret-value"};return ""};out:=s.Secrets();if !out["PROPIX_CLIENT_ID"]||out["PROPIX_CLIENT_SECRET"]||out["CASHINPAY_SECRET_KEY"]{t.Fatalf("out=%v",out)}}
func stringsHasPrefix(v,p string)bool{return len(v)>=len(p)&&v[:len(p)]==p}
