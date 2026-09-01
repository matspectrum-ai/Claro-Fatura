package nowbanks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
)

func TestCreatePIXAuthenticatesAndCachesToken(t *testing.T) {
	var authCalls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/login":
			authCalls.Add(1)
			_ = jsonWrite(w, map[string]any{"access_token":"token","expires_in":3600})
		case "/payments/deposit":
			if r.Header.Get("Authorization") != "Bearer token" { t.Fatal("auth") }
			_ = jsonWrite(w, map[string]any{"transaction_id":"n-1","pix_copy_paste":"000201NOW","pix_qr_code":"data:image/png;base64,x","status":"PENDING"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer s.Close()
	c := New("id", "secret", "hook")
	c.baseURL = s.URL
	c.http = s.Client()
	for i := 0; i < 2; i++ {
		created, err := c.CreatePIX(context.Background(), gateway.CreateInput{AmountCents: 1050, Name: "Maria", Reference: "ref", WebhookURL: "https://cb"})
		if err != nil { t.Fatal(err) }
		if created.CopyPaste != "000201NOW" { t.Fatalf("created=%+v", created) }
	}
	if authCalls.Load() != 1 { t.Fatalf("auth calls=%d", authCalls.Load()) }
}

func TestWebhookHMACCompatibility(t *testing.T) {
	c := New("id", "secret", "webhook-secret")
	raw := []byte(`{"data":{"transaction_id":"n-1","status":"COMPLETED"},"event":"payment"}`)
	mac := hmac.New(sha256.New, []byte("webhook-secret"))
	_, _ = mac.Write(raw)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Signature", "sha256="+fmt.Sprintf("%x", mac.Sum(nil)))
	read, err := c.ReadWebhook(req, raw, gateway.Record{})
	if err != nil || !read.Valid || !c.Paid(read.Status) { t.Fatalf("read=%+v err=%v", read, err) }
}

func jsonWrite(w http.ResponseWriter, value any) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(value)
}
