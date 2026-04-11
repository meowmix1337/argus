package events

import "log/slog"

// NoopPublisher discards all events. Used when NSQ is not configured.
type NoopPublisher struct{}

// PublishEvent implements Publisher by logging and discarding.
func (n *NoopPublisher) PublishEvent(topic string, _ any) error {
	slog.Debug("noop publisher: discarding event", "topic", topic)
	return nil
}

// Stop implements Publisher (no-op).
func (n *NoopPublisher) Stop() {}
