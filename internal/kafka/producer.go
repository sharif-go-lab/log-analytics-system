package kafka

import (
	"context"
	"log"

	"github.com/segmentio/kafka-go"
)

// Producer publishes events to Kafka.
type Producer struct {
	writer *kafka.Writer
}

// NewProducer creates a new Producer.
func NewProducer(brokers []string, topic string) *Producer {
	return &Producer{writer: &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}}
}

// Publish publishes data to Kafka.
func (p *Producer) Publish(ctx context.Context, data []byte) error {
	err := p.writer.WriteMessages(ctx, kafka.Message{Value: data})
	if err != nil {
		log.Printf("failed to publish message: %v", err)
	}
	return err
}
