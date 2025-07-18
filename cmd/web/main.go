package main

import (
    "log"
    "net/http"

    "log-analytics-system/internal/app/web"
)

func main() {
    srv := web.NewServer()
    log.Println("web ui listening on :8083")
    http.ListenAndServe(":8083", srv.Routes())
}
