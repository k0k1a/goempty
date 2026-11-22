package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

const deliveryTimeout = 10 * time.Second

// Producer is a shared publisher. Stop submitting new events before closing it.
type Producer interface {
	Publish(ctx context.Context, topic string, event MessageRawData, opts ...PublishOption) error
	// PublishAsync returns local validation errors directly. If it returns nil,
	// done is called once with the delivery result, including buffer-full errors.
	// Keep ctx active until delivery. done can run concurrently with callers and
	// must return promptly without calling methods on this producer.
	PublishAsync(ctx context.Context, topic string, event MessageRawData, done func(error), opts ...PublishOption) error
	Close(ctx context.Context) error
}

type producer struct {
	topicMapping map[string]string
	client       *kgo.Client
	closing      atomic.Bool
	closeOnce    sync.Once
	closeErr     error
}

// NewProducer validates the broker list and creates a publisher. Broker
// connectivity is checked when publishing.
func NewProducer(ctx context.Context, cfg Config) (Producer, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	brokers, err := cfg.brokers()
	if err != nil {
		return nil, err
	}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.RecordDeliveryTimeout(deliveryTimeout),
		kgo.MaxBufferedBytes(32<<20),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka: create producer: %w", err)
	}
	return &producer{topicMapping: cfg.TopicMapping, client: client}, nil
}

func (p *producer) Publish(ctx context.Context, topic string, event MessageRawData, opts ...PublishOption) error {
	record, err := p.prepare(ctx, topic, event, opts)
	if err != nil {
		return err
	}
	return p.client.ProduceSync(ctx, record).FirstErr()
}

func (p *producer) PublishAsync(ctx context.Context, topic string, event MessageRawData, done func(error), opts ...PublishOption) error {
	if done == nil {
		return errors.New("kafka: async publish callback is required")
	}
	record, err := p.prepare(ctx, topic, event, opts)
	if err != nil {
		return err
	}
	p.client.TryProduce(ctx, record, func(_ *kgo.Record, err error) { done(err) })
	return nil
}

func (p *producer) prepare(ctx context.Context, topic string, event MessageRawData, opts []PublishOption) (*kgo.Record, error) {
	if p.closing.Load() {
		return nil, errors.New("kafka: producer is closed")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(topic) == "" {
		return nil, errors.New("kafka: topic is required")
	}
	return p.record(topic, event, opts)
}

func (p *producer) Close(ctx context.Context) error {
	p.closeOnce.Do(func() {
		p.closing.Store(true)
		flushCtx, cancel := context.WithTimeout(ctx, deliveryTimeout)
		defer cancel()
		p.closeErr = p.client.Flush(flushCtx)
		p.client.Close()
	})
	return p.closeErr
}

// PublishOption sets an optional Kafka record field.
type PublishOption func(*publishOptions)

type publishOptions struct {
	key       []byte
	headers   []kgo.RecordHeader
	timestamp time.Time
}

// WithKey supplies the Kafka partition key.
func WithKey(key []byte) PublishOption {
	key = slices.Clone(key)
	return func(opts *publishOptions) { opts.key = key }
}

// WithHeader adds a Kafka record header. Multiple headers may share a name.
func WithHeader(name string, value []byte) PublishOption {
	header := kgo.RecordHeader{Key: name, Value: slices.Clone(value)}
	return func(opts *publishOptions) { opts.headers = append(opts.headers, header) }
}

// WithTimestamp sets the Kafka record timestamp separately from EventTime.
func WithTimestamp(timestamp time.Time) PublishOption {
	return func(opts *publishOptions) { opts.timestamp = timestamp }
}

func (p *producer) record(logicalTopic string, event MessageRawData, opts []PublishOption) (*kgo.Record, error) {
	topic := p.actualTopic(logicalTopic)
	if err := event.validate(); err != nil {
		return nil, err
	}
	event.EventTime = event.EventTime.UTC()
	value, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("kafka: encode event: %w", err)
	}
	var options publishOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	return &kgo.Record{
		Topic:     topic,
		Value:     value,
		Key:       options.key,
		Headers:   options.headers,
		Timestamp: options.timestamp,
	}, nil
}

// actualTopic returns the configured topic for a logical name, or the name
// itself when no mapping is configured.
func (p *producer) actualTopic(logicalTopic string) string {
	if topic := p.topicMapping[logicalTopic]; strings.TrimSpace(topic) != "" {
		return topic
	}
	return logicalTopic
}
