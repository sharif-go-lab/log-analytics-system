package query

import (
	"context"
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
func NewHandler(chConn *ch.Conn, cass *cassandra.Session) *Handler {
	return &Handler{ch: chConn, cass: cass}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		http.Error(w, "missing project", http.StatusBadRequest)
		return
	}
	ctx := context.Background()
	proj, err := h.cass.GetProject(ctx, project)
	if err != nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	filters := map[string]string{}
	for k := range r.URL.Query() {
		if k != "project" && k != "name" && k != "limit" && k != "offset" {
			allowed := false
			for _, a := range proj.SearchKeys {
				if k == a {
					allowed = true
					break
				}
			}
			if !allowed {
				http.Error(w, "invalid filter", http.StatusBadRequest)
				return
			}
			filters[k] = r.URL.Query().Get(k)
		}
	}
	if r.URL.Path == "/events" {
		name := r.URL.Query().Get("name")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit == 0 {
			limit = 10
		}
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		events, err := h.ch.QueryEvents(ctx, project, name, filters, limit, offset)
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
