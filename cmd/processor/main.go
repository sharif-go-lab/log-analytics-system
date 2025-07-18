package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    chdb "log-analytics-system/internal/db/clickhouse"
    "log-analytics-system/internal/app/processor"
    "log-analytics-system/internal/kafka"
)

func main() {
    ch := chdb.New(os.Getenv("CLICKHOUSE_DSN"))
    cons := kafka.NewConsumer([]string{os.Getenv("KAFKA_BROKERS")}, os.Getenv("KAFKA_TOPIC"), "processor")
    proc := processor.New(cons, ch)

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    log.Println("processor started")
    proc.Run(ctx)
    log.Println("processor stopped")
}
