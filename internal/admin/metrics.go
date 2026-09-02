package admin

import (
	"context"
	"time"
)

type AccessRecord struct {
	ID       string    `json:"id"`
	At       time.Time `json:"-"`
	AtRaw    string    `json:"data_hora"`
	Page     string    `json:"pagina"`
	Phone    *string   `json:"telefone_consultado"`
	Success  bool      `json:"sucesso"`
	Original *float64  `json:"valor_original"`
	Discount *float64  `json:"valor_desconto"`
}
type RecentAccess struct {
	ID       string   `json:"id"`
	DateTime string   `json:"data_hora"`
	Page     string   `json:"pagina"`
	Phone    *string  `json:"telefone_consultado"`
	Success  bool     `json:"sucesso"`
	Discount *float64 `json:"valor_desconto"`
}
type Metrics struct {
	ClientsTotal    int     `json:"clientes_total"`
	ClientsToday    int     `json:"clientes_hoje"`
	ClientsMonth    int     `json:"clientes_mes"`
	DiscountTotal   float64 `json:"valor_desconto_total"`
	DiscountToday   float64 `json:"valor_desconto_hoje"`
	DiscountMonth   float64 `json:"valor_desconto_mes"`
	OpenTotal       float64 `json:"valor_aberto_total"`
	OpenToday       float64 `json:"valor_aberto_hoje"`
	OpenMonth       float64 `json:"valor_aberto_mes"`
	AccessToday     int     `json:"acessos_hoje"`
	AccessMonth     int     `json:"acessos_mes"`
	AccessTotal     int     `json:"acessos_total"`
	QueriesTotal    int     `json:"consultas_total"`
	QueriesToday    int     `json:"consultas_hoje"`
	InvoicesViewed  int     `json:"faturas_visualizadas_total"`
	ValueViewed     float64 `json:"valor_visualizado_total"`
	Recent          []RecentAccess `json:"recentes"`
	DatabaseClients int     `json:"clientes_banco"`
}
type MetricsStore interface {
	AccessRecords(context.Context, int) ([]AccessRecord, error)
	CountClients(context.Context) (int, error)
	ClearAccesses(context.Context) (int, error)
}
type MetricsService struct {
	store MetricsStore
	now   func() time.Time
	loc   *time.Location
}
func NewMetricsService(store MetricsStore) *MetricsService {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil { loc = time.FixedZone("BRT", -3*3600) }
	return &MetricsService{store: store, now: time.Now, loc: loc}
}
func (s *MetricsService) Get(ctx context.Context) (Metrics, error) {
	records, err := s.store.AccessRecords(ctx, 50000)
	if err != nil { return Metrics{}, err }
	clients, err := s.store.CountClients(ctx)
	if err != nil { return Metrics{}, err }
	m := Metrics{AccessTotal: len(records), Recent: []RecentAccess{}, DatabaseClients: clients}
	seenRecent := map[string]bool{}
	for _, r := range records {
		if len(m.Recent) >= 20 { break }
		if r.Phone != nil {
			if seenRecent[*r.Phone] { continue }
			seenRecent[*r.Phone] = true
		}
		m.Recent = append(m.Recent, RecentAccess{ID: r.ID, DateTime: r.AtRaw, Page: r.Page, Phone: r.Phone, Success: r.Success, Discount: r.Discount})
	}
	now := s.now().In(s.loc)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.loc)
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, s.loc)
	totalSeen, daySeen, monthSeen := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for i := len(records)-1; i >= 0; i-- {
		r := records[i]
		at := r.At
		if at.IsZero() { at, _ = time.Parse(time.RFC3339Nano, r.AtRaw) }
		inDay, inMonth := !at.Before(day), !at.Before(month)
		if inDay { m.AccessToday++ }
		if inMonth { m.AccessMonth++ }
		if r.Phone != nil { m.QueriesTotal++; if inDay { m.QueriesToday++ } }
		if !r.Success || r.Phone == nil { continue }
		p, d, o := *r.Phone, metricNumber(r.Discount), metricNumber(r.Original)
		if !totalSeen[p] { totalSeen[p]=true; m.ClientsTotal++; m.InvoicesViewed++; m.ValueViewed+=d; m.DiscountTotal+=d; m.OpenTotal+=o }
		if inMonth && !monthSeen[p] { monthSeen[p]=true; m.ClientsMonth++; m.DiscountMonth+=d; m.OpenMonth+=o }
		if inDay && !daySeen[p] { daySeen[p]=true; m.ClientsToday++; m.DiscountToday+=d; m.OpenToday+=o }
	}
	return m, nil
}
func (s *MetricsService) Clear(ctx context.Context) (int, error) { return s.store.ClearAccesses(ctx) }
func metricNumber(v *float64) float64 { if v == nil { return 0 }; return *v }
