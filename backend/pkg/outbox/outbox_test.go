package outbox

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

type fakeStore struct {
	pending   []Record
	published []int64
	failed    map[int64]string
}

func (f *fakeStore) FetchPending(context.Context, int) ([]Record, error) {
	out := f.pending
	f.pending = nil
	return out, nil
}
func (f *fakeStore) MarkPublished(_ context.Context, id int64) error {
	f.published = append(f.published, id)
	return nil
}
func (f *fakeStore) MarkFailed(_ context.Context, id int64, cause error) error {
	if f.failed == nil {
		f.failed = map[int64]string{}
	}
	f.failed[id] = cause.Error()
	return nil
}

type fakePublisher struct {
	failTopic string
	published []string
}

func (p *fakePublisher) PublishRaw(_ context.Context, topic, _ string, _ []byte, _ map[string]string) error {
	if topic == p.failTopic {
		return errors.New("broker down")
	}
	p.published = append(p.published, topic)
	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeLagReporter struct {
	reported []time.Duration
	called   bool
}

func (f *fakeLagReporter) ObserveOutboxLag(d time.Duration) {
	f.called = true
	f.reported = append(f.reported, d)
}

func TestRelayRunOnce(t *testing.T) {
	store := &fakeStore{pending: []Record{
		{ID: 1, Topic: "order.events.v1", Payload: []byte(`{}`)},
		{ID: 2, Topic: "order.events.v1", Payload: []byte(`{}`)},
	}}
	pub := &fakePublisher{}
	relay := NewRelay(store, pub, testLogger())

	n, err := relay.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("sent = %d, want 2", n)
	}
	if len(pub.published) != 2 || len(store.published) != 2 {
		t.Fatalf("published=%v marked=%v", pub.published, store.published)
	}
}

func TestRelayReportsLag(t *testing.T) {
	created := time.Now().Add(-3 * time.Second)
	store := &fakeStore{pending: []Record{
		{ID: 1, Topic: "order.events.v1", CreatedAt: created},
		{ID: 2, Topic: "order.events.v1", CreatedAt: time.Now()},
	}}
	lag := &fakeLagReporter{}
	relay := NewRelay(store, &fakePublisher{}, testLogger())
	relay.SetLagReporter(lag)

	if _, err := relay.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !lag.called || len(lag.reported) != 1 {
		t.Fatalf("lag dilaporkan = %v", lag.reported)
	}
	if lag.reported[0] < 2*time.Second {
		t.Fatalf("lag = %v, harusnya ~3s (usia record tertua)", lag.reported[0])
	}
}

func TestRelayReportsZeroLagWhenEmpty(t *testing.T) {
	store := &fakeStore{}
	lag := &fakeLagReporter{}
	relay := NewRelay(store, &fakePublisher{}, testLogger())
	relay.SetLagReporter(lag)

	if _, err := relay.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(lag.reported) != 1 || lag.reported[0] != 0 {
		t.Fatalf("lag = %v, want 0", lag.reported)
	}
}

func TestRelayMarksFailure(t *testing.T) {
	store := &fakeStore{pending: []Record{
		{ID: 1, Topic: "order.events.v1"},
		{ID: 2, Topic: "broken.topic"},
	}}
	pub := &fakePublisher{failTopic: "broken.topic"}
	relay := NewRelay(store, pub, testLogger())

	n, err := relay.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("sent = %d, want 1", n)
	}
	if _, ok := store.failed[2]; !ok {
		t.Fatal("record gagal tidak ditandai")
	}
	if len(store.published) != 1 || store.published[0] != 1 {
		t.Fatalf("published = %v", store.published)
	}
}
