package cashinpay

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
)

func TestCreatePIXMatchesCurrentCashinPayPayloadShape(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("authorization=%q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":"tx-123","status":"pending","pix":{"copy_paste":"000201PIXCODE","qrcode":"data:image/png;base64,abc"}}}`))
	}))
	defer server.Close()
	client := New("secret", "", "Ebook Viver de Vendas", "cliente@ebookviver.app")
	client.baseURL = server.URL
	client.http = server.Client()
	email := "real@example.com"
	doc := "52998224725"
	created, err := client.CreatePIX(context.Background(), gateway.CreateInput{AmountCents: 12345, Name: "Maria", Phone: "(93) 99123-4567", Email: &email, Document: &doc, Reference: "req-1", WebhookURL: "https://site/webhook"})
	if err != nil {
		t.Fatal(err)
	}
	if created.TransactionID != "tx-123" || created.CopyPaste != "000201PIXCODE" {
		t.Fatalf("created=%+v", created)
	}
	if got["amount"].(float64) != 123.45 {
		t.Fatalf("amount=%v", got["amount"])
	}
	if got["transaction_id"] != "req-1" || got["description"] != "Ebook Viver de Vendas" || got["postbackUrl"] != "https://site/webhook" {
		t.Fatalf("payload=%v", got)
	}
	customer := got["customer"].(map[string]any)
	if customer["name"] != "Maria" || customer["email"] != "cliente@ebookviver.app" || customer["phone"] != "93991234567" {
		t.Fatalf("customer=%v", customer)
	}
}

func TestPaidStatusCompatibility(t *testing.T) {
	client := New("x", "", "", "")
	for _, value := range []string{"paid", "APPROVED", "completed", "confirmed", "pago", "aprovado"} {
		v := value
		if !client.Paid(&v) {
			t.Fatalf("expected %q paid", value)
		}
	}
	v := "pending"
	if client.Paid(&v) {
		t.Fatal("pending must not be paid")
	}
}
