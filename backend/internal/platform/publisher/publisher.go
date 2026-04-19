package publisher

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nsqio/go-nsq"
)

// Publisher abstracts event publishing so services don't depend on NSQ directly.
type Publisher interface {
	// PublishEvent serializes payload into an EventEnvelope and publishes to the given topic.
	PublishEvent(topic string, payload any) error
	// Stop gracefully shuts down the publisher.
	Stop()
}

// NSQPublisher publishes events to NSQ.
type NSQPublisher struct {
	producer *nsq.Producer
}

// NewNSQPublisher creates a Publisher backed by an NSQ producer connected to nsqdAddr (host:port).
func NewNSQPublisher(nsqdAddr string) (*NSQPublisher, error) {
	cfg := nsq.NewConfig()
	producer, err := nsq.NewProducer(nsqdAddr, cfg)
	if err != nil {
		return nil, fmt.Errorf("nsq producer: %w", err)
	}
	producer.SetLogger(nil, nsq.LogLevelError) // suppress noisy NSQ logs
	return &NSQPublisher{producer: producer}, nil
}

// PublishEvent implements Publisher.
func (p *NSQPublisher) PublishEvent(topic string, payload any) error {
	env := NewEnvelope(topic, payload)
	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	if err := p.producer.Publish(topic, data); err != nil {
		return fmt.Errorf("nsq publish: %w", err)
	}
	slog.Info("published event", "topic", topic)
	return nil
}

// Stop implements Publisher.
func (p *NSQPublisher) Stop() {
	p.producer.Stop()
}

// NoopPublisher discards all events. Used when NSQ is not configured.
type NoopPublisher struct{}

// PublishEvent implements Publisher by logging and discarding.
func (n *NoopPublisher) PublishEvent(topic string, _ any) error {
	slog.Debug("noop publisher: discarding event", "topic", topic)
	return nil
}

// Stop implements Publisher (no-op).
func (n *NoopPublisher) Stop() {}
