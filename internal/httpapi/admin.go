package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/matspectrum-ai/Claro-Fatura/internal/admin"
)

type AdminInvoices interface {
	List(context.Context, string, int) (admin.InvoicePage, error)
	Save(context.Context, string, string, admin.InvoiceEdit) error
	SetStatus(context.Context, string, string) error
	DeleteAll(context.Context, string) (admin.DeleteAllResult, error)
}
type AdminImporter interface {
	Import(context.Context, admin.ImportInput) (admin.ImportResult, error)
}
type AdminMetrics interface {
	Get(context.Context) (admin.Metrics, error)
	Clear(context.Context) (int, error)
}
type AdminActivity interface {
	Transactions(context.Context, string, bool, int) ([]admin.TransactionAdmin, error)
	Payments(context.Context, string, int) ([]admin.PaymentReceived, error)
	Logs(context.Context) (admin.Logs, error)
}

func (s *Server) adminMetrics(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.deps.AdminMetrics == nil {
		writeJSON(w, 503, map[string]string{"erro": "Painel indisponível."})
		return
	}
	m, err := s.deps.AdminMetrics.Get(r.Context())
	if err != nil {
		s.logger.Error("admin metrics failed", "error", err)
		writeJSON(w, 500, map[string]string{"erro": "Não foi possível carregar as métricas."})
		return
	}
	writeJSON(w, 200, m)
}
func (s *Server) adminClearMetrics(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	n, err := s.deps.AdminMetrics.Clear(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"erro": "Não foi possível limpar o histórico."})
		return
	}
	writeJSON(w, 200, map[string]int{"removidos": n})
}
func (s *Server) adminInvoices(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("pagina"))
	out, err := s.deps.AdminInvoices.List(r.Context(), r.URL.Query().Get("busca"), page)
	if err != nil {
		s.logger.Error("admin invoice list failed", "error", err)
		writeJSON(w, 500, map[string]string{"erro": "Não foi possível carregar as faturas."})
		return
	}
	writeJSON(w, 200, out)
}
func (s *Server) adminSaveInvoice(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	var body struct {
		ClientID string `json:"cliente_id"`
		admin.InvoiceEdit
	}
	if decodeJSON(r, &body) != nil {
		writeJSON(w, 400, map[string]string{"erro": "Dados inválidos."})
		return
	}
	if err := s.deps.AdminInvoices.Save(r.Context(), id, body.ClientID, body.InvoiceEdit); err != nil {
		status := 500
		if errors.Is(err, admin.ErrInvalidInvoice) {
			status = 400
		}
		writeJSON(w, status, map[string]string{"erro": mapAdminError(err)})
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (s *Server) adminInvoiceStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if decodeJSON(r, &body) != nil {
		writeJSON(w, 400, map[string]string{"erro": "Dados inválidos."})
		return
	}
	if err := s.deps.AdminInvoices.SetStatus(r.Context(), r.PathValue("id"), body.Status); err != nil {
		status := 500
		if errors.Is(err, admin.ErrInvalidInvoice) {
			status = 400
		}
		writeJSON(w, status, map[string]string{"erro": mapAdminError(err)})
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (s *Server) adminImport(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.deps.AdminImporter == nil {
		writeJSON(w, 503, map[string]string{"erro": "Importação indisponível."})
		return
	}
	var in admin.ImportInput
	if decodeJSON(r, &in) != nil {
		writeJSON(w, 400, map[string]string{"erro": "Dados inválidos."})
		return
	}
	out, err := s.deps.AdminImporter.Import(r.Context(), in)
	if err != nil {
		status := 500
		if errors.Is(err, admin.ErrInvalidImport) {
			status = 400
		}
		writeJSON(w, status, map[string]string{"erro": mapAdminError(err)})
		return
	}
	writeJSON(w, 200, out)
}
func (s *Server) adminDeleteAll(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var body struct {
		Confirmation string `json:"confirmacao"`
	}
	if decodeJSON(r, &body) != nil {
		writeJSON(w, 400, map[string]string{"erro": "Dados inválidos."})
		return
	}
	out, err := s.deps.AdminInvoices.DeleteAll(r.Context(), body.Confirmation)
	if err != nil {
		status := 500
		if errors.Is(err, admin.ErrInvalidInvoice) {
			status = 400
		}
		writeJSON(w, status, map[string]string{"erro": mapAdminError(err)})
		return
	}
	writeJSON(w, 200, out)
}
func mapAdminError(err error) string {
	switch {
	case errors.Is(err, admin.ErrInvalidImport):
		return "Lote de importação inválido."
	case errors.Is(err, admin.ErrInvalidInvoice):
		return "Dados da fatura inválidos."
	default:
		return "Não foi possível concluir a operação."
	}
}

func (s *Server) adminTransactions(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.deps.AdminActivity == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"erro": "Transações indisponíveis."})
		return
	}
	history := r.URL.Query().Get("historico") == "true"
	limit, _ := strconv.Atoi(r.URL.Query().Get("limite"))
	out, err := s.deps.AdminActivity.Transactions(r.Context(), r.URL.Query().Get("status"), !history, limit)
	if err != nil {
		s.logger.Error("admin transactions failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"erro": "Não foi possível carregar as transações."})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) adminPayments(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.deps.AdminActivity == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"erro": "Pagamentos indisponíveis."})
		return
	}
	days, _ := strconv.Atoi(r.URL.Query().Get("dias"))
	out, err := s.deps.AdminActivity.Payments(r.Context(), r.URL.Query().Get("gateway"), days)
	if err != nil {
		s.logger.Error("admin payments failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"erro": "Não foi possível carregar os pagamentos recebidos."})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) adminLogs(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if s.deps.AdminActivity == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"erro": "Logs indisponíveis."})
		return
	}
	out, err := s.deps.AdminActivity.Logs(r.Context())
	if err != nil {
		s.logger.Error("admin logs failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"erro": "Não foi possível carregar os logs."})
		return
	}
	writeJSON(w, http.StatusOK, out)
}
