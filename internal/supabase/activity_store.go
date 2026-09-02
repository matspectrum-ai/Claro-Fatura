package supabase

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/admin"
)

func (c *Client) TransactionRows(ctx context.Context, status string, limit int) ([]admin.TransactionRow, error) {
	var rows []admin.TransactionRow
	q := url.Values{"select": {"id,fatura_id,cliente_id,gateway_slug,transacao_gateway_id,valor_centavos,status,created_at,expira_em,pago_em,clientes(nome)"}, "order": {"created_at.desc"}, "limit": {fmt.Sprint(limit)}}
	if status != "" {
		q.Set("status", "eq."+status)
	}
	if err := c.Select(ctx, "transacoes_pix", q, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (c *Client) PaymentRows(ctx context.Context, gateway string, since time.Time, limit int) ([]admin.PaymentRow, error) {
	var rows []admin.PaymentRow
	q := url.Values{"select": {"id,gateway_slug,transacao_gateway_id,valor_centavos,valor_pago_centavos,pago_em,clientes(nome,telefone),faturas(descricao,status,valor_desconto)"}, "pago_em": {"not.is.null"}, "order": {"pago_em.desc"}, "limit": {fmt.Sprint(limit)}}
	q.Add("pago_em", "gte."+since.UTC().Format(time.RFC3339Nano))
	if gateway != "" {
		q.Set("gateway_slug", "eq."+gateway)
	}
	if err := c.Select(ctx, "transacoes_pix", q, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (c *Client) WebhookTransactionIDs(ctx context.Context, ids []string) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ID *string `json:"transacao_gateway_id"`
	}
	q := url.Values{"select": {"transacao_gateway_id"}, "transacao_gateway_id": {"in.(" + strings.Join(ids, ",") + ")"}}
	if err := c.Select(ctx, "webhooks_log", q, &rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.ID != nil {
			out[*r.ID] = struct{}{}
		}
	}
	return out, nil
}

func (c *Client) PaymentLogs(ctx context.Context, limit int) ([]admin.PaymentLog, error) {
	var rows []admin.PaymentLog
	q := url.Values{"select": {"id,gateway_slug,nivel,http_status,mensagem,created_at"}, "order": {"created_at.desc"}, "limit": {fmt.Sprint(limit)}}
	if err := c.Select(ctx, "pagamentos_log", q, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (c *Client) WebhookLogs(ctx context.Context, limit int) ([]admin.WebhookLog, error) {
	var rows []admin.WebhookLog
	q := url.Values{"select": {"id,gateway_slug,evento,transacao_gateway_id,assinatura_valida,resumo,created_at"}, "order": {"created_at.desc"}, "limit": {fmt.Sprint(limit)}}
	if err := c.Select(ctx, "webhooks_log", q, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (c *Client) TransactionsByGatewayIDs(ctx context.Context, ids []string) ([]admin.TransactionLookup, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []admin.TransactionLookup
	q := url.Values{"select": {"transacao_gateway_id,valor_centavos,status,clientes(nome,telefone)"}, "transacao_gateway_id": {"in.(" + strings.Join(ids, ",") + ")"}}
	if err := c.Select(ctx, "transacoes_pix", q, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}
