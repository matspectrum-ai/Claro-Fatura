package payment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
)

type confirmAdapter struct{ webhookStatus, gatewayStatus string }
func (a *confirmAdapter) Name() string { return "gw" }
func (a *confirmAdapter) Configured(gateway.Record) bool { return true }
func (a *confirmAdapter) CreatePIX(context.Context,gateway.CreateInput)(gateway.CreatedPIX,error){return gateway.CreatedPIX{},nil}
func (a *confirmAdapter) Status(context.Context,string,gateway.Record)(*string,error){v:=a.gatewayStatus;return &v,nil}
func (a *confirmAdapter) Paid(v *string)bool{return v!=nil&&*v=="PAID"}
func (a *confirmAdapter) ReadWebhook(*http.Request,[]byte,gateway.Record)(gateway.WebhookRead,error){id:="ext";st:=a.webhookStatus;ev:="paid";return gateway.WebhookRead{Valid:true,TransactionID:&id,Status:&st,Event:&ev},nil}
type confirmRegistry struct{a gateway.Adapter}
func(r confirmRegistry)AdapterFor(gateway.Record)gateway.Adapter{return r.a}
type confirmStore struct{mu sync.Mutex;tx ConfirmationTransaction;paid,cancelled,invoicePaid,paymentConfirmed bool;status string;logs []WebhookLog}
func(s *confirmStore)GatewayBySlug(context.Context,string)(gateway.Record,bool,error){return gateway.Record{ID:"g",Slug:"gw",Adapter:"gw"},true,nil}
func(s *confirmStore)ConfirmationTransaction(context.Context,string)(ConfirmationTransaction,bool,error){s.mu.Lock();defer s.mu.Unlock();return s.tx,true,nil}
func(s *confirmStore)TransactionByGatewayReference(context.Context,string,string)(ConfirmationTransaction,bool,error){s.mu.Lock();defer s.mu.Unlock();return s.tx,true,nil}
func(s *confirmStore)MarkTransactionPaid(_ context.Context,tx ConfirmationTransaction,_ time.Time)error{s.mu.Lock();defer s.mu.Unlock();s.paid=true;s.tx.Status="pago";return nil}
func(s *confirmStore)CancelOtherPendingTransactions(context.Context,string,string,time.Time)error{s.cancelled=true;return nil}
func(s *confirmStore)MarkInvoicePaid(context.Context,string,time.Time)error{s.invoicePaid=true;return nil}
func(s *confirmStore)ConfirmOrInsertPayment(context.Context,ConfirmationTransaction,time.Time)error{s.paymentConfirmed=true;return nil}
func(s *confirmStore)UpdateTransactionStatus(_ context.Context,_ string,status string)error{s.status=status;return nil}
func(s *confirmStore)InsertWebhookLog(_ context.Context,l WebhookLog)error{s.logs=append(s.logs,l);return nil}
func TestWebhookPaidRequiresGatewayDoubleCheckBeforeConfirming(t *testing.T){store:=&confirmStore{tx:ConfirmationTransaction{ID:"tx",InvoiceID:"inv",GatewaySlug:"gw",AmountCents:1000,Status:"pendente",GatewayTransactionID:strptr("ext")}};adapter:=&confirmAdapter{webhookStatus:"PAID",gatewayStatus:"PENDING"};svc:=NewWebhookService(NewConfirmer(store,confirmRegistry{adapter}));res:=svc.Handle(context.Background(),httptest.NewRequest(http.MethodPost,"/",nil),"gw",[]byte(`{}`));if res.Status!=http.StatusAccepted||store.paid{t.Fatalf("res=%+v paid=%v",res,store.paid)};adapter.gatewayStatus="PAID";res=svc.Handle(context.Background(),httptest.NewRequest(http.MethodPost,"/",nil),"gw",[]byte(`{}`));if res.Status!=http.StatusOK||!store.paid||!store.cancelled||!store.invoicePaid||!store.paymentConfirmed{t.Fatalf("res=%+v store=%+v",res,store)};if len(store.logs)!=2{t.Fatalf("logs=%d",len(store.logs))}}
func TestConfirmIsIdempotentForAlreadyPaidTransaction(t *testing.T){store:=&confirmStore{tx:ConfirmationTransaction{ID:"tx",InvoiceID:"inv",GatewaySlug:"gw",Status:"pago"}};if err:=NewConfirmer(store,confirmRegistry{&confirmAdapter{}}).Confirm(context.Background(),"tx");err!=nil{t.Fatal(err)};if store.paid||store.invoicePaid||store.paymentConfirmed{t.Fatalf("unexpected writes: %+v",store)}}
func strptr(v string)*string{return &v}
