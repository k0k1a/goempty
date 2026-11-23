package integration_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"goempty/pkg/kafka"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

// postEventData is the typed payload the router decodes for post events.
type postEventData struct {
	PostID int `json:"post_id"`
}

// postEventHandler adapts a typed callback to kafka.TypedHandler.
type postEventHandler struct {
	onEvent func(event kafka.MessageRawData, data postEventData) error
}

func (*postEventHandler) Name() string { return "postEvent" }

func (h *postEventHandler) Handle(_ context.Context, event kafka.MessageRawData, data postEventData) error {
	return h.onEvent(event, data)
}

// TestKafkaLifecycle uses a fresh topic and consumer group on an isolated broker.
func TestKafkaLifecycle(t *testing.T) {
	addrs := os.Getenv("GOEMPTY_TEST_KAFKA_BROKERS")
	if addrs == "" {
		t.Skip("GOEMPTY_TEST_KAFKA_BROKERS is not set")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Second)
	defer cancel()
	brokers := strings.Split(addrs, ",")
	topic := createKafkaTopic(t, ctx, brokers)
	const logicalTopic = "social-forum"
	cfg := kafka.Config{
		Brokers:      brokers,
		TopicMapping: map[string]string{logicalTopic: topic},
	}
	producer, err := kafka.NewProducer(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := producer.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})

	event := kafka.NewMessageRawData("forum.post.created", map[string]any{"post_id": 123})
	event.EventID = "first"
	event.EventTime = time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	if err := producer.Publish(ctx, logicalTopic, event); err != nil {
		t.Fatal(err)
	}
	event.EventID = "second"
	event.EventType = "forum.post.edited"
	done := make(chan error, 2)
	recordTime := time.Now().Truncate(time.Millisecond)
	err = producer.PublishAsync(ctx, logicalTopic, event, func(err error) { done <- err },
		kafka.WithKey([]byte("post:123")),
		kafka.WithHeader("trace", []byte("example")),
		kafka.WithTimestamp(recordTime),
	)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if err != nil {
			t.Fatalf("async delivery: %v", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	event.EventID = "third"
	if err := producer.Publish(ctx, logicalTopic, event); err != nil {
		t.Fatal(err)
	}
	verifyKafkaRecord(t, ctx, brokers, topic, recordTime)

	runConsumer := func(runCtx context.Context, consumerCfg kafka.ConsumerConfig, handler kafka.Handler) error {
		consumerCfg.Brokers = brokers
		consumerCfg.GroupName = topic
		consumerCfg.Topics = []string{logicalTopic}
		consumerCfg.RetryBackoff = 10 * time.Millisecond
		consumer, err := kafka.NewConsumer(consumerCfg, handler)
		if err != nil {
			t.Fatal(err)
		}
		if err := consumer.Start(runCtx); err != nil {
			t.Fatal(err)
		}
		<-consumer.Done()
		return consumer.Err()
	}

	// newRouter registers typed topic routes for both event types on a fresh router.
	newRouter := func(onEvent func(kafka.MessageRawData, postEventData) error) *kafka.Router {
		router := kafka.NewRouter()
		for _, eventType := range []string{"forum.post.created", "forum.post.edited"} {
			if err := kafka.Subscribe(router, logicalTopic, eventType, &postEventHandler{onEvent: onEvent}); err != nil {
				t.Fatal(err)
			}
		}
		return router
	}

	handlerErr := errors.New("handler failed")
	attempts := make(map[string]int)
	var firstRun, firstTopics []string
	runCtx, stop := context.WithCancel(ctx)
	defer stop()
	err = runConsumer(runCtx, kafka.ConsumerConfig{}, newRouter(func(got kafka.MessageRawData, data postEventData) error {
		attempts[got.EventID]++
		firstRun = append(firstRun, got.EventID)
		firstTopics = append(firstTopics, got.Topic)
		if data.PostID != 123 {
			t.Errorf("postID = %d, want 123", data.PostID)
		}
		if got.EventID == "second" && attempts["second"] == 1 {
			return handlerErr
		}
		if got.EventID == "third" {
			stop()
		}
		return nil
	}))
	if err != nil || strings.Join(firstRun, ",") != "first,second,second,third" ||
		attempts["second"] != 2 {
		t.Fatalf("first run: events = %v, attempts = %v, error = %v", firstRun, attempts, err)
	}
	for _, topic := range firstTopics {
		if topic != logicalTopic {
			t.Fatalf("logical topic = %q, want %q", topic, logicalTopic)
		}
	}

	for _, eventID := range []string{"fourth", "fifth"} {
		event.EventID = eventID
		if err := producer.Publish(ctx, logicalTopic, event); err != nil {
			t.Fatal(err)
		}
	}
	runCtx2, stop2 := context.WithCancel(ctx)
	defer stop2()
	var secondRun []string
	err = runConsumer(runCtx2, kafka.ConsumerConfig{MaxAttempts: 2}, newRouter(func(got kafka.MessageRawData, _ postEventData) error {
		secondRun = append(secondRun, got.EventID)
		if got.EventID == "fourth" {
			return handlerErr
		}
		stop2()
		return nil
	}))
	if err != nil || strings.Join(secondRun, ",") != "fourth,fourth,fifth" {
		t.Fatalf("second run: %v, %v", secondRun, err)
	}

	event.EventID = "sixth"
	if err := producer.Publish(ctx, logicalTopic, event); err != nil {
		t.Fatal(err)
	}
	finalCtx, stopFinal := context.WithCancel(ctx)
	defer stopFinal()
	var resumedAt string
	err = runConsumer(finalCtx, kafka.ConsumerConfig{}, newRouter(func(got kafka.MessageRawData, _ postEventData) error {
		resumedAt = got.EventID
		stopFinal()
		return nil
	}))
	if err != nil || resumedAt != "sixth" {
		t.Fatalf("resume after shutdown: event = %q, error = %v", resumedAt, err)
	}
	select {
	case err := <-done:
		t.Fatalf("async callback ran more than once: %v", err)
	default:
	}
}

func createKafkaTopic(t *testing.T, ctx context.Context, brokers []string) string {
	t.Helper()
	admin, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	topic := "goempty-test-" + uuid.NewString()
	req := kmsg.NewPtrCreateTopicsRequest()
	req.Topics = []kmsg.CreateTopicsRequestTopic{{
		Topic: topic, NumPartitions: 1, ReplicationFactor: 1,
	}}
	response, err := req.RequestWith(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Topics) != 1 || response.Topics[0].ErrorCode != 0 {
		t.Fatalf("create test topic: %+v", response.Topics)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req := kmsg.NewPtrDeleteTopicsRequest()
		req.Topics = []kmsg.DeleteTopicsRequestTopic{{Topic: &topic}}
		req.TopicNames = []string{topic}
		response, err := req.RequestWith(cleanupCtx, admin)
		if err != nil {
			t.Errorf("delete test topic: %v", err)
		} else if len(response.Topics) != 1 || response.Topics[0].ErrorCode != 0 {
			t.Errorf("delete test topic: %+v", response.Topics)
		}
	})
	return topic
}

func verifyKafkaRecord(t *testing.T, ctx context.Context, brokers []string, topic string, timestamp time.Time) {
	t.Helper()
	reader, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ConsumePartitions(
		map[string]map[int32]kgo.Offset{topic: {0: kgo.NewOffset().At(1)}},
	))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	fetches := reader.PollRecords(ctx, 1)
	if err := fetches.Err(); err != nil {
		t.Fatal(err)
	}
	records := fetches.Records()
	if len(records) != 1 {
		t.Fatalf("record count = %d", len(records))
	}
	record := records[0]
	if record.Topic != topic || string(record.Key) != "post:123" ||
		len(record.Headers) != 1 || record.Headers[0].Key != "trace" ||
		string(record.Headers[0].Value) != "example" || !record.Timestamp.Equal(timestamp) {
		t.Fatalf("async record = %+v", record)
	}
}
