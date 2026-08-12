package realtime

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestHub_SetNoCacheHeaders(t *testing.T) {
	h := NewHub(4)
	defer h.Shutdown()

	rr := httptest.NewRecorder()
	h.SetNoCache(rr)

	headers := map[string]string{
		"Content-Type":      "text/event-stream",
		"Cache-Control":     "no-cache",
		"Connection":        "keep-alive",
		"X-Accel-Buffering": "no",
	}
	for key, want := range headers {
		got := rr.Header().Get(key)
		if got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestMarshalEvent(t *testing.T) {
	tests := []struct {
		name string
		ev   Event
		want string
	}{
		{
			name: "evento simples",
			ev:   Event{Type: "beer.created", ID: "1", Data: map[string]string{"name": "IPA"}},
			want: `{"type":"beer.created","id":"1","data":{"name":"IPA"}}`,
		},
		{
			name: "evento com data nula",
			ev:   Event{Type: "ping", ID: "2", Data: nil},
			want: `{"type":"ping","id":"2","data":null}`,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := marshalEvent(tc.ev)
			if err != nil {
				t.Fatalf("marshalEvent: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("marshalEvent = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestNewHub_DefaultBufferSize(t *testing.T) {
	h := NewHub(0)
	if h.bufferSize != 64 {
		t.Fatalf("default bufferSize = %d, want 64", h.bufferSize)
	}
}

func TestNewHub_CustomBufferSize(t *testing.T) {
	h := NewHub(16)
	if h.bufferSize != 16 {
		t.Fatalf("bufferSize = %d, want 16", h.bufferSize)
	}
}

func TestHub_PublishAndReceive(t *testing.T) {
	h := NewHub(8)
	defer h.Shutdown()

	ctx := context.Background()
	events, unsub := h.Subscribe(ctx)
	defer unsub()

	want := Event{Type: "beer.created", ID: "b1", Data: map[string]any{"name": "IPA"}}
	h.Publish(want)

	got := <-events
	if got.Type != want.Type || got.ID != want.ID {
		t.Fatalf("evento inesperado: %+v", got)
	}
}

func TestHub_PublishToMultipleSubscribers(t *testing.T) {
	h := NewHub(8)
	defer h.Shutdown()

	ctx := context.Background()
	e1, u1 := h.Subscribe(ctx)
	defer u1()
	e2, u2 := h.Subscribe(ctx)
	defer u2()

	want := Event{Type: "comment.added", ID: "c1", Data: map[string]any{"text": "nice"}}
	h.Publish(want)

	assertEvent(t, <-e1, want)
	assertEvent(t, <-e2, want)
}

func TestHub_BackpressureDropsSlowSubscriber(t *testing.T) {
	h := NewHub(1)
	defer h.Shutdown()

	ctx := context.Background()
	_, unsub := h.Subscribe(ctx)
	defer unsub()

	h.Publish(Event{Type: "x", ID: "1"})
	h.Publish(Event{Type: "x", ID: "2"})
}

func TestHub_SubscribeAfterShutdown(t *testing.T) {
	h := NewHub(4)
	h.Shutdown()

	events, _ := h.Subscribe(context.Background())
	if _, ok := <-events; ok {
		t.Fatal("canal deveria estar fechado para subscribe após shutdown")
	}
}

func assertEvent(t *testing.T, got Event, want Event) {
	t.Helper()
	if got.Type != want.Type || got.ID != want.ID {
		t.Fatalf("evento inesperado: got=%+v want=%+v", got, want)
	}
	gotJSON, _ := json.Marshal(got.Data)
	wantJSON, _ := json.Marshal(want.Data)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("data mismatch: got=%s want=%s", gotJSON, wantJSON)
	}
}
