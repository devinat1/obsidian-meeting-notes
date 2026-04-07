package kafkautil_test

import (
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/shared/kafkautil"
)

func TestNewProducer(t *testing.T) {
	p := kafkautil.NewProducer(kafkautil.NewProducerParams{
		Brokers: []string{"localhost:9092"},
		Topic:   "test.topic",
	})
	if p == nil {
		t.Fatal("expected non-nil producer")
	}
	if err := p.Close(); err != nil {
		t.Fatalf("unexpected error closing producer: %v", err)
	}
}
