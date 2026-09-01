package supabase

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/admin"
)

func (c *Client) UpsertClients(ctx context.Context, rows []admin.ClientUpsert) ([]admin.ClientRef, error) {
	var saved []admin.ClientRef
	q := url.Values{"on_conflict": {"telefone"}, "select": {"id,telefone"}}
	if err := c.UpsertReturning(ctx, "clientes", q, rows, &saved); err != nil { return nil, err }
	return saved, nil
}
func (c *Client) PendingInvoices(ctx context.Context, clientIDs, statuses []string) ([]admin.PendingInvoice, error) {
	if len(clientIDs) == 0 { return nil, nil }
	var rows []admin.PendingInvoice
	q := url.Values{"select": {"id,cliente_id,vencimento"}, "cliente_id": {"in.(" + strings.Join(clientIDs, ",") + ")"}, "status": {"in.(" + strings.Join(statuses, ",") + ")"}, "order": {"vencimento.desc"}}
	if err := c.Select(ctx, "faturas", q, &rows); err != nil { return nil, err }
	return rows, nil
}
func (c *Client) UpsertInvoices(ctx context.Context, rows []admin.InvoiceWrite) (int, error) {
	var saved []struct{ ID string `json:"id"` }
	q := url.Values{"on_conflict": {"id"}, "select": {"id"}}
	if err := c.UpsertReturning(ctx, "faturas", q, rows, &saved); err != nil { return 0, err }
	return len(saved), nil
}
func (c *Client) InsertInvoices(ctx context.Context, rows []admin.InvoiceWrite) (int, error) {
	var saved []struct{ ID string `json:"id"` }
	q := url.Values{"select": {"id"}}
	if err := c.InsertReturning(ctx, "faturas", q, rows, &saved); err != nil { return 0, err }
	return len(saved), nil
}
func (c *Client) ListInvoices(ctx context.Context, search string, page, size int) (admin.InvoicePage, error) {
	var rows []admin.InvoiceRow
	q := url.Values{"select": {"id,cliente_id,descricao,valor_original,valor_desconto,vencimento,status,pix_copia_cola,boleto_codigo,clientes!inner(nome,telefone)"}, "order": {"vencimento.desc"}}
	if search != "" { d := onlyDigits(search); if len(d) >= 3 { q.Set("clientes.telefone", "ilike.*"+d+"*") } else { q.Set("clientes.nome", "ilike.*"+search+"*") } }
	from := page * size
	total, err := c.SelectRange(ctx, "faturas", q, from, from+size-1, &rows)
	if err != nil { return admin.InvoicePage{}, err }
	return admin.InvoicePage{Rows: rows, Total: total}, nil
}
func (c *Client) UpdateClient(ctx context.Context, id, name, phone string) error { return c.Update(ctx, "clientes", url.Values{"id": {"eq." + id}}, map[string]any{"nome": name, "telefone": phone}) }
func (c *Client) UpdateInvoice(ctx context.Context, id string, in admin.InvoiceEdit) error { return c.Update(ctx, "faturas", url.Values{"id": {"eq." + id}}, map[string]any{"valor_original": in.Original, "valor_desconto": in.Discount, "vencimento": in.DueDate, "status": in.Status}) }
func (c *Client) SetInvoiceStatus(ctx context.Context, id, status string) error { return c.Update(ctx, "faturas", url.Values{"id": {"eq." + id}}, map[string]any{"status": status}) }
func (c *Client) ManualPaymentInvoice(ctx context.Context, id string) (admin.ManualPaymentInvoice, bool, error) {
	var rows []struct { ID string `json:"id"`; ClientID string `json:"cliente_id"`; Original float64 `json:"valor_original"`; Discount float64 `json:"valor_desconto"` }
	q := url.Values{"select": {"id,cliente_id,valor_original,valor_desconto"}, "id": {"eq." + id}, "limit": {"1"}}
	if err := c.Select(ctx, "faturas", q, &rows); err != nil { return admin.ManualPaymentInvoice{}, false, err }
	if len(rows) == 0 { return admin.ManualPaymentInvoice{}, false, nil }
	r := rows[0]
	return admin.ManualPaymentInvoice{ID:r.ID, ClientID:r.ClientID, Original:r.Original, Discount:r.Discount}, true, nil
}
func (c *Client) InsertManualPayment(ctx context.Context, in admin.ManualPaymentInvoice, at time.Time) error { value := in.Discount; if value == 0 { value = in.Original }; return c.Insert(ctx, "pagamentos", map[string]any{"fatura_id":in.ID,"cliente_id":in.ClientID,"valor":value,"metodo":"manual","status":"confirmado","pago_em":at.Format(time.RFC3339Nano)}) }
func (c *Client) DeleteAll(ctx context.Context) (admin.DeleteAllResult, error) {
	const never = "00000000-0000-0000-0000-000000000000"
	q := url.Values{"id": {"neq." + never}, "select": {"id"}}
	var p, f, cl []struct{ ID string `json:"id"` }
	if err := c.DeleteReturning(ctx, "pagamentos", q, &p); err != nil { return admin.DeleteAllResult{}, err }
	if err := c.DeleteReturning(ctx, "faturas", q, &f); err != nil { return admin.DeleteAllResult{}, err }
	if err := c.DeleteReturning(ctx, "clientes", q, &cl); err != nil { return admin.DeleteAllResult{}, err }
	return admin.DeleteAllResult{Payments:len(p),Invoices:len(f),Clients:len(cl)}, nil
}
func onlyDigits(v string) string { var b strings.Builder; for _, r := range v { if r >= '0' && r <= '9' { b.WriteRune(r) } }; return b.String() }
func (c *Client) AccessRecords(ctx context.Context, limit int) ([]admin.AccessRecord, error) { var rows []admin.AccessRecord; q := url.Values{"select":{"id,data_hora,pagina,telefone_consultado,sucesso,valor_original,valor_desconto"},"order":{"data_hora.desc"},"limit":{fmt.Sprint(limit)}}; if err := c.Select(ctx,"acessos",q,&rows); err != nil { return nil,err }; return rows,nil }
func (c *Client) CountClients(ctx context.Context) (int, error) { return c.Count(ctx,"clientes",url.Values{}) }
func (c *Client) ClearAccesses(ctx context.Context) (int, error) { var rows []struct{ID string `json:"id"`}; q:=url.Values{"id":{"neq.00000000-0000-0000-0000-000000000000"},"select":{"id"}}; if err:=c.DeleteReturning(ctx,"acessos",q,&rows);err!=nil{return 0,err};return len(rows),nil }
