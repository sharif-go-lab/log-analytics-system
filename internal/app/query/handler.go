package query

import (
	"encoding/json"
	"net/http"
	"strconv"

	"log-analytics-system/internal/db/cassandra"
	ch "log-analytics-system/internal/db/clickhouse"
)

// Handler provides query endpoints for events.
type Handler struct {
	ch   *ch.Conn
	cass *cassandra.Session
}

// NewHandler initializes a query handler.
func NewHandler(chConn *ch.Conn, c *cassandra.Session) *Handler {
	return &Handler{ch: chConn, cass: c}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		http.Error(w, "missing project", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	proj, err := h.cass.GetProject(ctx, project)
	if err != nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	filters := map[string]string{}
	allowed := map[string]struct{}{}
	for _, k := range proj.SearchKeys {
		allowed[k] = struct{}{}
	}
	for k := range r.URL.Query() {
		if k != "project" && k != "name" && k != "limit" {
			if _, ok := allowed[k]; ok {
				filters[k] = r.URL.Query().Get(k)
			}
		}
	}
	if r.URL.Path == "/events" {
		name := r.URL.Query().Get("name")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit == 0 {
			limit = 10
		}
		events, err := h.ch.QueryEvents(ctx, project, name, filters, limit, 0)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(events)
		return
	}
	counts, err := h.ch.QueryCounts(ctx, project, filters)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(counts)
}
