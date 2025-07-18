package web

import (
	"context"
	"html/template"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"log-analytics-system/internal/db/cassandra"
	ch "log-analytics-system/internal/db/clickhouse"
	ck "log-analytics-system/internal/db/cockroach"
)

// Server provides minimal web UI.
type Server struct {
	cass *cassandra.Session
	ck   *ck.Conn
	ch   *ch.Conn
	tpl  *template.Template
	// simple cookie based session no storage
}

// NewServer creates a server and initializes schemas.
func NewServer() *Server {
	cass := cassandra.New([]string{os.Getenv("CASSANDRA_HOST")}, os.Getenv("CASSANDRA_KEYSPACE"))
	cass.InitSchema(context.Background())
	ckdb := ck.New(os.Getenv("COCKROACH_DSN"))
	ckdb.InitSchema(context.Background())
	ckdb.EnsureUser(context.Background(), "admin", "admin")
	chdb := ch.New(os.Getenv("CLICKHOUSE_DSN"))
	chdb.InitSchema(context.Background())
	tpl := template.Must(template.New("").Funcs(template.FuncMap{"add": func(a, b int) int { return a + b }}).ParseGlob("templates/*.html"))
	return &Server{cass: cass, ck: ckdb, ch: chdb, tpl: tpl}
}

// Routes returns the http handler.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", s.login)
	mux.HandleFunc("/projects", s.auth(s.projects))
	mux.HandleFunc("/projects/new", s.auth(s.newProject))
	mux.HandleFunc("/events", s.auth(s.events))
	mux.HandleFunc("/event", s.auth(s.eventDetail))
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
		http.SetCookie(w, &http.Cookie{Name: "user", Value: user, Path: "/", HttpOnly: true})
		http.Redirect(w, r, "/projects", http.StatusSeeOther)
	}
}

func (s *Server) projects(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(string)
	projs, _ := s.cass.ListProjects(r.Context(), user)
	_ = s.tpl.ExecuteTemplate(w, "projects.html", projs)
}

func (s *Server) newProject(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		_ = s.tpl.ExecuteTemplate(w, "create_project.html", nil)
	case http.MethodPost:
		id := r.FormValue("id")
		ttl, _ := strconv.Atoi(r.FormValue("ttl"))
		keys := strings.Split(r.FormValue("keys"), ",")
		for i := range keys {
			keys[i] = strings.TrimSpace(keys[i])
		}
		key := randomKey()
		user := r.Context().Value("user").(string)
		s.cass.CreateProject(r.Context(), cassandra.Project{ID: id, APIKey: key, TTL: ttl, Owner: user, SearchKeys: keys})
		_ = s.tpl.ExecuteTemplate(w, "project_key.html", struct{ APIKey string }{key})
	}
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	prj, _ := s.cass.GetProject(r.Context(), project)
	filters := map[string]string{}
	for _, k := range prj.SearchKeys {
		if v := r.URL.Query().Get(k); v != "" {
			filters[k] = v
		}
	}
	counts, _ := s.ch.QueryCounts(r.Context(), project, filters)
	qs := make(url.Values)
	for k, v := range filters {
		qs.Set(k, v)
	}
	filterQ := ""
	if len(filters) > 0 {
		filterQ = "&" + qs.Encode()
	}
	data := struct {
		Project string
		Keys    []string
		Values  map[string]string
		Counts  []ch.EventCount
		FilterQ string
	}{Project: project, Keys: prj.SearchKeys, Values: filters, Counts: counts, FilterQ: filterQ}
	_ = s.tpl.ExecuteTemplate(w, "events.html", data)
}

func (s *Server) eventDetail(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	name := r.URL.Query().Get("name")
	index, _ := strconv.Atoi(r.URL.Query().Get("index"))
	prj, _ := s.cass.GetProject(r.Context(), project)
	filters := map[string]string{}
	for _, k := range prj.SearchKeys {
		if v := r.URL.Query().Get(k); v != "" {
			filters[k] = v
		}
	}
	evs, _ := s.ch.QueryEvents(r.Context(), project, name, filters, 1, index)
	if len(evs) == 0 {
		http.NotFound(w, r)
		return
	}
	qs := make(url.Values)
	for k, v := range filters {
		qs.Set(k, v)
	}
	filterQ := ""
	if len(filters) > 0 {
		filterQ = "&" + qs.Encode()
	}
	data := struct {
		Project string
		Event   ch.Event
		Filters map[string]string
		Index   int
		FilterQ string
	}{Project: project, Event: evs[0], Filters: filters, Index: index, FilterQ: filterQ}
	_ = s.tpl.ExecuteTemplate(w, "event_detail.html", data)
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("user")
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), "user", cookie.Value)
		next(w, r.WithContext(ctx))
	}
}

func randomKey() string {
	b := make([]byte, 8)
	rand.Seed(time.Now().UnixNano())
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	for i := range b {
		b[i] = byte(letters[rand.Intn(len(letters))])
	}
	return string(b)
}
