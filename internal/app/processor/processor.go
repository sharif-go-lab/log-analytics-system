package processor

import (
    "context"
    "encoding/json"
    "log"

    "log-analytics-system/internal/db/clickhouse"
    "log-analytics-system/internal/kafka"
)

// Processor consumes events from Kafka and stores them in ClickHouse.
type Processor struct {
    consumer *kafka.Consumer
    click    *clickhouse.Conn
}

// New creates a Processor.
func New(cons *kafka.Consumer, ch *clickhouse.Conn) *Processor {
    return &Processor{consumer: cons, click: ch}
}

// Run starts consuming until ctx is canceled.
func (p *Processor) Run(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        default:
        }
        msg, err := p.consumer.Consume(ctx)
        if err != nil {
            continue
        }
        var ev struct {
            ProjectID string            `json:"project_id"`
            Name      string            `json:"name"`
            Timestamp int64             `json:"timestamp"`
            Fields    map[string]string `json:"fields"`
        }
        if err := json.Unmarshal(msg, &ev); err != nil {
            log.Printf("failed to decode event: %v", err)
            continue
        }
        if err := p.click.InsertEvent(ctx, ev.ProjectID, ev.Name, ev.Timestamp, ev.Fields); err != nil {
            log.Printf("clickhouse insert error: %v", err)
        }
    }
}
