package kafka

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
)

func noopHandler(context.Context, MessageRawData) error { return nil }

func testEvent() MessageRawData {
	return MessageRawData{
		EventID: "event-1", EventSource: eventSource, EventType: "forum.post.created",
		EventTime: time.Date(2026, 11, 20, 8, 0, 0, 0, time.FixedZone("UTC+8", 8*3600)),
		Data:      map[string]any{"post_id": 123},
	}
}

func TestNewMessageRawData(t *testing.T) {
	data := map[string]any{"post_id": 123}
	event := NewMessageRawData("forum.post.created", data)
	if event.EventSource != eventSource || event.EventType != "forum.post.created" ||
		event.EventTime.IsZero() || event.EventTime.Location() != time.UTC ||
		!reflect.DeepEqual(event.Data, data) {
		t.Fatalf("event = %+v", event)
	}
	if _, err := uuid.Parse(event.EventID); err != nil {
		t.Fatalf("eventID = %q: %v", event.EventID, err)
	}
	if err := event.validate(); err != nil {
		t.Fatal(err)
	}
}

func TestPublishRecord(t *testing.T) {
	key := []byte("post:123")
	trace := []byte("trace-id")
	keyOption := WithKey(key)
	headerOption := WithHeader("traceparent", trace)
	key[0], trace[0] = 'X', 'X'
	event := testEvent()
	p := &producer{topicMapping: map[string]string{"social-forum": "forum-dev"}}
	record, err := p.record("social-forum", event, []PublishOption{
		keyOption, headerOption, WithHeader("traceparent", []byte("other")), WithTimestamp(event.EventTime),
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Topic != "forum-dev" || string(record.Key) != "post:123" || len(record.Headers) != 2 ||
		string(record.Headers[0].Value) != "trace-id" || !record.Timestamp.Equal(event.EventTime) {
		t.Fatalf("record = %+v", record)
	}
	if !bytes.Contains(record.Value, []byte(`"_event_time":"2026-11-20T00:00:00Z"`)) {
		t.Fatalf("value = %s", record.Value)
	}
	decoded, err := decodeEvent(record.Value)
	if err != nil || decoded.EventID != event.EventID || decoded.EventType != event.EventType ||
		!bytes.Equal(decoded.rawData, []byte(`{"post_id":123}`)) {
		t.Fatalf("decoded = %+v, error = %v", decoded, err)
	}
}

func TestInvalidEventAndUnknownTopics(t *testing.T) {
	cfg := Config{
		Brokers:      []string{"127.0.0.1:9092"},
		TopicMapping: map[string]string{"social-forum": "forum-test"},
	}
	c, err := NewConsumer(ConsumerConfig{
		Brokers: cfg.Brokers, GroupName: "forum", Topics: []string{"missing"},
		TopicMapping: map[string]string{"missing": "forum-missing"},
	}, HandlerFunc(noopHandler))
	if err != nil || !reflect.DeepEqual(c.actualTopics(), []string{"forum-missing"}) {
		t.Fatalf("mapped topic = %v, error = %v", c.actualTopics(), err)
	}
	p, err := NewProducer(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close(context.Background()) })
	if err := p.PublishSync(t.Context(), "", testEvent()); err == nil {
		t.Fatal("publishing to an empty topic succeeded")
	}
	record, err := p.(*producer).record("missing", testEvent(), nil)
	if err != nil || record.Topic != "missing" {
		t.Fatalf("unmapped topic = %q, error = %v", record.Topic, err)
	}
	event := testEvent()
	event.EventType = ""
	called := false
	if err := p.PublishAsync(t.Context(), "social-forum", event, func(error) { called = true }); err == nil || called {
		t.Fatalf("invalid async publish: error = %v, callback = %t", err, called)
	}
	event = testEvent()
	event.EventSource = ""
	if err := p.PublishAsync(t.Context(), "social-forum", event, func(error) {}); err == nil {
		t.Fatal("publishing without an eventSource succeeded")
	}
	if _, err := decodeEvent([]byte(`{"_event_id":"1","_event_source":"app","_event_type":"created","_event_time":"2026-11-20T08:00:00Z","data":{}}`)); err != nil {
		t.Fatalf("valid event was rejected: %v", err)
	}
	if _, err := decodeEvent([]byte(`{"_event_id":"1","_event_source":"app","_event_type":"created","_event_time":"2026-11-20T08:00:00Z"}`)); err == nil {
		t.Fatal("event without data was accepted")
	}
	if _, err := decodeEvent([]byte(`{"_event_id":"1","_event_source":"app","_event_type":"created","_event_time":"2026-11-20T08:00:00Z","data":null}`)); err == nil {
		t.Fatal("event with null data was accepted")
	}
}

func TestProducerTopicFallback(t *testing.T) {
	p := &producer{topicMapping: map[string]string{
		"social-forum": "forum-dev",
		"empty-value":  " ",
	}}
	for logicalTopic, want := range map[string]string{
		"social-forum": "forum-dev",
		"empty-value":  "empty-value",
		"unmapped":     "unmapped",
	} {
		if got := p.actualTopic(logicalTopic); got != want {
			t.Errorf("actualTopic(%q) = %q, want %q", logicalTopic, got, want)
		}
	}
}

func TestTopicMappingResolution(t *testing.T) {
	const topic = "forum.events"
	cfg := Config{
		Brokers:      []string{"127.0.0.1:9092"},
		TopicMapping: map[string]string{topic: "Forum.Events.Dev"},
	}
	p, err := NewProducer(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close(context.Background()) })
	c, err := NewConsumer(ConsumerConfig{
		Brokers: cfg.Brokers, GroupName: "forum", Topics: []string{topic},
		TopicMapping: cfg.TopicMapping,
	}, HandlerFunc(noopHandler))
	if err != nil {
		t.Fatal(err)
	}
	record, err := p.(*producer).prepare(t.Context(), topic, testEvent(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if record.Topic != "Forum.Events.Dev" || c.actualTopics()[0] != record.Topic {
		t.Fatalf("producer topic = %q, consumer topics = %v", record.Topic, c.actualTopics())
	}
}

func TestNewConsumerValidation(t *testing.T) {
	cfg := Config{
		Brokers:      []string{"127.0.0.1:9092"},
		TopicMapping: map[string]string{"social-forum": "forum-test"},
	}
	for name, consumerCfg := range map[string]ConsumerConfig{
		"missing group":         {Topics: []string{"social-forum"}},
		"blank group":           {GroupName: " ", Topics: []string{"social-forum"}},
		"negative max attempts": {GroupName: "forum", Topics: []string{"social-forum"}, MaxAttempts: -1},
		"negative backoff":      {GroupName: "forum", Topics: []string{"social-forum"}, RetryBackoff: -time.Second},
	} {
		t.Run(name, func(t *testing.T) {
			consumerCfg.Brokers = cfg.Brokers
			if _, err := NewConsumer(consumerCfg, HandlerFunc(noopHandler)); err == nil {
				t.Fatal("invalid consumer configuration succeeded")
			}
		})
	}
	if _, err := NewConsumer(ConsumerConfig{
		Brokers: cfg.Brokers, GroupName: "forum", Topics: []string{"social-forum"},
	}, nil); err == nil {
		t.Fatal("nil handler succeeded")
	}
	if _, err := NewConsumer(ConsumerConfig{
		GroupName: "forum", Topics: []string{"social-forum"},
	}, HandlerFunc(noopHandler)); err == nil {
		t.Fatal("missing brokers succeeded")
	}
}

func TestConsumerTopicMapping(t *testing.T) {
	c, err := NewConsumer(ConsumerConfig{
		Brokers:   []string{"127.0.0.1:9092"},
		GroupName: "forum",
		Topics:    []string{"social-forum", "audit"},
		TopicMapping: map[string]string{
			"social-forum": "forum-override",
			"audit":        "audit-shared",
		},
	}, HandlerFunc(noopHandler))
	if err != nil {
		t.Fatal(err)
	}
	wantTopics := []string{"forum-override", "audit-shared"}
	if !reflect.DeepEqual(c.actualTopics(), wantTopics) {
		t.Fatalf("topics = %v, want %v", c.actualTopics(), wantTopics)
	}
	wantLogical := []string{"social-forum", "audit"}
	if !reflect.DeepEqual(c.topics, wantLogical) {
		t.Fatalf("configured topics = %v, want %v", c.topics, wantLogical)
	}
}

func TestConsumerLifecycle(t *testing.T) {
	cfg := Config{Brokers: []string{"127.0.0.1:1"}, TopicMapping: map[string]string{"social-forum": "forum-test"}}
	newConsumer := func() *Consumer {
		c, err := NewConsumer(ConsumerConfig{
			Brokers: cfg.Brokers, GroupName: "forum", Topics: []string{"social-forum"},
			TopicMapping: cfg.TopicMapping,
		}, HandlerFunc(noopHandler))
		if err != nil {
			t.Fatal(err)
		}
		return c
	}

	neverStarted := newConsumer()
	if neverStarted.Done() != nil {
		t.Fatal("Done is non-nil before Start")
	}
	if err := neverStarted.Stop(t.Context()); err != nil {
		t.Fatalf("Stop before Start = %v", err)
	}
	if err := neverStarted.Start(t.Context()); err == nil {
		t.Fatal("Start after Stop succeeded")
	}

	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := newConsumer().Start(canceled); err == nil {
		t.Fatal("Start with a canceled context succeeded")
	}

	c := newConsumer()
	if err := c.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := c.Start(t.Context()); err == nil {
		t.Fatal("second Start succeeded")
	}
	if c.Done() == nil {
		t.Fatal("Done is nil after Start")
	}
	stopCtx, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	if err := c.Stop(stopCtx); err != nil {
		t.Fatalf("Stop = %v", err)
	}
	select {
	case <-c.Done():
	default:
		t.Fatal("Done is not closed after Stop")
	}
	if err := c.Err(); err != nil {
		t.Fatalf("Err = %v", err)
	}
}

func TestConsumerRetryDefaults(t *testing.T) {
	c := newTestConsumer(t, ConsumerConfig{}, HandlerFunc(noopHandler))
	if c.maxAttempts != defaultMaxAttempts || c.retryBackoff != defaultRetryBackoff {
		t.Fatalf("defaults = %d/%v", c.maxAttempts, c.retryBackoff)
	}
	if c.backoffFor(1) != defaultRetryBackoff || c.backoffFor(2) != 2*defaultRetryBackoff ||
		c.backoffFor(50) != maxRetryBackoff {
		t.Fatalf("backoff schedule = %v/%v/%v", c.backoffFor(1), c.backoffFor(2), c.backoffFor(50))
	}
}

func TestConsumerRetriesUntilSuccess(t *testing.T) {
	var attempts int
	c := newTestConsumer(t, ConsumerConfig{MaxAttempts: 3, RetryBackoff: time.Millisecond},
		HandlerFunc(func(context.Context, MessageRawData) error {
			attempts++
			if attempts < 3 {
				return errors.New("transient")
			}
			return nil
		}))
	if !c.processRecord(t.Context(), testRecord(testEvent())) {
		t.Fatal("record was not completed")
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestConsumerSkipsAfterAttempts(t *testing.T) {
	var attempts int
	c := newTestConsumer(t, ConsumerConfig{MaxAttempts: 2, RetryBackoff: time.Millisecond},
		HandlerFunc(func(context.Context, MessageRawData) error {
			attempts++
			return errors.New("permanent")
		}))
	if !c.processRecord(t.Context(), testRecord(testEvent())) {
		t.Fatal("record was not completed")
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestConsumerRecoversHandlerPanic(t *testing.T) {
	var attempts int
	c := newTestConsumer(t, ConsumerConfig{MaxAttempts: 2, RetryBackoff: time.Millisecond},
		HandlerFunc(func(context.Context, MessageRawData) error {
			attempts++
			panic("boom")
		}))
	if !c.processRecord(t.Context(), testRecord(testEvent())) {
		t.Fatal("record was not completed")
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestConsumerSkipsMalformedRecord(t *testing.T) {
	called := false
	c := newTestConsumer(t, ConsumerConfig{}, HandlerFunc(func(context.Context, MessageRawData) error {
		called = true
		return nil
	}))
	record := &kgo.Record{Topic: "forum-test", Value: []byte("{")}
	if !c.processRecord(t.Context(), record) {
		t.Fatal("malformed record was not completed")
	}
	if called {
		t.Fatal("handler was called for a malformed record")
	}
}

func TestConsumerRetryStopsOnShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var attempts int
	c := newTestConsumer(t, ConsumerConfig{MaxAttempts: 5, RetryBackoff: time.Hour},
		HandlerFunc(func(context.Context, MessageRawData) error {
			attempts++
			cancel()
			return errors.New("failed")
		}))
	if c.processRecord(ctx, testRecord(testEvent())) {
		t.Fatal("interrupted record was committed")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func newTestConsumer(t *testing.T, consumerCfg ConsumerConfig, handler Handler) *Consumer {
	t.Helper()
	consumerCfg.Brokers = []string{"127.0.0.1:1"}
	consumerCfg.GroupName = "forum"
	consumerCfg.Topics = []string{"social-forum"}
	consumerCfg.TopicMapping = map[string]string{"social-forum": "forum-test"}
	c, err := NewConsumer(consumerCfg, handler)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func testRecord(event MessageRawData) *kgo.Record {
	value, err := json.Marshal(event)
	if err != nil {
		panic(err)
	}
	return &kgo.Record{Topic: "forum-test", Partition: 3, Offset: 42, Value: value}
}

func TestConsumerPopulatesRecordMetadata(t *testing.T) {
	var got MessageRawData
	c := newTestConsumer(t, ConsumerConfig{}, HandlerFunc(func(_ context.Context, event MessageRawData) error {
		got = event
		return nil
	}))
	record := testRecord(testEvent())
	if !c.processRecord(t.Context(), record) {
		t.Fatal("record was not completed")
	}
	if got.Topic != "social-forum" || got.Partition != record.Partition || got.Offset != record.Offset {
		t.Fatalf("metadata = %q/%d/%d", got.Topic, got.Partition, got.Offset)
	}
}

func TestMessageMetadataIsNotSerialized(t *testing.T) {
	event := testEvent()
	event.Topic = "forum-test"
	event.Partition = 3
	event.Offset = 42
	value, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(value, []byte("forum-test")) {
		t.Fatalf("metadata was serialized: %s", value)
	}
	var fields map[string]any
	if err := json.Unmarshal(value, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"Topic", "Partition", "Offset", "topic", "partition", "offset"} {
		if _, exists := fields[key]; exists {
			t.Fatalf("field %q was serialized: %s", key, value)
		}
	}
}

// recordingHandler captures the events and decoded data a router passes to a
// typed handler. Handler invocations are serialized by the consumer loop, so
// no locking is needed.
type recordingHandler[Data any] struct {
	name   string
	err    error
	events []MessageRawData
	data   []Data
}

func (h *recordingHandler[Data]) Name() string { return h.name }

func (h *recordingHandler[Data]) Handle(_ context.Context, event MessageRawData, data Data) error {
	h.events = append(h.events, event)
	h.data = append(h.data, data)
	return h.err
}

func TestRouterDispatchesByEventType(t *testing.T) {
	created := &recordingHandler[struct {
		PostID int `json:"post_id"`
	}]{name: "created"}
	edited := &recordingHandler[string]{name: "edited"}
	router := NewRouter()
	if err := Subscribe(router, "social-forum", "forum.post.created", created); err != nil {
		t.Fatal(err)
	}
	if err := Subscribe(router, "social-forum", "forum.post.edited", edited); err != nil {
		t.Fatal(err)
	}
	c := newTestConsumer(t, ConsumerConfig{}, router)

	if !c.processRecord(t.Context(), testRecord(testEvent())) {
		t.Fatal("record was not completed")
	}
	if len(created.data) != 1 || created.data[0].PostID != 123 {
		t.Fatalf("created data = %+v", created.data)
	}
	if len(created.events) != 1 || created.events[0].Topic != "social-forum" {
		t.Fatalf("created event = %+v", created.events)
	}
	if len(edited.data) != 0 {
		t.Fatalf("edited handler was called: %+v", edited.data)
	}
}

func TestRouterSkipsUnregisteredEventType(t *testing.T) {
	created := &recordingHandler[json.RawMessage]{name: "created"}
	router := NewRouter()
	if err := Subscribe(router, "social-forum", "forum.post.created", created); err != nil {
		t.Fatal(err)
	}
	c := newTestConsumer(t, ConsumerConfig{}, router)

	event := testEvent()
	event.EventType = "forum.post.unknown"
	if !c.processRecord(t.Context(), testRecord(event)) {
		t.Fatal("record was not completed")
	}
	if len(created.data) != 0 {
		t.Fatal("handler was called for an unregistered event type")
	}
}

func TestRouterSkipsMismatchedData(t *testing.T) {
	created := &recordingHandler[struct {
		PostID int `json:"post_id"`
	}]{name: "created"}
	router := NewRouter()
	if err := Subscribe(router, "social-forum", "forum.post.created", created); err != nil {
		t.Fatal(err)
	}
	c := newTestConsumer(t, ConsumerConfig{}, router)

	event := testEvent()
	event.Data = "not-an-object"
	if !c.processRecord(t.Context(), testRecord(event)) {
		t.Fatal("record was not completed")
	}
	if len(created.data) != 0 {
		t.Fatal("handler was called for mismatched data")
	}
}

func TestRouterRetriesHandlerErrors(t *testing.T) {
	created := &recordingHandler[struct {
		PostID int `json:"post_id"`
	}]{name: "created", err: errors.New("permanent")}
	router := NewRouter()
	if err := Subscribe(router, "social-forum", "forum.post.created", created); err != nil {
		t.Fatal(err)
	}
	c := newTestConsumer(t, ConsumerConfig{MaxAttempts: 2, RetryBackoff: time.Millisecond}, router)

	if !c.processRecord(t.Context(), testRecord(testEvent())) {
		t.Fatal("record was not completed")
	}
	if len(created.data) != 2 {
		t.Fatalf("attempts = %d, want 2", len(created.data))
	}
}

func TestSubscribeValidation(t *testing.T) {
	router := NewRouter()
	created := &recordingHandler[struct {
		PostID int `json:"post_id"`
	}]{name: "created"}
	if err := Subscribe(nil, "social-forum", "forum.post.created", created); err == nil {
		t.Fatal("nil router succeeded")
	}
	if err := Subscribe(router, "", "forum.post.created", created); err == nil {
		t.Fatal("empty topic succeeded")
	}
	if err := Subscribe(router, " social-forum", "forum.post.created", created); err == nil {
		t.Fatal("padded topic succeeded")
	}
	if err := Subscribe(router, "social-forum", "", created); err == nil {
		t.Fatal("empty event type succeeded")
	}
	if err := Subscribe(router, "social-forum", " forum.post.created", created); err == nil {
		t.Fatal("padded event type succeeded")
	}
	if err := Subscribe[struct{ PostID int }](router, "social-forum", "forum.post.created", nil); err == nil {
		t.Fatal("nil handler succeeded")
	}
	if err := Subscribe(router, "social-forum", "forum.post.created", &recordingHandler[struct{ PostID int }]{name: " "}); err == nil {
		t.Fatal("blank handler name succeeded")
	}
	if err := Subscribe(router, "social-forum", "forum.post.created", created); err != nil {
		t.Fatal(err)
	}
	err := Subscribe(router, "social-forum", "forum.post.created", &recordingHandler[struct{ PostID int }]{name: "other"})
	if err == nil || !errors.Is(err, ErrDuplicateRoute) {
		t.Fatalf("duplicate route = %v", err)
	}
	if err := Subscribe(router, "audit", "forum.post.created", &recordingHandler[struct{ PostID int }]{name: "audit"}); err != nil {
		t.Fatalf("same event type on another topic failed: %v", err)
	}
}

func newTwoTopicConsumer(t *testing.T, handler Handler) *Consumer {
	t.Helper()
	c, err := NewConsumer(ConsumerConfig{
		Brokers:   []string{"127.0.0.1:1"},
		GroupName: "forum",
		Topics:    []string{"social-forum", "audit"},
		TopicMapping: map[string]string{
			"social-forum": "forum-test",
			"audit":        "audit-shared",
		},
	}, handler)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestRouterDispatchesByTopicAndEventType(t *testing.T) {
	social := &recordingHandler[struct {
		PostID int `json:"post_id"`
	}]{name: "social"}
	audit := &recordingHandler[string]{name: "audit"}
	router := NewRouter()
	if err := Subscribe(router, "social-forum", "forum.post.created", social); err != nil {
		t.Fatal(err)
	}
	if err := Subscribe(router, "audit", "forum.post.created", audit); err != nil {
		t.Fatal(err)
	}
	c := newTwoTopicConsumer(t, router)

	socialRecord := testRecord(testEvent())
	if !c.processRecord(t.Context(), socialRecord) {
		t.Fatal("social record was not completed")
	}
	if len(social.data) != 1 || social.data[0].PostID != 123 {
		t.Fatalf("social data = %+v", social.data)
	}

	auditEvent := testEvent()
	auditEvent.Data = "audited"
	auditRecord := testRecord(auditEvent)
	auditRecord.Topic = "audit-shared"
	if !c.processRecord(t.Context(), auditRecord) {
		t.Fatal("audit record was not completed")
	}
	if len(audit.data) != 1 || audit.data[0] != "audited" {
		t.Fatalf("audit data = %v", audit.data)
	}
	if len(social.data) != 1 {
		t.Fatalf("social handler was called for the audit topic: %+v", social.data)
	}
}

func TestRouterSkipsUnregisteredTopicCombination(t *testing.T) {
	social := &recordingHandler[struct {
		PostID int `json:"post_id"`
	}]{name: "social"}
	router := NewRouter()
	if err := Subscribe(router, "social-forum", "forum.post.created", social); err != nil {
		t.Fatal(err)
	}
	c := newTwoTopicConsumer(t, router)

	auditRecord := testRecord(testEvent())
	auditRecord.Topic = "audit-shared"
	if !c.processRecord(t.Context(), auditRecord) {
		t.Fatal("record was not completed")
	}
	if len(social.data) != 0 {
		t.Fatal("handler was called for an unregistered topic combination")
	}
}
