package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHub_ConcurrentSubscribeUnsubscribe(t *testing.T) {
	h := NewHub(8)
	defer h.Shutdown()

	var wg sync.WaitGroup
	iterations := 100

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			events, unsub := h.Subscribe(ctx)
			unsub()
			select {
			case <-events:
			default:
			}
		}(i)
	}

	wg.Wait()
}

func TestHub_PublishToClosedChannel_NoPanic(t *testing.T) {
	h := NewHub(8)
	defer h.Shutdown()

	ctx := context.Background()
	events, unsub := h.Subscribe(ctx)

	unsub()

	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("expected channel to be closed")
		}
	default:
	}

	h.Publish(Event{Type: "test", ID: "1", Data: "payload"})
}

func TestHub_PublishAfterUnsubscribe_NoPanic(t *testing.T) {
	h := NewHub(8)
	defer h.Shutdown()

	ctx := context.Background()
	_, unsub := h.Subscribe(ctx)

	unsub()
	time.Sleep(10 * time.Millisecond)

	h.Publish(Event{Type: "test", ID: "1", Data: "payload"})
}

func TestHub_Shutdown_ClosesAllChannels(t *testing.T) {
	h := NewHub(8)

	ctx := context.Background()
	e1, u1 := h.Subscribe(ctx)
	e2, u2 := h.Subscribe(ctx)
	_ = e1
	_ = e2

	h.Shutdown()

	select {
	case _, ok := <-e1:
		if ok {
			t.Fatal("channel should be closed after shutdown")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for channel close")
	}

	select {
	case _, ok := <-e2:
		if ok {
			t.Fatal("channel should be closed after shutdown")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for channel close")
	}

	u1()
	u2()
}

func TestHub_SubscribeAfterShutdown_ReturnsClosedChannel(t *testing.T) {
	h := NewHub(8)
	h.Shutdown()

	events, _ := h.Subscribe(context.Background())
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("channel should be closed after shutdown")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for channel close")
	}
}

func TestHub_ConcurrentPublish_NoRace(t *testing.T) {
	h := NewHub(64)
	defer h.Shutdown()

	ctx := context.Background()
	_, unsub := h.Subscribe(ctx)
	defer unsub()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			h.Publish(Event{Type: "concurrent", ID: string(rune(id)), Data: id})
		}(i)
	}
	wg.Wait()
}

func TestHub_ContextCancellation_CleansUpSubscriber(t *testing.T) {
	h := NewHub(8)
	defer h.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	events, unsub := h.Subscribe(ctx)
	defer unsub()

	time.Sleep(100 * time.Millisecond)

	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("channel should be closed after context cancellation")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for channel close")
	}
}

func TestHub_PublishToManySubscribers(t *testing.T) {
	h := NewHub(16)
	defer h.Shutdown()

	ctx := context.Background()
	numSubs := 50
	channels := make([]<-chan Event, numSubs)
	unsubs := make([]func(), numSubs)

	for i := 0; i < numSubs; i++ {
		channels[i], unsubs[i] = h.Subscribe(ctx)
	}
	for _, unsub := range unsubs {
		defer unsub()
	}

	ev := Event{Type: "broadcast", ID: "1", Data: map[string]string{"msg": "hello"}}
	h.Publish(ev)

	var received int32
	for _, ch := range channels {
		select {
		case got := <-ch:
			if got.Type != ev.Type || got.ID != ev.ID {
				t.Fatalf("unexpected event: %+v", got)
			}
			atomic.AddInt32(&received, 1)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timeout waiting for broadcast")
		}
	}

	if received != int32(numSubs) {
		t.Fatalf("expected %d subscribers to receive event, got %d", numSubs, received)
	}
}

func TestSSEHandler_HeadersSet(t *testing.T) {
	h := NewHub(8)
	defer h.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stream", nil)
	req = req.WithContext(ctx)
	req.Header.Set("Accept", "text/event-stream")

	SSEHandler(h)(rr, req)

	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("expected text/event-stream content type, got %q", ct)
	}
}

func TestSSEHandler_ContextCancellation(t *testing.T) {
	h := NewHub(8)
	defer h.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stream", nil)
	req = req.WithContext(ctx)

	SSEHandler(h)(rr, req)

	if rr.Code != http.StatusOK && rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 200 or 503 after context cancellation, got %d", rr.Code)
	}
}

func TestSSEHandler_SendsInitialComment(t *testing.T) {
	h := NewHub(8)
	defer h.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stream", nil)
	req = req.WithContext(ctx)

	SSEHandler(h)(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, ": connected") {
		t.Fatal("expected initial SSE comment in response body")
	}
}

func TestSSEHandler_DeliversEvents(t *testing.T) {
	h := NewHub(8)
	defer h.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stream", nil)
	req = req.WithContext(ctx)

	go func() {
		time.Sleep(10 * time.Millisecond)
		h.Publish(Event{Type: "test", ID: "1", Data: map[string]string{"msg": "hello"}})
	}()

	SSEHandler(h)(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, `"type":"test"`) {
		t.Fatalf("expected event in SSE stream, got: %s", body)
	}
}

func TestHub_ShutdownIdempotent(t *testing.T) {
	h := NewHub(8)

	h.Shutdown()
	h.Shutdown()

	events, _ := h.Subscribe(context.Background())
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("channel should be closed after double shutdown")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for channel close")
	}
}

func TestHub_PublishAfterShutdown_Noop(t *testing.T) {
	h := NewHub(8)
	h.Shutdown()

	h.Publish(Event{Type: "test", ID: "1", Data: "payload"})
}

func TestMarshalEvent_ComplexData(t *testing.T) {
	ev := Event{
		Type: "beer.created",
		ID:   "b1",
		Data: map[string]any{
			"id":   "1",
			"name": "IPA",
			"tags": []string{"craft", "hoppy"},
		},
	}

	got, err := marshalEvent(ev)
	if err != nil {
		t.Fatalf("marshalEvent: %v", err)
	}

	var parsed Event
	if err := json.Unmarshal(got, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed.Type != ev.Type || parsed.ID != ev.ID {
		t.Fatalf("metadata mismatch: got=%+v want=%+v", parsed, ev)
	}
}
