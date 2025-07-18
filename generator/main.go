package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"log"
	"math/rand"
	"net/http"
	"time"
)

type Event struct {
	ProjectID string            `json:"project_id"`
	APIKey    string            `json:"api_key"`
	Name      string            `json:"name"`
	Timestamp time.Time         `json:"timestamp"`
	Data      map[string]string `json:"data"`
}

func main() {
	endpoint := flag.String("endpoint", "http://localhost:8081/log", "log endpoint")
	project := flag.String("project", "", "project id")
	apiKey := flag.String("key", "", "api key")
	count := flag.Int("n", 100, "number of events")
	flag.Parse()

	if *project == "" || *apiKey == "" {
		log.Fatal("project and key required")
	}

	for i := 0; i < *count; i++ {
		ev := Event{
			ProjectID: *project,
			APIKey:    *apiKey,
			Name:      "test_event",
			Timestamp: time.Now(),
			Data: map[string]string{
				"value": randSeq(5),
			},
		}
		body, _ := json.Marshal(ev)
		resp, err := http.Post(*endpoint, "application/json", bytes.NewReader(body))
		if err != nil {
			log.Printf("send: %v", err)
			continue
		}
		resp.Body.Close()
		time.Sleep(100 * time.Millisecond)
	}
}

var letters = []rune("abcdefghijklmnopqrstuvwxyz")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
