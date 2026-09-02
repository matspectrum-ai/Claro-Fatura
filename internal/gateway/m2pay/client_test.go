package m2pay

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
)

func TestCreatePIXUsesCentsAndRequiredM2PayShape(t *testing.T) {
	var got map[string]any
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "key" { t.Fatal("api key") }
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"transactionId": "m-1", "status": "PENDING", "pix": map[string]any{"emv": "000201M2", "qrcode": "img", "expiresAt": "2030-01-01T00:00:00Z"}}})
	}))
	defer s.Close()
	c := New("key", "", "")
	c.baseURL = s.URL
	c.http = s.Client()
	doc := "52998224725"
	created, err := c.CreatePIX(context.Background(), gateway.CreateInput{AmountCents: 12345, Name: "Maria", Phone: "+55 (93) 99123-4567", Document: &doc, Reference: "ref", WebhookURL: "https://cb"})
	if err != nil { t.Fatal(err) }
	if got["amount"] != float64(12345) || got["paymentMethod"] != "pix" { t.Fatalf("payload=%v", got) }
	items := got["items"].([]any)
	item := items[0].(map[string]any)
	if item["title"] != gateway.DefaultProductName || item["unitPrice"] != float64(12345) { t.Fatalf("item=%v", item) }
	customer := got["customer"].(map[string]any)
	if customer["email"] != gateway.DefaultCustomerEmail || customer["phone"] != "93991234567" { t.Fatalf("customer=%v", customer) }
	if created.CopyPaste != "000201M2" || created.ExpiresAt == nil { t.Fatalf("created=%+v", created) }
}

func TestPaidOnlyPAID(t *testing.T) {
	c := New("key", "", "")
	v := "PAID"
	if !c.Paid(&v) { t.Fatal("paid") }
	v = "COMPLETED"
	if c.Paid(&v) { t.Fatal("completed must not mean paid for M2 Pay") }
}
