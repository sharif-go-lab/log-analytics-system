package main

import (
    "log"
    "net/http"
    "os"

    chdb "log-analytics-system/internal/db/clickhouse"
    "log-analytics-system/internal/app/query"
)

func main() {
    ch := chdb.New(os.Getenv("CLICKHOUSE_DSN"))
    handler := query.NewHandler(ch)
    log.Println("query service listening on :8082")
    if err := http.ListenAndServe(":8082", handler); err != nil {
        log.Fatal(err)
    }
}
