package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"log-analytics-system/internal/app/query"
	cassdb "log-analytics-system/internal/db/cassandra"
	chdb "log-analytics-system/internal/db/clickhouse"
)

func main() {
	ch := chdb.New(os.Getenv("CLICKHOUSE_DSN"))
	if err := ch.InitSchema(context.Background()); err != nil {
		log.Fatalf("clickhouse init error: %v", err)
	}
	cass := cassdb.New([]string{os.Getenv("CASSANDRA_HOST")}, os.Getenv("CASSANDRA_KEYSPACE"))
	_ = cass.InitSchema(context.Background())
	handler := query.NewHandler(ch, cass)
	log.Println("query service listening on :8082")
	if err := http.ListenAndServe(":8082", handler); err != nil {
		log.Fatal(err)
	}
}
