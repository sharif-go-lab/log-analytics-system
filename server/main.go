package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/gin-gonic/gin"
	"github.com/gocql/gocql"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
)

type App struct {
	router      *gin.Engine
	kafkaWriter *kafka.Writer
	kafkaReader *kafka.Reader
	cassandra   *gocql.Session
	cockroach   *pgxpool.Pool
	clickhouse  clickhouse.Conn
}

func main() {
	app := &App{}
	if err := app.initConnections(); err != nil {
		log.Fatalf("init: %v", err)
	}
	defer app.shutdown()

	app.routes()
	go app.consumeLoop()

	if err := app.router.Run(":8081"); err != nil {
		log.Fatal(err)
	}
}

func (a *App) initConnections() error {
	a.router = gin.Default()

	// Cassandra
	if err := a.initCassandra(); err != nil {
		return err
	}

	// Kafka
	a.kafkaWriter = &kafka.Writer{
		Addr:     kafka.TCP("kafka1:9092", "kafka2:9093", "kafka3:9094"),
		Topic:    "logs",
		Balancer: &kafka.LeastBytes{},
	}
	a.kafkaReader = kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{"kafka1:9092", "kafka2:9093", "kafka3:9094"},
		Topic:   "logs",
		GroupID: "log-consumer",
	})

	// CockroachDB
	dbpool, err := pgxpool.New(context.Background(), "postgresql://root@cockroach1:26257/defaultdb?sslmode=disable")
	if err != nil {
		return err
	}
	a.cockroach = dbpool
	if err := a.initSQL(); err != nil {
		return err
	}

	// ClickHouse
	ch, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{"clickhouse:9000"},
		Auth: clickhouse.Auth{Database: "default"},
	})
	if err != nil {
		return err
	}
	a.clickhouse = ch
	if err := a.initClickhouse(); err != nil {
		return err
	}

	return nil
}

func (a *App) shutdown() {
	a.kafkaWriter.Close()
	a.kafkaReader.Close()
	a.cassandra.Close()
	a.cockroach.Close()
}

func (a *App) routes() {
	a.router.POST("/log", a.handleLog)
	a.router.POST("/projects", a.createProject)
	a.router.GET("/projects", a.listProjects)
	a.router.POST("/login", a.login)
	a.router.GET("/projects/:id/events", a.searchEvents)
	a.router.GET("/projects/:id/events/:name", a.eventDetail)
	a.router.GET("/healthz", a.healthz)
}

// ----- initialization helpers -----

func (a *App) initCassandra() error {
	cluster := gocql.NewCluster("cassandra1", "cassandra2", "cassandra3")
	cluster.Consistency = gocql.Quorum
	// connect without keyspace to create it
	session, err := cluster.CreateSession()
	if err != nil {
		return err
	}
	if err := session.Query(`CREATE KEYSPACE IF NOT EXISTS logs WITH replication = {'class':'SimpleStrategy','replication_factor':3}`).Exec(); err != nil {
		session.Close()
		return err
	}
	session.Close()
	cluster.Keyspace = "logs"
	session, err = cluster.CreateSession()
	if err != nil {
		return err
	}
	if err := session.Query(`CREATE TABLE IF NOT EXISTS events (project_id text, name text, timestamp timestamp, data text, PRIMARY KEY ((project_id), name, timestamp))`).Exec(); err != nil {
		session.Close()
		return err
	}
	a.cassandra = session
	return nil
}

func (a *App) initSQL() error {
	ctx := context.Background()
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
            id SERIAL PRIMARY KEY,
            username STRING UNIQUE,
            password STRING
        )`,
		`CREATE TABLE IF NOT EXISTS projects (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            name STRING,
            api_key STRING,
            search_keys STRING[],
            ttl INT
        )`,
		`CREATE TABLE IF NOT EXISTS user_projects (
            user_id INT REFERENCES users(id),
            project_id UUID REFERENCES projects(id)
        )`,
	}
	for _, s := range stmts {
		if _, err := a.cockroach.Exec(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) initClickhouse() error {
	ctx := context.Background()
	ddl := `CREATE TABLE IF NOT EXISTS events (
        project_id UUID,
        name String,
        timestamp DateTime,
        data String
    ) ENGINE = MergeTree() ORDER BY (project_id, name, timestamp)`
	return a.clickhouse.Exec(ctx, ddl)
}

// ----- handlers -----

type LogRequest struct {
	ProjectID string            `json:"project_id"`
	APIKey    string            `json:"api_key"`
	Name      string            `json:"name"`
	Timestamp time.Time         `json:"timestamp"`
	Data      map[string]string `json:"data"`
}

func (a *App) handleLog(c *gin.Context) {
	var req LogRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()
	var storedKey string
	if err := a.cockroach.QueryRow(ctx, `SELECT api_key FROM projects WHERE id=$1`, req.ProjectID).Scan(&storedKey); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid project"})
		return
	}
	if storedKey != req.APIKey {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
		return
	}
	data, _ := json.Marshal(req)
	if err := a.kafkaWriter.WriteMessages(ctx, kafka.Message{Value: data}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusAccepted)
}

type ProjectRequest struct {
	Name       string   `json:"name"`
	SearchKeys []string `json:"search_keys"`
	TTL        int      `json:"ttl"`
}

func randomKey() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (a *App) createProject(c *gin.Context) {
	var req ProjectRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	apiKey := randomKey()
	var id string
	err := a.cockroach.QueryRow(c.Request.Context(), `INSERT INTO projects (name, api_key, search_keys, ttl) VALUES ($1,$2,$3,$4) RETURNING id`, req.Name, apiKey, req.SearchKeys, req.TTL).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "api_key": apiKey})
}

func (a *App) listProjects(c *gin.Context) {
	rows, err := a.cockroach.Query(c.Request.Context(), `SELECT id, name FROM projects`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	var res []map[string]string
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			continue
		}
		res = append(res, map[string]string{"id": id, "name": name})
	}
	c.JSON(http.StatusOK, res)
}

func (a *App) login(c *gin.Context) {
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BindJSON(&credentials); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var stored string
	err := a.cockroach.QueryRow(c.Request.Context(), `SELECT password FROM users WHERE username=$1`, credentials.Username).Scan(&stored)
	if err != nil || stored != credentials.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid"})
		return
	}
	c.Status(http.StatusOK)
}

func (a *App) searchEvents(c *gin.Context) {
	projectID := c.Param("id")
	name := c.Query("name")
	query := `SELECT name, timestamp, data FROM events WHERE project_id=?`
	var args []interface{}
	args = append(args, projectID)
	if name != "" {
		query += " AND name=?"
		args = append(args, name)
	}
	query += " ORDER BY timestamp DESC LIMIT 100"
	rows, err := a.clickhouse.Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	type row struct {
		Name      string    `json:"name"`
		Timestamp time.Time `json:"timestamp"`
		Data      string    `json:"data"`
	}
	var out []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.Name, &r.Timestamp, &r.Data); err != nil {
			continue
		}
		out = append(out, r)
	}
	c.JSON(http.StatusOK, out)
}

func (a *App) eventDetail(c *gin.Context) {
	projectID := c.Param("id")
	name := c.Param("name")
	row := a.clickhouse.QueryRow(c.Request.Context(), `SELECT name, timestamp, data FROM events WHERE project_id=? AND name=? ORDER BY timestamp DESC LIMIT 1`, projectID, name)
	var r struct {
		Name      string    `json:"name"`
		Timestamp time.Time `json:"timestamp"`
		Data      string    `json:"data"`
	}
	if err := row.Scan(&r.Name, &r.Timestamp, &r.Data); err != nil {
		if err == sql.ErrNoRows {
			c.Status(http.StatusNotFound)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, r)
}

func (a *App) healthz(c *gin.Context) {
	if err := a.cockroach.Ping(c.Request.Context()); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

// ----- consumer -----

type event struct {
	ProjectID string            `json:"project_id"`
	Name      string            `json:"name"`
	Timestamp time.Time         `json:"timestamp"`
	Data      map[string]string `json:"data"`
}

func (a *App) consumeLoop() {
	for {
		m, err := a.kafkaReader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("read kafka: %v", err)
			time.Sleep(time.Second)
			continue
		}
		var ev event
		if err := json.Unmarshal(m.Value, &ev); err != nil {
			continue
		}
		ttl := 0
		if err := a.cockroach.QueryRow(context.Background(), `SELECT ttl FROM projects WHERE id=$1`, ev.ProjectID).Scan(&ttl); err != nil {
			log.Printf("ttl lookup: %v", err)
			continue
		}
		// insert into Cassandra with TTL
		jsonData, _ := json.Marshal(ev.Data)
		if err := a.cassandra.Query(`INSERT INTO events (project_id, name, timestamp, data) VALUES (?,?,?,?) USING TTL ?`, ev.ProjectID, ev.Name, ev.Timestamp, string(jsonData), ttl).Exec(); err != nil {
			log.Printf("cassandra insert: %v", err)
		}
		// insert into ClickHouse
		if err := a.clickhouse.Exec(context.Background(), `INSERT INTO events (project_id, name, timestamp, data) VALUES (?,?,?,?)`, ev.ProjectID, ev.Name, ev.Timestamp, string(jsonData)); err != nil {
			log.Printf("clickhouse insert: %v", err)
		}
	}
}
