package kafka

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"goempty/pkg/log"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
)

const (
	maxPollRecords      = 32
	commitTimeout       = 5 * time.Second
	leaveTimeout        = 5 * time.Second
	defaultMaxAttempts  = 3
	defaultRetryBackoff = time.Second
	maxRetryBackoff     = 30 * time.Second
)

// Handler processes one event. Return nil to commit the record and continue.
// Return an error to retry the same record after a backoff; after MaxAttempts
// the record is logged and skipped. Dispatch by EventType and respond to
// cancellation promptly.
type Handler func(ctx context.Context, event MessageRawData) error

// Consumer runs one consumer group in a single goroutine. Create it with
// NewConsumer, call Start once, and call Stop to shut it down. A stopped
// consumer cannot be restarted.
type Consumer struct {
	brokers      []string
	group        string
	topics       []string
	topicMapping map[string]string
	handler      Handler
	maxAttempts  int
	retryBackoff time.Duration

	mu      sync.Mutex
	started bool
	stopped bool
	cancel  context.CancelFunc
	done    chan struct{}
	err     error
}

// NewConsumer validates the configuration without opening connections or
// starting goroutines. Topics are resolved when the consumer starts.
func NewConsumer(consumerCfg ConsumerConfig, handler Handler) (*Consumer, error) {
	brokers, err := consumerCfg.brokers()
	if err != nil {
		return nil, err
	}
	if handler == nil {
		return nil, errors.New("kafka: consumer handler is required")
	}
	group := strings.TrimSpace(consumerCfg.GroupName)
	if group == "" {
		return nil, errors.New("kafka: consumer group name is required")
	}
	if consumerCfg.MaxAttempts < 0 {
		return nil, errors.New("kafka: max attempts must not be negative")
	}
	if consumerCfg.RetryBackoff < 0 {
		return nil, errors.New("kafka: retry backoff must not be negative")
	}
	maxAttempts := consumerCfg.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = defaultMaxAttempts
	}
	retryBackoff := consumerCfg.RetryBackoff
	if retryBackoff == 0 {
		retryBackoff = defaultRetryBackoff
	}
	return &Consumer{
		brokers:      brokers,
		group:        group,
		topics:       consumerCfg.Topics,
		topicMapping: consumerCfg.TopicMapping,
		handler:      handler,
		maxAttempts:  maxAttempts,
		retryBackoff: retryBackoff,
	}, nil
}

// actualTopics resolves the configured logical topics for the Kafka client.
func (c *Consumer) actualTopics() []string {
	topics := make([]string, 0, len(c.topics))
	for _, logicalTopic := range c.topics {
		topics = append(topics, c.topicMapping[logicalTopic])
	}
	return topics
}

// logicalTopic returns the configured logical name for an actual topic. The
// configuration is expected to map every topic, so the first match is final.
func (c *Consumer) logicalTopic(actual string) string {
	for _, logicalTopic := range c.topics {
		if c.topicMapping[logicalTopic] == actual {
			return logicalTopic
		}
	}
	return actual
}

// Start creates the client and launches consumption in the background. ctx is
// the parent of the run context; canceling it stops consumption. Start does not
// wait for broker connectivity. Call it at most once per consumer.
func (c *Consumer) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch {
	case c.stopped:
		return errors.New("kafka: cannot start a stopped consumer")
	case c.started:
		return errors.New("kafka: consumer is already started")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("kafka: start consumer: %w", err)
	}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(c.brokers...),
		kgo.ConsumerGroup(c.group),
		kgo.ConsumeTopics(c.actualTopics()...),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll(),
	)
	if err != nil {
		return fmt.Errorf("kafka: create consumer: %w", err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.done = make(chan struct{})
	c.started = true
	go func() {
		runErr := c.run(runCtx, client)
		c.mu.Lock()
		c.err = runErr
		c.mu.Unlock()
		close(c.done)
	}()
	return nil
}

// Done is closed when the run goroutine exits. It returns nil before Start.
func (c *Consumer) Done() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.done
}

// Err returns the run error after Done is closed. A normal shutdown and
// skipped records are not errors; polling, commit and leave-group failures are.
func (c *Consumer) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// Stop cancels consumption and waits for the run goroutine to exit. It is safe
// to call before Start, repeatedly, and again after a timeout: every call waits
// for the same run, bounded by its own ctx. A timeout does not force the handler
// to return; handlers must respond to cancellation.
func (c *Consumer) Stop(ctx context.Context) error {
	c.mu.Lock()
	c.stopped = true
	done, cancel := c.done, c.cancel
	c.mu.Unlock()
	if done == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
	case <-ctx.Done():
		return fmt.Errorf("kafka: stop consumer: %w", ctx.Err())
	}
	return c.Err()
}

func (c *Consumer) run(ctx context.Context, client *kgo.Client) (runErr error) {
	defer func() {
		leaveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), leaveTimeout)
		defer cancel()
		if err := client.LeaveGroupContext(leaveCtx); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("kafka: leave consumer group: %w", err))
		}
		client.Close()
	}()

	for ctx.Err() == nil {
		fetches := client.PollRecords(ctx, maxPollRecords)
		if err := c.consumeBatch(ctx, client, fetches); err != nil {
			return err
		}
	}
	return nil
}

func (c *Consumer) consumeBatch(ctx context.Context, client *kgo.Client, fetches kgo.Fetches) error {
	// Allow rebalancing only after the batch is processed so offsets are not
	// committed for partitions that have been revoked.
	defer client.AllowRebalance()
	if ctx.Err() != nil {
		return nil
	}
	if err := fetches.Err(); err != nil {
		// Shutdown can cancel the run context between the check above and here.
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("kafka: poll records: %w", err)
	}

	var processed []*kgo.Record
	for _, record := range fetches.Records() {
		if ctx.Err() != nil {
			break
		}
		if !c.processRecord(ctx, record) {
			break
		}
		processed = append(processed, record)
	}
	return c.commit(ctx, client, processed)
}

// processRecord returns false only when shutdown interrupts processing; the
// record must then stay uncommitted. Handler failures never abort the consumer:
// they are retried and eventually skipped.
func (c *Consumer) processRecord(ctx context.Context, record *kgo.Record) bool {
	event, err := decodeEvent(record.Value)
	event = c.withRecordMetadata(event, record)
	if err != nil {
		// A malformed record cannot be fixed by retrying it.
		logRejected(ctx, event, err, 0)
		return true
	}
	for attempt := 1; ; attempt++ {
		err = c.callHandler(ctx, event)
		if err == nil {
			return true
		}
		if ctx.Err() != nil {
			return false
		}
		if attempt >= c.maxAttempts {
			logRejected(ctx, event, err, attempt)
			return true
		}
		backoff := c.backoffFor(attempt)
		log.Warn(ctx, "Kafka message retry scheduled",
			zap.String("topic", event.Topic),
			zap.Int32("partition", event.Partition),
			zap.Int64("offset", event.Offset),
			zap.String("event_id", event.EventID),
			zap.String("event_type", event.EventType),
			zap.Int("attempt", attempt),
			zap.Duration("backoff", backoff),
			zap.Error(err),
		)
		if !waitRetry(ctx, backoff) {
			return false
		}
	}
}

// callHandler converts a handler panic into an ordinary error so one bad
// record cannot terminate the process.
func (c *Consumer) callHandler(ctx context.Context, event MessageRawData) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Error(ctx, "Kafka handler panic", zap.Any("panic", recovered), zap.ByteString("stack", debug.Stack()))
			err = fmt.Errorf("kafka: handler panic: %v", recovered)
		}
	}()
	return c.handler(ctx, event)
}

// backoffFor returns the wait after the given failed attempt: the configured
// base doubled per attempt, capped at maxRetryBackoff.
func (c *Consumer) backoffFor(attempt int) time.Duration {
	backoff := c.retryBackoff
	for i := 1; i < attempt && backoff < maxRetryBackoff; i++ {
		backoff *= 2
	}
	if backoff > maxRetryBackoff {
		backoff = maxRetryBackoff
	}
	return backoff
}

func waitRetry(ctx context.Context, backoff time.Duration) bool {
	timer := time.NewTimer(backoff)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// withRecordMetadata fills consumer-side metadata. Topic is the logical name so
// handlers and dispatch code stay independent of environment topic names.
func (c *Consumer) withRecordMetadata(event MessageRawData, record *kgo.Record) MessageRawData {
	event.Topic = c.logicalTopic(record.Topic)
	event.Partition = record.Partition
	event.Offset = record.Offset
	return event
}

func logRejected(ctx context.Context, event MessageRawData, err error, attempts int) {
	log.Error(ctx, "Kafka message rejected",
		zap.String("topic", event.Topic),
		zap.Int32("partition", event.Partition),
		zap.Int64("offset", event.Offset),
		zap.String("event_id", event.EventID),
		zap.String("event_type", event.EventType),
		zap.Int("attempts", attempts),
		zap.Error(err),
	)
}

// commit persists processed offsets even when the run context was canceled.
func (c *Consumer) commit(ctx context.Context, client *kgo.Client, records []*kgo.Record) error {
	if len(records) == 0 {
		return nil
	}
	commitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), commitTimeout)
	defer cancel()
	if err := client.CommitRecords(commitCtx, records...); err != nil {
		return fmt.Errorf("kafka: commit offsets: %w", err)
	}
	return nil
}
