package ingestion

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"log-analytics-system/internal/db/cassandra"
	ckdb "log-analytics-system/internal/db/cockroach"
	"log-analytics-system/internal/kafka"
)

// LogEvent represents a single log event.
type LogEvent struct {
	ProjectID string            `json:"project_id"`
	APIKey    string            `json:"api_key"`
	Name      string            `json:"name"`
	Timestamp time.Time         `json:"timestamp"`
	Fields    map[string]string `json:"fields"`
}

// Handler handles log ingestion requests.
type Handler struct {
	producer  *kafka.Producer
	cockroach *ckdb.Conn
	cass      *cassandra.Session
}

// NewHandler creates a new Handler with real connections.
func NewHandler() http.Handler {
	prod := kafka.NewProducer([]string{os.Getenv("KAFKA_BROKERS")}, os.Getenv("KAFKA_TOPIC"))
	ck := ckdb.New(os.Getenv("COCKROACH_DSN"))
	cass := cassandra.New([]string{os.Getenv("CASSANDRA_HOST")}, os.Getenv("CASSANDRA_KEYSPACE"))
	_ = cass.InitSchema(context.Background())
	return &Handler{producer: prod, cockroach: ck, cass: cass}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var ev LogEvent
	if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// validate project ID and API key using Cassandra
	ctx := context.Background()
	proj, err := h.cass.GetProject(ctx, ev.ProjectID)
	if err != nil || proj.APIKey != ev.APIKey {
		http.Error(w, "invalid project", http.StatusUnauthorized)
		return
	}
	for k := range ev.Fields {
		allowed := false
		for _, a := range proj.SearchKeys {
			if k == a {
				allowed = true
				break
			}
		}
		if !allowed {
			http.Error(w, "invalid field key", http.StatusBadRequest)
			return
		}
	}
	expire := ev.Timestamp.Add(time.Duration(proj.TTL) * time.Second).Unix()
	data, _ := json.Marshal(struct {
		ProjectID string            `json:"project_id"`
		Name      string            `json:"name"`
		Timestamp int64             `json:"timestamp"`
		ExpireAt  int64             `json:"expire_at"`
		Fields    map[string]string `json:"fields"`
	}{
		ProjectID: ev.ProjectID,
		Name:      ev.Name,
		Timestamp: ev.Timestamp.Unix(),
		ExpireAt:  expire,
		Fields:    ev.Fields,
	})
	_ = h.producer.Publish(ctx, data)
	_ = h.cockroach.InsertAudit(ctx, ev.ProjectID, ev.Name, ev.Timestamp)
	w.WriteHeader(http.StatusAccepted)
}
