package events

import (
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/nsqio/go-nsq"
)

const (
	maxRetries      = 5
	baseBackoffSecs = 5
)

// MessageHandler is the interface each consumer must implement.
type MessageHandler interface {
	// Topic returns the NSQ topic to subscribe to.
	Topic() string
	// Channel returns the stable channel name for this consumer.
	Channel() string
	// Process handles a single message body. Return an error to trigger retry.
	Process(body []byte) error
}

// ConsumerManager registers and manages NSQ consumers.
type ConsumerManager struct {
	lookupdAddr string
	consumers   []*nsq.Consumer
}

// NewConsumerManager creates a manager that connects consumers to the given nsqlookupd HTTP address.
func NewConsumerManager(lookupdAddr string) *ConsumerManager {
	return &ConsumerManager{lookupdAddr: lookupdAddr}
}

// Register adds a MessageHandler as an NSQ consumer.
func (cm *ConsumerManager) Register(h MessageHandler) error {
	cfg := nsq.NewConfig()
	consumer, err := nsq.NewConsumer(h.Topic(), h.Channel(), cfg)
	if err != nil {
		return fmt.Errorf("new consumer %s/%s: %w", h.Topic(), h.Channel(), err)
	}
	consumer.SetLogger(nil, nsq.LogLevelError)

	consumer.AddHandler(nsq.HandlerFunc(func(msg *nsq.Message) error {
		return handleWithRetry(h, msg)
	}))

	cm.consumers = append(cm.consumers, consumer)
	return nil
}

// Start connects all registered consumers to nsqlookupd.
func (cm *ConsumerManager) Start() error {
	for _, c := range cm.consumers {
		if err := c.ConnectToNSQLookupd(cm.lookupdAddr); err != nil {
			return fmt.Errorf("connect to nsqlookupd: %w", err)
		}
	}
	slog.Info("consumer manager started", "consumers", len(cm.consumers), "lookupd", cm.lookupdAddr)
	return nil
}

// Stop gracefully shuts down all consumers.
func (cm *ConsumerManager) Stop() {
	for _, c := range cm.consumers {
		c.Stop()
	}
	for _, c := range cm.consumers {
		<-c.StopChan
	}
	slog.Info("consumer manager stopped")
}

// handleWithRetry processes a message with exponential backoff.
// After maxRetries, the message is logged and ACKed (discarded) to prevent poison-pill loops.
func handleWithRetry(h MessageHandler, msg *nsq.Message) error {
	if err := h.Process(msg.Body); err != nil {
		attempts := int(msg.Attempts)
		if attempts >= maxRetries {
			slog.Error("discarding message after max retries",
				"topic", h.Topic(),
				"channel", h.Channel(),
				"attempts", attempts,
				"error", err,
			)
			msg.Finish()
			return nil
		}
		delay := backoffDelay(attempts)
		slog.Warn("retrying message",
			"topic", h.Topic(),
			"channel", h.Channel(),
			"attempt", attempts,
			"delay", delay,
			"error", err,
		)
		msg.RequeueWithoutBackoff(delay)
		return nil
	}
	return nil
}

// backoffDelay returns the exponential backoff duration for the given attempt number.
// backoffDelay(1)=5s, backoffDelay(2)=10s, backoffDelay(3)=20s, backoffDelay(4)=40s, backoffDelay(5)=80s.
func backoffDelay(attempt int) time.Duration {
	secs := float64(baseBackoffSecs) * math.Pow(2, float64(attempt-1))
	return time.Duration(secs) * time.Second
}
