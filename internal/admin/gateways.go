package admin

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
)

var ErrInvalidGateway = errors.New("gateway inválida")
var ErrInvalidRouting = errors.New("roteamento inválido")

var slugPattern = regexp.MustCompile(`^[a-z0-9-]+$`)
var secretPattern = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

type GatewayConfig struct {
	gateway.Record
	Configured bool `json:"configurado"`
}

type RoutingConfig struct {
	Strategy        string  `json:"estrategia"`
	FixedGateway    *string `json:"gateway_fixa"`
	NewPIXPerAccess bool    `json:"novo_pix_por_acesso"`
}

type RoutingInput struct {
	Strategy        string  `json:"estrategia"`
	FixedGateway    *string `json:"gateway_fixa,omitempty"`
	NewPIXPerAccess *bool   `json:"novo_pix_por_acesso,omitempty"`
}

type GatewayInput struct {
	ID           string   `json:"id,omitempty"`
	Slug         string   `json:"slug"`
	Label        string   `json:"rotulo"`
	Adapter      string   `json:"adapter"`
	APIURL       *string  `json:"api_url"`
	Environment  string   `json:"ambiente"`
	Priority     int      `json:"prioridade"`
	DailyLimit   *int     `json:"limite_diario"`
	WebhookURL   *string  `json:"webhook_url"`
	SecretNames  []string `json:"secret_names"`
	Observations *string  `json:"observacoes"`
	Active       bool     `json:"ativo"`
}

type GatewayPatch struct {
	ID       string `json:"id"`
	Active   *bool  `json:"ativo,omitempty"`
	Priority *int   `json:"prioridade,omitempty"`
}

type WebhookGatewayRow struct {
	Gateway string `json:"gateway_slug"`
	At      string `json:"created_at"`
}

type GatewayWebhookSummary struct {
	Gateway string  `json:"gateway_slug"`
	LastAt  *string `json:"ultimo_em"`
	Last24h int     `json:"total_24h"`
}

type GatewayAdminStore interface {
	ListGatewayRecords(context.Context) ([]gateway.Record, error)
	ReadGatewayRouting(context.Context) (RoutingConfig, bool, error)
	SaveGatewayRouting(context.Context, RoutingInput) error
	InsertGatewayRecord(context.Context, gateway.Record, time.Time) error
	UpdateGatewayRecord(context.Context, string, gateway.Record, time.Time) error
	PatchGatewayRecord(context.Context, string, *bool, *int, time.Time) error
	DeleteGatewayRecord(context.Context, string) error
	UseOnlyGateway(context.Context, string) error
	ActivateAllGateways(context.Context) error
	WebhookGatewayRows(context.Context, int) ([]WebhookGatewayRow, error)
}

type GatewayAdminService struct {
	store  GatewayAdminStore
	lookup func(string) string
	now    func() time.Time
}

func NewGatewayAdminService(store GatewayAdminStore, lookup func(string) string) *GatewayAdminService {
	if lookup == nil {
		lookup = func(string) string { return "" }
	}
	return &GatewayAdminService{store: store, lookup: lookup, now: time.Now}
}

func (s *GatewayAdminService) List(ctx context.Context) ([]GatewayConfig, error) {
	rows, err := s.store.ListGatewayRecords(ctx)
	if err != nil { return nil, err }
	out := make([]GatewayConfig, 0, len(rows))
	for _, row := range rows {
		out = append(out, GatewayConfig{Record: row, Configured: gatewayConfigured(row, s.lookup)})
	}
	return out, nil
}

func gatewayConfigured(g gateway.Record, lookup func(string) string) bool {
	key := g.Adapter
	if key == "" { key = g.Slug }
	switch key {
	case "cashinpay":
		return lookup("CASHINPAY_SECRET_KEY") != ""
	case "propix":
		return lookup("PROPIX_CLIENT_ID") != "" && lookup("PROPIX_CLIENT_SECRET") != ""
	case "m2pay":
		return lookup("M2PAY_API_KEY") != ""
	case "nowbanks":
		return lookup("NOWBANKS_CLIENT_ID") != "" && lookup("NOWBANKS_CLIENT_SECRET") != ""
	case "pix-estatico":
		return lookup("PIX_CHAVE") != ""
	default:
		if len(g.SecretNames) == 0 { return false }
		for _, name := range g.SecretNames { if lookup(name) == "" { return false } }
		return true
	}
}

func (s *GatewayAdminService) Routing(ctx context.Context) (RoutingConfig, error) {
	cfg, ok, err := s.store.ReadGatewayRouting(ctx)
	if err != nil { return RoutingConfig{}, err }
	if !ok { return RoutingConfig{Strategy: "prioridade", NewPIXPerAccess: true}, nil }
	if cfg.Strategy == "" { cfg.Strategy = "prioridade" }
	return cfg, nil
}

func (s *GatewayAdminService) SaveRouting(ctx context.Context, in RoutingInput) error {
	if !validStrategy(in.Strategy) { return ErrInvalidRouting }
	if in.FixedGateway != nil && *in.FixedGateway != "" && !validUUID(*in.FixedGateway) { return ErrInvalidRouting }
	if in.Strategy != "fixa" { in.FixedGateway = nil }
	return s.store.SaveGatewayRouting(ctx, in)
}

func (s *GatewayAdminService) Save(ctx context.Context, in GatewayInput) error {
	normalizeGatewayInput(&in)
	if !validGatewayInput(in) { return ErrInvalidGateway }
	record := gateway.Record{ID: in.ID, Slug: in.Slug, Label: in.Label, Adapter: in.Adapter, Active: in.Active, Priority: in.Priority, APIURL: in.APIURL, Environment: in.Environment, DailyLimit: in.DailyLimit, WebhookURL: in.WebhookURL, SecretNames: in.SecretNames, Observations: in.Observations}
	if in.ID == "" { return s.store.InsertGatewayRecord(ctx, record, s.now()) }
	return s.store.UpdateGatewayRecord(ctx, in.ID, record, s.now())
}

func (s *GatewayAdminService) Patch(ctx context.Context, in GatewayPatch) error {
	if !validUUID(in.ID) || (in.Active == nil && in.Priority == nil) { return ErrInvalidGateway }
	if in.Priority != nil && (*in.Priority < 1 || *in.Priority > 999) { return ErrInvalidGateway }
	return s.store.PatchGatewayRecord(ctx, in.ID, in.Active, in.Priority, s.now())
}
func (s *GatewayAdminService) Remove(ctx context.Context, id string) error { if !validUUID(id) { return ErrInvalidGateway }; return s.store.DeleteGatewayRecord(ctx, id) }
func (s *GatewayAdminService) UseOnly(ctx context.Context, id string) error { if !validUUID(id) { return ErrInvalidGateway }; return s.store.UseOnlyGateway(ctx, id) }
func (s *GatewayAdminService) ActivateAll(ctx context.Context) error { return s.store.ActivateAllGateways(ctx) }

func (s *GatewayAdminService) WebhookSummary(ctx context.Context) ([]GatewayWebhookSummary, error) {
	rows, err := s.store.WebhookGatewayRows(ctx, 1000)
	if err != nil { return nil, err }
	limit := s.now().Add(-24 * time.Hour)
	order := make([]string, 0)
	byGateway := map[string]*GatewayWebhookSummary{}
	for _, row := range rows {
		current := byGateway[row.Gateway]
		if current == nil { current = &GatewayWebhookSummary{Gateway: row.Gateway}; byGateway[row.Gateway] = current; order = append(order, row.Gateway) }
		if current.LastAt == nil { v := row.At; current.LastAt = &v }
		at, parseErr := time.Parse(time.RFC3339Nano, row.At)
		if parseErr == nil && !at.Before(limit) { current.Last24h++ }
	}
	out := make([]GatewayWebhookSummary, 0, len(order))
	for _, key := range order { out = append(out, *byGateway[key]) }
	return out, nil
}

func normalizeGatewayInput(in *GatewayInput) {
	in.Slug = strings.ToLower(strings.TrimSpace(in.Slug)); in.Label = strings.TrimSpace(in.Label); in.Adapter = strings.TrimSpace(in.Adapter)
	in.APIURL = cleanPtr(in.APIURL); in.WebhookURL = cleanPtr(in.WebhookURL); in.Observations = cleanPtr(in.Observations)
	clean := make([]string, 0, len(in.SecretNames)); for _, name := range in.SecretNames { name = strings.TrimSpace(name); if name != "" { clean = append(clean, name) } }; in.SecretNames = clean
}
func validGatewayInput(in GatewayInput) bool {
	if in.ID != "" && !validUUID(in.ID) { return false }; if len(in.Slug)<2||len(in.Slug)>40||!slugPattern.MatchString(in.Slug){return false}; if len(in.Label)<2||len(in.Label)>80{return false}; if len(in.Adapter)<2||len(in.Adapter)>40{return false}; if in.Environment!="producao"&&in.Environment!="teste"{return false}; if in.Priority<1||in.Priority>999{return false}; if in.DailyLimit!=nil&&(*in.DailyLimit<0||*in.DailyLimit>1_000_000){return false}; if len(in.SecretNames)>6{return false}; for _,name:=range in.SecretNames{if !secretPattern.MatchString(name){return false}}; if in.Observations!=nil&&len(*in.Observations)>500{return false}; if in.APIURL!=nil&&(len(*in.APIURL)>300||!validURL(*in.APIURL)){return false}; if in.WebhookURL!=nil&&(len(*in.WebhookURL)>300||!validURL(*in.WebhookURL)){return false}; return true
}
func validStrategy(v string) bool { return v=="prioridade"||v=="rodizio"||v=="fixa" }
func validURL(v string) bool { u,err:=url.ParseRequestURI(v); return err==nil&&u.Scheme!=""&&u.Host!="" }
func validUUID(v string) bool { if len(v)!=36||v[8]!='-'||v[13]!='-'||v[18]!='-'||v[23]!='-'{return false};for i,c:=range v{if i==8||i==13||i==18||i==23{continue};if !((c>='0'&&c<='9')||(c>='a'&&c<='f')||(c>='A'&&c<='F')){return false}};return true }
func cleanPtr(v *string)*string{if v==nil{return nil};x:=strings.TrimSpace(*v);if x==""{return nil};return &x}
