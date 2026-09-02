package propix

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
)

const defaultBaseURL = "https://api.propixbr.com/api/v1"

type Client struct {
	clientID      string
	clientSecret  string
	baseURL       string
	http          *http.Client
	productName   string
	customerEmail string
}

func New(clientID, clientSecret, productName, customerEmail string) *Client {
	if strings.TrimSpace(productName) == "" {
		productName = gateway.DefaultProductName
	}
	if strings.TrimSpace(customerEmail) == "" {
		customerEmail = gateway.DefaultCustomerEmail
	}
	return &Client{
		clientID: clientID, clientSecret: clientSecret, baseURL: defaultBaseURL,
		http:        &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{MaxIdleConns: 128, MaxIdleConnsPerHost: 64, IdleConnTimeout: 90 * time.Second}},
		productName: productName, customerEmail: customerEmail,
	}
}

func (c *Client) Name() string { return "propix" }
func (c *Client) Configured(gateway.Record) bool {
	return strings.TrimSpace(c.clientID) != "" && strings.TrimSpace(c.clientSecret) != ""
}

func (c *Client) CreatePIX(ctx context.Context, in gateway.CreateInput) (gateway.CreatedPIX, error) {
	if !c.Configured(in.Gateway) {
		return gateway.CreatedPIX{}, errors.New("credenciais ProPix (CLIENT_ID/SECRET) não configuradas")
	}
	document := onlyDigits(deref(in.Document))
	body := map[string]any{
		"amount":        float64(in.AmountCents) / 100,
		"description":   truncate(c.productName, 50),
		"payerName":     truncate(customerName(in.Name), 50),
		"payerEmail":    c.customerEmail,
		"payerDocument": fallback(document, "00000000000"),
	}
	var response struct {
		Success       bool   `json:"success"`
		TransactionID any    `json:"transactionId"`
		CopyPaste     string `json:"copyPaste"`
		QRCodeURL     string `json:"qrcodeUrl"`
		Status        string `json:"status"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/deposit", body, &response); err != nil {
		return gateway.CreatedPIX{}, err
	}
	if !response.Success {
		return gateway.CreatedPIX{}, errors.New("ProPix não retornou a cobrança")
	}
	qrcode := strings.TrimPrefix(response.QRCodeURL, "base64:")
	var qr *string
	if qrcode != "" {
		qr = &qrcode
	}
	status := response.Status
	if status == "" {
		status = "PENDENTE"
	}
	return gateway.CreatedPIX{TransactionID: fmt.Sprint(response.TransactionID), CopyPaste: response.CopyPaste, QRCode: qr, Status: status}, nil
}

func (c *Client) Status(ctx context.Context, id string, _ gateway.Record) (*string, error) {
	var response struct {
		Transaction *struct {
			State string `json:"transactionState"`
		} `json:"transaction"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/check", map[string]any{"transactionId": id}, &response); err != nil {
		return nil, nil
	}
	if response.Transaction == nil || response.Transaction.State == "" {
		return nil, nil
	}
	return &response.Transaction.State, nil
}

func (c *Client) Paid(status *string) bool {
	if status == nil {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(*status)) {
	case "COMPLETO", "APROVADO", "PAID", "SUCCESS":
		return true
	default:
		return false
	}
}

func (c *Client) ReadWebhook(_ *http.Request, raw []byte, _ gateway.Record) (gateway.WebhookRead, error) {
	var body map[string]any
	if json.Unmarshal(raw, &body) != nil {
		return gateway.WebhookRead{Valid: false}, nil
	}
	id := scalar(body["transactionId"])
	status := scalar(body["status"])
	if tx, ok := body["transaction"].(map[string]any); ok {
		if id == "" {
			id = scalar(tx["transactionId"])
		}
		if status == "" {
			status = scalar(tx["transactionState"])
		}
	}
	event := scalar(body["event"])
	if event == "" && strings.EqualFold(status, "COMPLETO") {
		event = "DEPOSITO_COMPLETO"
	}
	return gateway.WebhookRead{Valid: true, TransactionID: ptr(id), Status: ptr(status), Event: ptr(event)}, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, dst any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-client-id", c.clientID)
	req.Header.Set("x-client-secret", c.clientSecret)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ProPix HTTP %d", resp.StatusCode)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return errors.New("resposta inválida da ProPix")
	}
	return nil
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
func fallback(v, f string) string {
	if v == "" {
		return f
	}
	return v
}
func customerName(v string) string {
	if strings.TrimSpace(v) == "" {
		return "Cliente"
	}
	return strings.TrimSpace(v)
}
func truncate(v string, n int) string {
	r := []rune(v)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}
func scalar(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return fmt.Sprint(x)
	default:
		return fmt.Sprint(x)
	}
}
func ptr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
