package main

import (
	"bytes"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

// Event is the payload sent to the ingestion API
type Event struct {
	ProjectID string            `json:"project_id"`
	APIKey    string            `json:"api_key"`
	Name      string            `json:"name"`
	Timestamp time.Time         `json:"timestamp"`
	Fields    map[string]string `json:"fields"`
}

func main() {
	url := os.Getenv("INGEST_URL")
	if url == "" {
		url = "http://localhost:8080/"
	}
	project := os.Getenv("PROJECT_ID")
	if project == "" {
		project = "proj1"
	}
	key := os.Getenv("API_KEY")
	if key == "" {
		key = "k"
	}

	names := []string{"login", "purchase", "error"}
	users := []string{"alice", "bob", "carol"}
	levels := []string{"info", "warn", "error"}

	client := &http.Client{Timeout: 5 * time.Second}
	rand.Seed(time.Now().UnixNano())

	for {
		ev := Event{
			ProjectID: project,
			APIKey:    key,
			Name:      names[rand.Intn(len(names))],
			Timestamp: time.Now().UTC(),
			Fields: map[string]string{
				"user":  users[rand.Intn(len(users))],
				"level": levels[rand.Intn(len(levels))],
			},
		}
		data, err := json.Marshal(ev)
		if err != nil {
			log.Printf("marshal: %v", err)
			continue
		}
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
		if err != nil {
			log.Printf("request: %v", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("post: %v", err)
		} else {
			resp.Body.Close()
		}
		time.Sleep(time.Second)
	}
}
