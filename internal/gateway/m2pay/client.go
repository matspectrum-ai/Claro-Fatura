package m2pay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
)

const defaultBaseURL = "https://api.m2pay.pro/api"

type Client struct {
	apiKey string
	baseURL string
	http *http.Client
	productName string
	customerEmail string
}

func New(apiKey, productName, customerEmail string) *Client {
	if strings.TrimSpace(productName)=="" { productName=gateway.DefaultProductName }
	if strings.TrimSpace(customerEmail)=="" { customerEmail=gateway.DefaultCustomerEmail }
	return &Client{apiKey:apiKey, baseURL:defaultBaseURL, http:&http.Client{Timeout:30*time.Second, Transport:&http.Transport{MaxIdleConns:128,MaxIdleConnsPerHost:64,IdleConnTimeout:90*time.Second}}, productName:productName, customerEmail:customerEmail}
}
func (c *Client) Name() string { return "m2pay" }
func (c *Client) Configured(gateway.Record) bool { return strings.TrimSpace(c.apiKey)!="" }
func (c *Client) CreatePIX(ctx context.Context, in gateway.CreateInput) (gateway.CreatedPIX,error) {
	if !c.Configured(in.Gateway) { return gateway.CreatedPIX{}, errors.New("credencial M2 Pay (M2PAY_API_KEY) não configurada") }
	cents:=in.AmountCents
	doc:=digits(deref(in.Document)); if len(doc)!=11 { doc="00000000000" }
	phone:=digits(in.Phone); if len(phone)>11 { phone=phone[len(phone)-11:] }; if phone=="" { phone="11999999999" }
	body:=map[string]any{
		"amount":cents,"paymentMethod":"pix",
		"items":[]map[string]any{{"title":truncate(c.productName,100),"unitPrice":cents,"quantity":1,"tangible":false}},
		"customer":map[string]any{"name":truncate(name(in.Name),100),"email":c.customerEmail,"phone":phone,"document":map[string]any{"number":doc,"type":"cpf"}},
		"postbackUrl":in.WebhookURL,"externalRef":in.Reference,
	}
	raw,status,err:=c.request(ctx,http.MethodPost,"/sales/create-transaction",body); if err!=nil{return gateway.CreatedPIX{},err}
	if status==401 { return gateway.CreatedPIX{}, errors.New("API Key da M2 Pay inválida ou expirada") }
	if status<200||status>=300 { return gateway.CreatedPIX{},fmt.Errorf("M2 Pay HTTP %d",status) }
	var envelope map[string]any; if json.Unmarshal(raw,&envelope)!=nil{return gateway.CreatedPIX{},errors.New("resposta inválida da M2 Pay")}
	if success,ok:=envelope["success"].(bool); ok && !success { return gateway.CreatedPIX{},errors.New("M2 Pay não retornou a cobrança") }
	data:=asMap(envelope["data"]); if data==nil {data=envelope}
	pix:=asMap(data["pix"]); if pix==nil {pix=map[string]any{}}
	copyPaste:=firstString(pix,"emv","copyPaste"); if copyPaste=="" {return gateway.CreatedPIX{},errors.New("resposta sem campo pix.emv (copia e cola)")}
	id:=firstString(data,"transactionId","id"); st:=firstString(data,"status"); if st==""{st="PENDING"}
	qr:=firstString(pix,"qrcode"); exp:=firstString(pix,"expiresAt")
	return gateway.CreatedPIX{TransactionID:id,CopyPaste:copyPaste,QRCode:ptr(qr),Status:st,ExpiresAt:ptr(exp)},nil
}
func (c *Client) Status(ctx context.Context,id string,_ gateway.Record)(*string,error){
	raw,status,err:=c.request(ctx,http.MethodGet,"/sales/"+url.PathEscape(id)+"/status",nil); if err!=nil||status<200||status>=300{return nil,nil}
	var env map[string]any; if json.Unmarshal(raw,&env)!=nil{return nil,nil}; data:=asMap(env["data"]);if data==nil{data=env}; st:=firstString(data,"status","transactionStatus");return ptr(st),nil
}
func (c *Client) Paid(status *string) bool { return status!=nil && strings.EqualFold(strings.TrimSpace(*status),"PAID") }
func (c *Client) ReadWebhook(_ *http.Request,raw []byte,_ gateway.Record)(gateway.WebhookRead,error){var body map[string]any;if json.Unmarshal(raw,&body)!=nil{return gateway.WebhookRead{Valid:false},nil};data:=asMap(body["data"]);if data==nil{data=body};id:=search(data,[]string{"transactionId","transaction_id","id"},0);st:=search(data,[]string{"status","transactionStatus"},0);ev:=search(body,[]string{"event","type"},0);return gateway.WebhookRead{Valid:true,TransactionID:ptr(id),Status:ptr(st),Event:ptr(ev)},nil}
func(c *Client)request(ctx context.Context,method,path string,body any)([]byte,int,error){var rd *bytes.Reader;if body!=nil{b,e:=json.Marshal(body);if e!=nil{return nil,0,e};rd=bytes.NewReader(b)}else{rd=bytes.NewReader(nil)};req,e:=http.NewRequestWithContext(ctx,method,c.baseURL+path,rd);if e!=nil{return nil,0,e};req.Header.Set("Content-Type","application/json");req.Header.Set("X-API-Key",c.apiKey);resp,e:=c.http.Do(req);if e!=nil{return nil,0,e};defer resp.Body.Close();raw,_:=io.ReadAll(io.LimitReader(resp.Body,1<<20));return raw,resp.StatusCode,nil}
func digits(v string)string{var b strings.Builder;for _,r:=range v{if r>='0'&&r<='9'{b.WriteRune(r)}};return b.String()}
func deref(v *string)string{if v==nil{return ""};return *v}
func name(v string)string{v=strings.TrimSpace(v);if v==""{return "Cliente"};return v}
func truncate(v string,n int)string{r:=[]rune(v);if len(r)>n{r=r[:n]};return string(r)}
func asMap(v any)map[string]any{m,_:=v.(map[string]any);return m}
func firstString(m map[string]any,keys ...string)string{for _,k:=range keys{if v:=m[k];v!=nil{return fmt.Sprint(v)}};return ""}
func ptr(v string)*string{if v==""{return nil};return &v}
func search(v any,keys []string,depth int)string{if depth>6||v==nil{return ""};m,ok:=v.(map[string]any);if !ok{return ""};for _,k:=range keys{if x:=m[k];x!=nil{if _,isMap:=x.(map[string]any);!isMap{return fmt.Sprint(x)}}};for _,x:=range m{if y:=search(x,keys,depth+1);y!=""{return y}};return ""}
