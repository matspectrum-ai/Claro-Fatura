package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type roles struct {
	admin bool
	id    string
}

func (r *roles) IsAdmin(_ context.Context, id string) (bool, error) { r.id = id; return r.admin, nil }

func TestSignInRequiresAdminRole(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/v1/token" || r.URL.Query().Get("grant_type") != "password" {
			t.Fatalf("url=%s", r.URL.String())
		}
		if r.Header.Get("apikey") != "pub" {
			t.Fatalf("apikey=%q", r.Header.Get("apikey"))
		}
		_ = json.NewEncoder(w).Encode(Session{AccessToken: "a", RefreshToken: "r", ExpiresIn: 3600, User: User{ID: "u1", Email: "a@b.com"}})
	}))
	defer server.Close()
	store := &roles{admin: true}
	s := New(server.URL, "pub", store)
	s.http = server.Client()
	got, err := s.SignIn(context.Background(), "a@b.com", "pw")
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != "a" || store.id != "u1" {
		t.Fatalf("got=%+v id=%s", got, store.id)
	}
	store.admin = false
	if _, err := s.SignIn(context.Background(), "a@b.com", "pw"); err != ErrNotAdmin {
		t.Fatalf("err=%v", err)
	}
}

func TestRefreshAndUserBearer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/v1/token":
			_ = json.NewEncoder(w).Encode(Session{AccessToken: "new", RefreshToken: "new-r", User: User{ID: "u1"}})
		case "/auth/v1/user":
			if r.Header.Get("Authorization") != "Bearer new" {
				t.Fatalf("auth=%q", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(User{ID: "u1"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	s := New(server.URL, "pub", &roles{admin: true})
	s.http = server.Client()
	sess, err := s.Refresh(context.Background(), "r")
	if err != nil {
		t.Fatal(err)
	}
	user, err := s.User(context.Background(), sess.AccessToken)
	if err != nil || user.ID != "u1" {
		t.Fatalf("user=%+v err=%v", user, err)
	}
}

func TestRecoveryKeepsRedirect(t *testing.T) {
	var redirect string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirect = r.URL.Query().Get("redirect_to")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	s := New(server.URL, "pub", &roles{})
	s.http = server.Client()
	if err := s.SendRecovery(context.Background(), "a@b.com", "https://site/reset-password"); err != nil {
		t.Fatal(err)
	}
	if redirect != "https://site/reset-password" {
		t.Fatalf("redirect=%q", redirect)
	}
}
