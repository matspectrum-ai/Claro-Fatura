package propix

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
)

func TestCreatePIXPayloadAndResponseCompatibility(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/deposit" { t.Fatalf("path=%s", r.URL.Path) }
		if r.Header.Get("x-client-id") != "id" || r.Header.Get("x-client-secret") != "secret" { t.Fatal("missing credentials") }
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil { t.Fatal(err) }
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "transactionId": "p-1", "copyPaste": "000201PIX", "qrcodeUrl": "base64:abc", "status": "PENDENTE"})
	}))
	defer server.Close()
	c := New("id", "secret", "", "")
	c.baseURL = server.URL
	c.http = server.Client()
	doc := "529.982.247-25"
	created, err := c.CreatePIX(context.Background(), gateway.CreateInput{AmountCents: 12345, Name: "Maria", Document: &doc})
	if err != nil { t.Fatal(err) }
	if got["amount"] != float64(123.45) || got["description"] != gateway.DefaultProductName || got["payerEmail"] != gateway.DefaultCustomerEmail || got["payerDocument"] != "52998224725" { t.Fatalf("payload=%v", got) }
	if created.TransactionID != "p-1" || created.CopyPaste != "000201PIX" || created.QRCode == nil || *created.QRCode != "abc" { t.Fatalf("created=%+v", created) }
}

func TestWebhookAndPaidStatuses(t *testing.T) {
	c := New("id", "secret", "", "")
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	raw, err := json.Marshal(map[string]any{"transaction": map[string]any{"transactionId": "x", "transactionState": "COMPLETO"}})
	if err != nil { t.Fatal(err) }
	read, err := c.ReadWebhook(req, raw, gateway.Record{})
	if err != nil || !read.Valid || read.TransactionID == nil || *read.TransactionID != "x" || !c.Paid(read.Status) { t.Fatalf("read=%+v err=%v", read, err) }
}
