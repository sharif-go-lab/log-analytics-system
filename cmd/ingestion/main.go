package main

import (
	"log"
	"net/http"

	"log-analytics-system/internal/app/ingestion"
)

func main() {
	handler := ingestion.NewHandler()
	log.Println("ingestion service listening on :8080")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatal(err)
	}
}
