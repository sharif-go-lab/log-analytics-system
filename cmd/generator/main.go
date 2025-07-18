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

type payload struct {
	Project   string    `json:"project"`
	Data      string    `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	url := flag.String("url", "http://localhost:8080/ingest", "Ingestion API URL")
	interval := flag.Duration("interval", time.Second, "Interval between requests")
	flag.Parse()

	projects := []string{"alpha", "beta", "gamma"}

	client := &http.Client{Timeout: 5 * time.Second}
	rand.Seed(time.Now().UnixNano())

	for {
		body := payload{
			Project:   projects[rand.Intn(len(projects))],
			Data:      "sample log entry",
			Timestamp: time.Now().UTC(),
		}
		b, err := json.Marshal(body)
		if err != nil {
			log.Printf("failed to marshal payload: %v", err)
			continue
		}

		req, err := http.NewRequest("POST", *url, bytes.NewReader(b))
		if err != nil {
			log.Printf("failed to create request: %v", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("request error: %v", err)
		} else {
			resp.Body.Close()
			if resp.StatusCode >= 300 {
				log.Printf("non-OK status: %s", resp.Status)
			} else {
				log.Printf("sent project %s", body.Project)
			}
		}

		time.Sleep(*interval)
	}
}
