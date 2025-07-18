package web

import (
    "context"
    "html/template"
    "net/http"
    "os"

    "log-analytics-system/internal/db/cassandra"
    ck "log-analytics-system/internal/db/cockroach"
    ch "log-analytics-system/internal/db/clickhouse"
)

// Server provides minimal web UI.
type Server struct {
    cass *cassandra.Session
    ck   *ck.Conn
    ch   *ch.Conn
    tpl  *template.Template
}

// NewServer creates a server and initializes schemas.
func NewServer() *Server {
    cass := cassandra.New([]string{os.Getenv("CASSANDRA_HOST")}, os.Getenv("CASSANDRA_KEYSPACE"))
    cass.InitSchema(context.Background())
    ckdb := ck.New(os.Getenv("COCKROACH_DSN"))
    ckdb.InitSchema(context.Background())
    chdb := ch.New(os.Getenv("CLICKHOUSE_DSN"))
    chdb.InitSchema(context.Background())
    tpl := template.Must(template.ParseGlob("templates/*.html"))
    return &Server{cass: cass, ck: ckdb, ch: chdb, tpl: tpl}
}

// Routes returns the http handler.
func (s *Server) Routes() http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("/login", s.login)
    mux.HandleFunc("/projects", s.projects)
    mux.HandleFunc("/events", s.events)
    return mux
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        _ = s.tpl.ExecuteTemplate(w, "login.html", nil)
    case http.MethodPost:
        user := r.FormValue("username")
        pass := r.FormValue("password")
        ok, _ := s.ck.CheckUser(r.Context(), user, pass)
        if !ok {
            http.Error(w, "invalid", http.StatusUnauthorized)
            return
        }
        http.Redirect(w, r, "/projects", http.StatusSeeOther)
    }
}

func (s *Server) projects(w http.ResponseWriter, r *http.Request) {
    projs, _ := s.cass.ListProjects(r.Context())
    _ = s.tpl.ExecuteTemplate(w, "projects.html", projs)
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
    project := r.URL.Query().Get("project")
    name := r.URL.Query().Get("name")
    evs, _ := s.ch.QueryEvents(r.Context(), project, name, nil, 20, 0)
    _ = s.tpl.ExecuteTemplate(w, "events.html", evs)
}
