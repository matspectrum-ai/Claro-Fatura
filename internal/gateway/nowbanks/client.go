package nowbanks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
)

const defaultBaseURL = "https://api.nowbanks.com.br/v1"

type tokenEntry struct { token string; expires time.Time }
type Client struct { clientID, clientSecret, webhookSecret, baseURL string; http *http.Client; mu sync.Mutex; token tokenEntry }

func New(clientID, clientSecret, webhookSecret string) *Client { return &Client{clientID: clientID, clientSecret: clientSecret, webhookSecret: webhookSecret, baseURL: defaultBaseURL, http: &http.Client{Timeout: 30*time.Second, Transport: &http.Transport{MaxIdleConns:128, MaxIdleConnsPerHost:64, IdleConnTimeout:90*time.Second}}} }
func (c *Client) Name() string { return "nowbanks" }
func (c *Client) Configured(gateway.Record) bool { return strings.TrimSpace(c.clientID)!="" && strings.TrimSpace(c.clientSecret)!="" }
func (c *Client) tokenFor(ctx context.Context)(string,error){
	c.mu.Lock(); defer c.mu.Unlock(); now:=time.Now(); if c.token.token!="" && c.token.expires.Add(-time.Minute).After(now){ return c.token.token,nil }
	body,_:=json.Marshal(map[string]any{"client_id":c.clientID,"client_secret":c.clientSecret}); req,e:=http.NewRequestWithContext(ctx,http.MethodPost,c.baseURL+"/auth/login",bytes.NewReader(body)); if e!=nil{return "",e}; req.Header.Set("Content-Type","application/json"); req.Header.Set("Accept","application/json"); resp,e:=c.http.Do(req); if e!=nil{return "",e}; defer resp.Body.Close(); raw,_:=io.ReadAll(io.LimitReader(resp.Body,1<<20)); if resp.StatusCode<200||resp.StatusCode>=300{return "",errors.New("não foi possível autenticar na NowBanks")}; var out struct{AccessToken string `json:"access_token"`; ExpiresIn float64 `json:"expires_in"`}; if json.Unmarshal(raw,&out)!=nil{return "",errors.New("resposta de autenticação inválida da NowBanks")}; if out.AccessToken==""{return "",errors.New("NowBanks não retornou access_token")}; if out.ExpiresIn==0{out.ExpiresIn=3600}; c.token=tokenEntry{token:out.AccessToken,expires:now.Add(time.Duration(out.ExpiresIn*float64(time.Second)))}; return out.AccessToken,nil
}
func (c *Client) authRequest(ctx context.Context,method,path string,body any,retry bool)([]byte,int,error){token,e:=c.tokenFor(ctx);if e!=nil{return nil,0,e};var rd io.Reader;if body!=nil{b,er:=json.Marshal(body);if er!=nil{return nil,0,er};rd=bytes.NewReader(b)};req,e:=http.NewRequestWithContext(ctx,method,c.baseURL+path,rd);if e!=nil{return nil,0,e};req.Header.Set("Content-Type","application/json");req.Header.Set("Accept","application/json");req.Header.Set("Authorization","Bearer "+token);resp,e:=c.http.Do(req);if e!=nil{return nil,0,e};defer resp.Body.Close();raw,_:=io.ReadAll(io.LimitReader(resp.Body,1<<20));if resp.StatusCode==401&&retry{c.mu.Lock();c.token=tokenEntry{};c.mu.Unlock();return c.authRequest(ctx,method,path,body,false)};return raw,resp.StatusCode,nil}
func (c *Client) CreatePIX(ctx context.Context,in gateway.CreateInput)(gateway.CreatedPIX,error){if !c.Configured(in.Gateway){return gateway.CreatedPIX{},errors.New("credenciais NowBanks não configuradas")};doc:=digits(deref(in.Document));body:=map[string]any{"amount":float64(in.AmountCents)/100,"external_id":in.Reference,"payer":map[string]any{"name":truncate(name(in.Name),100),"document":doc},"clientCallbackUrl":in.WebhookURL};raw,status,e:=c.authRequest(ctx,http.MethodPost,"/payments/deposit",body,true);if e!=nil{return gateway.CreatedPIX{},e};if status<200||status>=300{return gateway.CreatedPIX{},errors.New(friendly(status))};var out map[string]any;if json.Unmarshal(raw,&out)!=nil{return gateway.CreatedPIX{},errors.New("resposta inválida da NowBanks")};copyPaste:=str(out["pix_copy_paste"]);if copyPaste==""{return gateway.CreatedPIX{},errors.New("resposta sem pix_copy_paste")};id:=str(out["transaction_id"]);st:=str(out["status"]);if st==""{st="PENDING"};qr:=str(out["pix_qr_code"]);return gateway.CreatedPIX{TransactionID:id,CopyPaste:copyPaste,QRCode:ptr(qr),Status:st},nil}
func (c *Client) Status(ctx context.Context,id string,_ gateway.Record)(*string,error){raw,status,e:=c.authRequest(ctx,http.MethodGet,"/transactions/"+url.PathEscape(id),nil,true);if e!=nil||status<200||status>=300{return nil,nil};var out map[string]any;if json.Unmarshal(raw,&out)!=nil{return nil,nil};data:=asMap(out["data"]);if data==nil{data=out};return ptr(str(data["status"])),nil}
func (c *Client) Paid(status *string)bool{return status!=nil&&strings.EqualFold(strings.TrimSpace(*status),"COMPLETED")}
func (c *Client) ReadWebhook(req *http.Request,raw []byte,_ gateway.Record)(gateway.WebhookRead,error){var body map[string]any;if json.Unmarshal(raw,&body)!=nil{return gateway.WebhookRead{Valid:false},nil};return c.readWebhookWithHeader(raw,body,req.Header.Get("X-Signature"))}
func (c *Client) readWebhookWithHeader(raw []byte,body map[string]any,signature string)(gateway.WebhookRead,error){valid:=true;if c.webhookSecret!=""{sent:=strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(signature,"sha256="),"SHA256="));mac:=hmac.New(sha256.New,[]byte(c.webhookSecret));_,_=mac.Write(raw);expected:=hex.EncodeToString(mac.Sum(nil));valid=len(sent)==len(expected)&&subtle.ConstantTimeCompare([]byte(sent),[]byte(expected))==1};data:=asMap(body["data"]);if data==nil{data=body};id:=search(data,[]string{"transaction_id","transactionId","id"},0);st:=search(data,[]string{"status"},0);ev:=search(body,[]string{"event","type"},0);return gateway.WebhookRead{Valid:valid,TransactionID:ptr(id),Status:ptr(st),Event:ptr(ev)},nil}
func friendly(status int)string{switch status{case 400:return "não foi possível gerar o PIX. Verifique os dados e tente novamente";case 401,403:return "pagamento indisponível no momento. Tente novamente em instantes";case 404:return "cobrança não encontrada";default:return "serviço de pagamento temporariamente indisponível. Tente novamente"}}
func digits(v string)string{var b strings.Builder;for _,r:=range v{if r>='0'&&r<='9'{b.WriteRune(r)}};return b.String()}
func deref(v *string)string{if v==nil{return ""};return *v}
func name(v string)string{v=strings.TrimSpace(v);if v==""{return "Cliente"};return v}
func truncate(v string,n int)string{r:=[]rune(v);if len(r)>n{r=r[:n]};return string(r)}
func str(v any)string{if v==nil{return ""};return fmt.Sprint(v)}
func ptr(v string)*string{if v==""{return nil};return &v}
func asMap(v any)map[string]any{m,_:=v.(map[string]any);return m}
func search(v any,keys []string,depth int)string{if depth>6||v==nil{return ""};m,ok:=v.(map[string]any);if !ok{return ""};for _,k:=range keys{if x:=m[k];x!=nil{if _,nested:=x.(map[string]any);!nested{return fmt.Sprint(x)}}};for _,x:=range m{if y:=search(x,keys,depth+1);y!=""{return y}};return ""}
