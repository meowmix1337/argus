# Deep Interview Spec: NSQ Consumer Architecture

## Metadata
- Interview ID: nsq-consumer-2026-04-05
- Rounds: 3
- Final Ambiguity Score: ~19.5%
- Type: brownfield (augmenting argus-fi5 social feed plan)
- Generated: 2026-04-05
- Status: PASSED

## Clarity Breakdown
| Dimension | Score | Weight | Weighted |
|---|---|---|---|
| Goal Clarity | 0.85 | 35% | 0.298 |
| Constraint Clarity | 0.78 | 25% | 0.195 |
| Success Criteria | 0.80 | 25% | 0.200 |
| Context Clarity | 0.75 | 15% | 0.113 |
| **Total Clarity** | | | **0.806** |
| **Ambiguity** | | | **~19.5%** |

---

## Goal

Build a general-purpose NSQ consumer framework in `internal/events/` that:
1. Any future consumer (not just social feed) can plug into by implementing `MessageHandler`
2. Manages all consumer lifecycle (connect, start, stop, graceful shutdown) via `ConsumerManager`
3. Uses a structured JSON envelope so publisher and consumer never misalign on schema
4. Handles retries with exponential backoff and discards after max attempts

---

## Architecture Decisions

### 1. Message Payload — JSON Envelope

**File:** `backend/internal/events/payloads.go`

Every NSQ message body is a JSON-encoded `EventEnvelope`:

```go
// EventEnvelope wraps all NSQ messages with versioning and type metadata.
type EventEnvelope struct {
    Version    int             `json:"v"`       // schema version, start at 1
    EventType  string          `json:"type"`    // matches topic constant
    OccurredAt string          `json:"ts"`      // RFC3339 UTC
    Payload    json.RawMessage `json:"payload"` // typed per EventType
}

// PostCreatedPayload is the payload for TopicPostCreated events.
type PostCreatedPayload struct {
    PostID   string `json:"postId"`
    AuthorID string `json:"authorId"`
    Content  string `json:"content"` // sanitized; included for future consumers (analytics, notifications)
}

// PostLikedPayload is the payload for TopicPostLiked events.
type PostLikedPayload struct {
    PostID string `json:"postId"`
    UserID string `json:"userId"`
    Liked  bool   `json:"liked"` // true=liked, false=unliked
}

// UserFollowedPayload is the payload for TopicUserFollowed events.
type UserFollowedPayload struct {
    FollowerID  string `json:"followerId"`
    FollowingID string `json:"followingId"`
}
```

**Publisher helper** (add to `events.go`):
```go
// PublishEvent wraps payload in an EventEnvelope and publishes to the given topic.
func (b *EventBus) PublishEvent(topic string, payload any) error {
    raw, err := json.Marshal(payload)
    if err != nil {
        return err
    }
    env := EventEnvelope{
        Version:    1,
        EventType:  topic,
        OccurredAt: time.Now().UTC().Format(time.RFC3339),
        Payload:    raw,
    }
    data, err := json.Marshal(env)
    if err != nil {
        return err
    }
    return b.producer.Publish(topic, data)
}
```

---

### 2. General Consumer Framework

**File:** `backend/internal/events/consumer.go`

```go
// MessageHandler is the interface all NSQ consumers must implement.
// Adding a new consumer = implement this interface + Register() in main.go.
type MessageHandler interface {
    Topic() string   // NSQ topic to subscribe to
    Channel() string // NSQ channel name (stable, unique per consumer type)
    process(body []byte) error // business logic — unit-testable without NSQ
}

// ConsumerManager manages the lifecycle of all registered NSQ consumers.
type ConsumerManager struct {
    consumers  []*nsq.Consumer
    handlers   []MessageHandler
    lookupAddr string
    cfg        *nsq.Config
}

func NewConsumerManager(lookupAddr string) *ConsumerManager {
    cfg := nsq.NewConfig()
    cfg.MaxInFlight = 1 // default: one message at a time per consumer
    return &ConsumerManager{lookupAddr: lookupAddr, cfg: cfg}
}

// Register adds a consumer handler. Call before Start().
func (m *ConsumerManager) Register(h MessageHandler) error {
    c, err := nsq.NewConsumer(h.Topic(), h.Channel(), m.cfg)
    if err != nil {
        return err
    }
    c.AddHandler(nsq.HandlerFunc(func(msg *nsq.Message) error {
        return handleWithRetry(h, msg)
    }))
    m.consumers = append(m.consumers, c)
    m.handlers = append(m.handlers, h)
    return nil
}

// Start connects all registered consumers to NSQ lookupd.
func (m *ConsumerManager) Start() error {
    for _, c := range m.consumers {
        if err := c.ConnectToNSQLookupd(m.lookupAddr); err != nil {
            return err
        }
    }
    return nil
}

// Stop gracefully shuts down all consumers.
func (m *ConsumerManager) Stop() {
    for _, c := range m.consumers {
        c.Stop()
    }
}
```

---

### 3. Retry Policy — Exponential Backoff with Max Attempts

**Constants (add to `events/consumer.go`):**
```go
const (
    maxRetries  = 5                 // discard after 5 failed attempts
    baseBackoff = 5 * time.Second   // first retry after 5s
    maxBackoff  = 10 * time.Minute  // cap backoff at 10 minutes
)

// backoffDelay returns the requeue delay for attempt n (1-indexed).
// Sequence: 5s, 10s, 20s, 40s, 80s (capped at 10m).
func backoffDelay(attempt uint16) time.Duration {
    d := baseBackoff * (1 << (attempt - 1))
    if d > maxBackoff {
        d = maxBackoff
    }
    return d
}

func handleWithRetry(h MessageHandler, msg *nsq.Message) error {
    if err := h.process(msg.Body); err != nil {
        if msg.Attempts >= maxRetries {
            slog.Error("dropping NSQ message after max retries",
                "topic", h.Topic(),
                "channel", h.Channel(),
                "attempts", msg.Attempts,
                "err", err,
            )
            return nil // ACK to discard — prevents infinite retry loop
        }
        delay := backoffDelay(msg.Attempts)
        slog.Warn("requeueing NSQ message",
            "topic", h.Topic(),
            "attempt", msg.Attempts,
            "next_retry_in", delay,
            "err", err,
        )
        msg.RequeueWithoutBackoff(delay)
        return nil
    }
    return nil // ACK on success
}
```

---

### 4. Channel Naming Convention

Each `MessageHandler` implementation defines its own stable channel slug:

```go
// Naming rule: descriptive lowercase slug, hyphen-separated
// Must be stable — changing it creates a new orphaned channel in NSQ

func (c *FeedFanoutConsumer) Topic()   string { return events.TopicPostCreated }
func (c *FeedFanoutConsumer) Channel() string { return "feed-fanout" }

// Future consumers follow the same pattern:
// func (c *NotificationConsumer) Channel() string { return "notifications" }
// func (c *AnalyticsConsumer) Channel() string    { return "analytics" }
```

**Rule:** Never change a channel name in production without draining the old channel first.

---

### 5. Consumer Testing Strategy

Split `HandleMessage` (NSQ plumbing — not unit tested) from `process()` (business logic — fully unit tested):

```go
// FeedFanoutConsumer — process() is the testable unit
func (c *FeedFanoutConsumer) process(body []byte) error {
    var env events.EventEnvelope
    if err := json.Unmarshal(body, &env); err != nil {
        return fmt.Errorf("unmarshal envelope: %w", err)
    }
    var p events.PostCreatedPayload
    if err := json.Unmarshal(env.Payload, &p); err != nil {
        return fmt.Errorf("unmarshal payload: %w", err)
    }
    // business logic here — uses injected store/service dependencies
    slog.Info("feed fanout", "postId", p.PostID, "authorId", p.AuthorID)
    return nil
}

// Test file (no NSQ required):
func TestFeedFanoutConsumer_process(t *testing.T) {
    payload, _ := json.Marshal(events.PostCreatedPayload{
        PostID: "01JR4K...", AuthorID: "01JR3F...", Content: "hello",
    })
    env := events.EventEnvelope{Version: 1, EventType: events.TopicPostCreated,
        OccurredAt: time.Now().UTC().Format(time.RFC3339), Payload: payload}
    body, _ := json.Marshal(env)

    consumer := &events.FeedFanoutConsumer{store: mockStore}
    err := consumer.process(body)
    assert.NoError(t, err)
}
```

---

## File Structure for `internal/events/`

```
backend/internal/events/
├── events.go       # Publisher interface, EventBus, topic constants, PublishEvent helper
├── noop.go         # NoopPublisher (testing + NSQ-unavailable fallback)
├── payloads.go     # EventEnvelope + all typed payload structs (PostCreatedPayload, etc.)
└── consumer.go     # MessageHandler interface, ConsumerManager, handleWithRetry, backoff
```

**Future consumers live in the same package:**
```
└── feed_fanout.go    # FeedFanoutConsumer (Phase 4)
└── notification.go   # NotificationConsumer (Phase 5)
└── analytics.go      # AnalyticsConsumer (Phase N)
```

---

## main.go Wiring

```go
// cmd/server/main.go — consumer registration
if cfg.NSQLookupAddr != "" {
    cm := events.NewConsumerManager(cfg.NSQLookupAddr)

    feedFanout := &events.FeedFanoutConsumer{Store: followRepo}
    if err := cm.Register(feedFanout); err != nil {
        slog.Error("failed to register feed fanout consumer", "err", err)
    }
    // Future: cm.Register(&events.NotificationConsumer{...})

    if err := cm.Start(); err != nil {
        slog.Warn("NSQ consumer failed to start, continuing without consumers", "err", err)
    } else {
        defer cm.Stop()
    }
}
```

---

## Acceptance Criteria (Phase 4)

- [ ] `FeedFanoutConsumer.process()` unit tests pass without NSQ running
- [ ] `TestFeedFanoutConsumer_process` with valid envelope → no error, slog output logged
- [ ] `TestFeedFanoutConsumer_process` with malformed JSON → error returned (triggers requeue in prod)
- [ ] `ConsumerManager.Register()` wires topic + channel correctly (verified by inspection/integration)
- [ ] Consumer gracefully shuts down via `cm.Stop()` on server SIGTERM
- [ ] Server starts and functions normally when `NSQ_LOOKUPD_ADDR` is empty (NSQ disabled)
- [ ] `maxRetries = 5` constant is defined in `events/consumer.go`
- [ ] `backoffDelay(1)` = 5s, `backoffDelay(5)` = 80s (capped correctly)
- [ ] `go test -race ./internal/events/...` passes
- [ ] `golangci-lint run ./...` passes

---

## Constraints

- `ConsumerManager` is optional — if `NSQ_LOOKUPD_ADDR` is empty, no consumers are registered
- `MaxInFlight = 1` per consumer by default (one message at a time). Override per-consumer if needed in future.
- Channel names are permanent — changing them creates orphaned channels in NSQ
- Messages are dropped (ACK'd) after `maxRetries = 5` attempts, not sent to a DLQ (NSQ has no native DLQ)
- `process()` must be idempotent-safe at the storage layer (use INSERT OR IGNORE / upsert patterns) since NSQ is at-least-once

## Non-Goals
- No dead letter queue (DLQ) — log + discard on max retry
- No message ordering guarantees (NSQ is unordered by design)
- No cross-consumer correlation (no distributed trace IDs in MVP)
- No consumer metrics/dashboards in MVP (nsqadmin provides basic visibility)

---

## Assumptions Exposed & Resolved

| Assumption | Challenge | Resolution |
|---|---|---|
| Consumer is social-feed-specific | User wants general pattern for future consumers | ConsumerManager + MessageHandler interface |
| Payload is ad-hoc bytes | Publisher/consumer schema must match | JSON EventEnvelope with typed payload structs |
| Retry behavior unspecified | What happens on process failure? | Exponential backoff (5s→80s), discard after 5 attempts |
| Channel names are implicit | NSQ channels must be stable and unique | Descriptive slugs defined on each MessageHandler struct |
| Consumer is hard to test | Requires live NSQ in CI | Split process() from HandleMessage; unit test process() directly |
