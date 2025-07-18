package kafka

import (
    "context"
    "log"

    "github.com/segmentio/kafka-go"
)

// Consumer reads messages from Kafka.
type Consumer struct {
    reader *kafka.Reader
}

// NewConsumer creates a new consumer for the given brokers and topic.
func NewConsumer(brokers []string, topic, group string) *Consumer {
    return &Consumer{reader: kafka.NewReader(kafka.ReaderConfig{
        Brokers: brokers,
        Topic:   topic,
        GroupID: group,
    })}
}

// Consume receives the next message.
func (c *Consumer) Consume(ctx context.Context) ([]byte, error) {
    m, err := c.reader.ReadMessage(ctx)
    if err != nil {
        if err == context.Canceled {
            return nil, err
        }
        log.Printf("kafka read error: %v", err)
        return nil, err
    }
    return m.Value, nil
}
