package generic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
)

func TestGenericPayloadHeadersAndRecursiveExtraction(t *testing.T) {
	var got map[string]any
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" || r.Header.Get("x-secret-key") != "secret" { t.Fatal("headers") }
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"transaction_id": "g-1", "pix": map[string]any{"payload": "000201GENERIC"}, "status": "pending"}})
	}))
	defer s.Close()
	c := New()
	c.http = s.Client()
	c.lookup = func(k string) string { if k=="TOKEN" { return "token" }; if k=="SECRET" { return "secret" }; return "" }
	u := s.URL
	record := gateway.Record{APIURL: &u, SecretNames: []string{"TOKEN", "SECRET"}}
	created, err := c.CreatePIX(context.Background(), gateway.CreateInput{Gateway: record, AmountCents: 999, Name: "Maria", Phone: "9399", Description: "Fatura", Reference: "ref", WebhookURL: "https://cb"})
	if err != nil { t.Fatal(err) }
	if created.TransactionID != "g-1" || created.CopyPaste != "000201GENERIC" { t.Fatalf("created=%+v", created) }
	if got["amount_cents"] != float64(999) || got["payment_method"] != "pix" { t.Fatalf("payload=%v", got) }
}
