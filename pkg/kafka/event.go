package kafka

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// eventSource is recorded as the source on messages created by
// NewMessageRawData.
// Change it to the actual service name when generating a project.
const eventSource = "app"

// MessageRawData is the JSON payload shared by producers and consumers.
// EventType identifies the message type within a topic. Topic, Partition and
// Offset are filled by the consumer: Topic is the logical topic from the
// consumer configuration, while Partition and Offset come from the fetched
// record. They are never serialized and are zero on messages built for
// publishing.
type MessageRawData struct {
	EventID     string    `json:"_event_id"`
	EventSource string    `json:"_event_source"`
	EventType   string    `json:"_event_type"`
	EventTime   time.Time `json:"_event_time"`
	Data        any       `json:"data"`

	Topic     string `json:"-"`
	Partition int32  `json:"-"`
	Offset    int64  `json:"-"`
}

// NewMessageRawData creates a message with a generated ID, the current UTC
// time, and this service as its source. Data is serialized with encoding/json
// when the message is published.
func NewMessageRawData(eventType string, data any) MessageRawData {
	return MessageRawData{
		EventID:     uuid.NewString(),
		EventSource: eventSource,
		EventType:   eventType,
		EventTime:   time.Now().UTC(),
		Data:        data,
	}
}

func (e MessageRawData) validate() error {
	switch {
	case strings.TrimSpace(e.EventID) == "":
		return errors.New("kafka: eventID is required")
	case strings.TrimSpace(e.EventSource) == "":
		return errors.New("kafka: eventSource is required")
	case strings.TrimSpace(e.EventType) == "":
		return errors.New("kafka: eventType is required")
	case e.EventTime.IsZero():
		return errors.New("kafka: eventTime is required")
	case e.Data == nil:
		return errors.New("kafka: data is required")
	}
	return nil
}

// decodeEvent returns the decoded event alongside a validation error so
// callers can log its identifiers; unmarshal failures return a zero event.
func decodeEvent(value []byte) (MessageRawData, error) {
	var event MessageRawData
	if err := json.Unmarshal(value, &event); err != nil {
		return MessageRawData{}, fmt.Errorf("kafka: decode event: %w", err)
	}
	if err := event.validate(); err != nil {
		return event, err
	}
	return event, nil
}
