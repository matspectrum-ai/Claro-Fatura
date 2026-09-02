package admin

import (
	"context"
	"strings"
	"time"
)

type TransactionRow struct {
	ID                   string  `json:"id"`
	InvoiceID            string  `json:"fatura_id"`
	ClientID             *string `json:"cliente_id"`
	GatewaySlug          string  `json:"gateway_slug"`
	GatewayTransactionID *string `json:"transacao_gateway_id"`
	AmountCents          int     `json:"valor_centavos"`
	Status               string  `json:"status"`
	CreatedAt            string  `json:"created_at"`
	ExpiresAt            *string `json:"expira_em"`
	PaidAt               *string `json:"pago_em"`
	Client               *struct {
		Name string `json:"nome"`
	} `json:"clientes"`
}

type TransactionAdmin struct {
	ID                   string  `json:"id"`
	InvoiceID            string  `json:"fatura_id"`
	GatewaySlug          string  `json:"gateway_slug"`
	GatewayTransactionID *string `json:"transacao_gateway_id"`
	AmountCents          int     `json:"valor_centavos"`
	Status               string  `json:"status"`
	CreatedAt            string  `json:"created_at"`
	ExpiresAt            *string `json:"expira_em"`
	PaidAt               *string `json:"pago_em"`
	ClientName           *string `json:"cliente_nome"`
	Attempts             int     `json:"tentativas"`
}

type PaymentRow struct {
	ID                   string  `json:"id"`
	GatewaySlug          string  `json:"gateway_slug"`
	GatewayTransactionID *string `json:"transacao_gateway_id"`
	AmountCents          int     `json:"valor_centavos"`
	PaidAmountCents      *int    `json:"valor_pago_centavos"`
	PaidAt               string  `json:"pago_em"`
	Client               *struct {
		Name  string `json:"nome"`
		Phone string `json:"telefone"`
	} `json:"clientes"`
	Invoice *struct {
		Description string  `json:"descricao"`
		Status      string  `json:"status"`
		Discount    float64 `json:"valor_desconto"`
	} `json:"faturas"`
}

type PaymentReceived struct {
	ID                 string  `json:"id"`
	ClientName         *string `json:"cliente_nome"`
	ClientPhone        *string `json:"cliente_telefone"`
	GatewaySlug        string  `json:"gateway_slug"`
	AmountCents        int     `json:"valor_centavos"`
	InvoiceAmountCents *int    `json:"valor_fatura_centavos"`
	PaidAt             string  `json:"pago_em"`
	ConfirmedBy        string  `json:"confirmado_por"`
	InvoiceStatus      *string `json:"status_fatura"`
	Description        *string `json:"descricao"`
}

type PaymentLog struct {
	ID         string `json:"id"`
	Gateway    string `json:"gateway_slug"`
	Level      string `json:"nivel"`
	HTTPStatus *int   `json:"http_status"`
	Message    string `json:"mensagem"`
	CreatedAt  string `json:"created_at"`
}

type WebhookLog struct {
	ID                   string  `json:"id"`
	Gateway              string  `json:"gateway_slug"`
	Event                *string `json:"evento"`
	GatewayTransactionID *string `json:"transacao_gateway_id"`
	SignatureValid       bool    `json:"assinatura_valida"`
	Summary              *string `json:"resumo"`
	CreatedAt            string  `json:"created_at"`
	ClientName           *string `json:"cliente_nome"`
	ClientPhone          *string `json:"cliente_telefone"`
	AmountCents          *int    `json:"valor_centavos"`
	TransactionStatus    *string `json:"status_transacao"`
	Recognized           bool    `json:"reconhecido"`
}

type TransactionLookup struct {
	GatewayTransactionID string `json:"transacao_gateway_id"`
	AmountCents          int    `json:"valor_centavos"`
	Status               string `json:"status"`
	Client               *struct {
		Name  string `json:"nome"`
		Phone string `json:"telefone"`
	} `json:"clientes"`
}

type Logs struct {
	Payments []PaymentLog `json:"pagamentos"`
	Webhooks []WebhookLog `json:"webhooks"`
}

type ActivityStore interface {
	TransactionRows(context.Context, string, int) ([]TransactionRow, error)
	PaymentRows(context.Context, string, time.Time, int) ([]PaymentRow, error)
	WebhookTransactionIDs(context.Context, []string) (map[string]struct{}, error)
	PaymentLogs(context.Context, int) ([]PaymentLog, error)
	WebhookLogs(context.Context, int) ([]WebhookLog, error)
	TransactionsByGatewayIDs(context.Context, []string) ([]TransactionLookup, error)
}

type ActivityService struct {
	store ActivityStore
	now   func() time.Time
}

func NewActivityService(store ActivityStore) *ActivityService {
	return &ActivityService{store: store, now: time.Now}
}

func (s *ActivityService) Transactions(ctx context.Context, status string, grouped bool, limit int) ([]TransactionAdmin, error) {
	status = strings.TrimSpace(status)
	if status == "" {
		status = "todos"
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	queryStatus := ""
	fetchLimit := limit
	if grouped {
		fetchLimit = 500
	} else if status != "todos" {
		queryStatus = status
	}
	rows, err := s.store.TransactionRows(ctx, queryStatus, fetchLimit)
	if err != nil {
		return nil, err
	}
	mapRow := func(r TransactionRow) TransactionAdmin {
		var name *string
		if r.Client != nil {
			v := r.Client.Name
			name = &v
		}
		return TransactionAdmin{ID: r.ID, InvoiceID: r.InvoiceID, GatewaySlug: r.GatewaySlug, GatewayTransactionID: r.GatewayTransactionID, AmountCents: r.AmountCents, Status: r.Status, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, PaidAt: r.PaidAt, ClientName: name, Attempts: 1}
	}
	if !grouped {
		out := make([]TransactionAdmin, 0, len(rows))
		for _, r := range rows {
			out = append(out, mapRow(r))
		}
		return out, nil
	}
	order := make([]string, 0, len(rows))
	byKey := make(map[string]*TransactionAdmin, len(rows))
	for _, r := range rows {
		key := r.InvoiceID
		if r.ClientID != nil && *r.ClientID != "" {
			key = *r.ClientID
		}
		if existing := byKey[key]; existing != nil {
			existing.Attempts++
			continue
		}
		mapped := mapRow(r)
		byKey[key] = &mapped
		order = append(order, key)
	}
	out := make([]TransactionAdmin, 0, min(limit, len(order)))
	for _, key := range order {
		item := byKey[key]
		if status != "todos" && item.Status != status {
			continue
		}
		out = append(out, *item)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *ActivityService) Payments(ctx context.Context, gateway string, days int) ([]PaymentReceived, error) {
	if days <= 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}
	gateway = strings.TrimSpace(gateway)
	if gateway == "todas" {
		gateway = ""
	}
	rows, err := s.store.PaymentRows(ctx, gateway, s.now().Add(-time.Duration(days)*24*time.Hour), 300)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	seen := map[string]bool{}
	for _, r := range rows {
		if r.GatewayTransactionID != nil && *r.GatewayTransactionID != "" && !seen[*r.GatewayTransactionID] {
			seen[*r.GatewayTransactionID] = true
			ids = append(ids, *r.GatewayTransactionID)
		}
	}
	viaWebhook, err := s.store.WebhookTransactionIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]PaymentReceived, 0, len(rows))
	for _, r := range rows {
		amount := r.AmountCents
		if r.PaidAmountCents != nil {
			amount = *r.PaidAmountCents
		}
		item := PaymentReceived{ID: r.ID, GatewaySlug: r.GatewaySlug, AmountCents: amount, PaidAt: r.PaidAt, ConfirmedBy: "consulta"}
		if r.GatewayTransactionID != nil {
			if _, ok := viaWebhook[*r.GatewayTransactionID]; ok {
				item.ConfirmedBy = "webhook"
			}
		}
		if r.Client != nil {
			n, p := r.Client.Name, r.Client.Phone
			item.ClientName = &n
			item.ClientPhone = &p
		}
		if r.Invoice != nil {
			d, st := r.Invoice.Description, r.Invoice.Status
			cents := int(r.Invoice.Discount*100 + 0.5)
			item.Description = &d
			item.InvoiceStatus = &st
			item.InvoiceAmountCents = &cents
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *ActivityService) Logs(ctx context.Context) (Logs, error) {
	payments, err := s.store.PaymentLogs(ctx, 100)
	if err != nil {
		return Logs{}, err
	}
	webhooks, err := s.store.WebhookLogs(ctx, 100)
	if err != nil {
		return Logs{}, err
	}
	ids := make([]string, 0, len(webhooks))
	seen := map[string]bool{}
	for _, w := range webhooks {
		if w.GatewayTransactionID != nil && *w.GatewayTransactionID != "" && !seen[*w.GatewayTransactionID] {
			seen[*w.GatewayTransactionID] = true
			ids = append(ids, *w.GatewayTransactionID)
		}
	}
	details, err := s.store.TransactionsByGatewayIDs(ctx, ids)
	if err != nil {
		return Logs{}, err
	}
	byID := map[string]TransactionLookup{}
	for _, d := range details {
		if d.GatewayTransactionID != "" {
			byID[d.GatewayTransactionID] = d
		}
	}
	for i := range webhooks {
		w := &webhooks[i]
		if w.GatewayTransactionID == nil {
			continue
		}
		d, ok := byID[*w.GatewayTransactionID]
		if !ok {
			continue
		}
		w.Recognized = true
		amount, status := d.AmountCents, d.Status
		w.AmountCents = &amount
		w.TransactionStatus = &status
		if d.Client != nil {
			n, p := d.Client.Name, d.Client.Phone
			w.ClientName = &n
			w.ClientPhone = &p
		}
	}
	return Logs{Payments: payments, Webhooks: webhooks}, nil
}
