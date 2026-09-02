package supabase

import (
	"context"
	"net/url"

	"github.com/matspectrum-ai/Claro-Fatura/internal/legacy"
)

func (c *Client) LegacyInvoiceViewByPhone(ctx context.Context, phone string) (legacy.InvoiceView, bool, error) {
	var rows []legacy.InvoiceView
	q := url.Values{"select": {"telefone,nome,fatura_id,valor_em_aberto,valor_com_desconto,status,data_vencimento,pix_copia_e_cola,boleto_codigo,boleto_url,data_pagamento"}, "telefone": {"eq." + phone}, "limit": {"1"}}
	if err := c.Select(ctx, "faturas_por_telefone", q, &rows); err != nil { return legacy.InvoiceView{}, false, err }
	if len(rows) == 0 { return legacy.InvoiceView{}, false, nil }
	return rows[0], true, nil
}

func (c *Client) LegacyClientByPhone(ctx context.Context, phone string) (legacy.Client, bool, error) { return c.legacyClient(ctx, url.Values{"telefone": {"eq." + phone}}) }
func (c *Client) LegacyClientByID(ctx context.Context, id string) (legacy.Client, bool, error) { return c.legacyClient(ctx, url.Values{"id": {"eq." + id}}) }
func (c *Client) legacyClient(ctx context.Context, filters url.Values) (legacy.Client, bool, error) {
	q := url.Values{"select": {"id,nome,telefone,email,documento"}, "limit": {"1"}}
	for k, v := range filters { q[k] = v }
	var rows []struct { ID string `json:"id"`; Name string `json:"nome"`; Phone string `json:"telefone"`; Email *string `json:"email"`; Document *string `json:"documento"` }
	if err := c.Select(ctx, "clientes", q, &rows); err != nil { return legacy.Client{}, false, err }
	if len(rows) == 0 { return legacy.Client{}, false, nil }
	r := rows[0]; return legacy.Client{ID:r.ID,Name:r.Name,Phone:r.Phone,Email:r.Email,Document:r.Document}, true, nil
}
func (c *Client) LegacyPendingInvoiceID(ctx context.Context, customerID string) (string, bool, error) {
	var rows []struct{ ID string `json:"id"` }
	q := url.Values{"select":{"id"},"cliente_id":{"eq."+customerID},"status":{"in.(em_aberto,vencida)"},"order":{"vencimento.asc"},"limit":{"1"}}
	if err:=c.Select(ctx,"faturas",q,&rows);err!=nil{return "",false,err};if len(rows)==0{return "",false,nil};return rows[0].ID,true,nil
}
func (c *Client) LegacyInvoiceByID(ctx context.Context,id string)(legacy.Invoice,bool,error){
	var rows []struct{ID string `json:"id"`;CustomerID string `json:"cliente_id"`;Discount float64 `json:"valor_desconto"`;Original float64 `json:"valor_original"`;Due string `json:"vencimento"`;Status string `json:"status"`}
	q:=url.Values{"select":{"id,cliente_id,valor_desconto,valor_original,vencimento,status"},"id":{"eq."+id},"limit":{"1"}}
	if err:=c.Select(ctx,"faturas",q,&rows);err!=nil{return legacy.Invoice{},false,err};if len(rows)==0{return legacy.Invoice{},false,nil};r:=rows[0];return legacy.Invoice{ID:r.ID,CustomerID:r.CustomerID,DiscountAmount:r.Discount,OriginalAmount:r.Original,DueDate:r.Due,Status:r.Status},true,nil
}
func(c *Client)LegacyFirstOpenInvoice(ctx context.Context)(legacy.OpenInvoice,bool,error){var rows []struct{ID string `json:"id"`;CustomerID string `json:"cliente_id"`;Discount float64 `json:"valor_desconto"`};q:=url.Values{"select":{"id,cliente_id,valor_desconto"},"status":{"eq.em_aberto"},"limit":{"1"}};if err:=c.Select(ctx,"faturas",q,&rows);err!=nil{return legacy.OpenInvoice{},false,err};if len(rows)==0{return legacy.OpenInvoice{},false,nil};r:=rows[0];return legacy.OpenInvoice{ID:r.ID,CustomerID:r.CustomerID,DiscountAmount:r.Discount},true,nil}
func(c *Client)LegacyProPixExists(ctx context.Context)(bool,error){var rows []struct{ID string `json:"id"`};q:=url.Values{"select":{"id"},"adapter":{"eq.propix"},"limit":{"1"}};if err:=c.Select(ctx,"gateways_config",q,&rows);err!=nil{return false,err};return len(rows)>0,nil}
func(c *Client)LegacyInsertProPix(ctx context.Context)error{return c.Insert(ctx,"gateways_config",map[string]any{"slug":"propix","rotulo":"ProPix","adapter":"propix","ativo":false,"prioridade":2,"secret_names":[]string{"PROPIX_CLIENT_ID","PROPIX_CLIENT_SECRET"},"ambiente":"producao"})}
