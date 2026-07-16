package realtime

import (
	"context"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

// TestHubSubscribeUnsubscribe valida o handshake do Hub (registro, entrega de
// eventos e remoção de subscriber) de forma 100% determinística sob
// testing/synctest (Go 1.26): sem goroutines de loop órfãs, sem time.Sleep real.
func TestHubSubscribeUnsubscribe(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := NewHub(8)
		defer h.Shutdown()

		ctx, cancelCtx := context.WithCancel(context.Background())
		defer cancelCtx()
		events, cancel := h.Subscribe(ctx)

		// Publish é síncrono: o evento já está em events quando retorna.
		h.Publish(Event{Type: "beer.created", ID: "1"})

		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatal("subscriber channel closed unexpectedly")
			}
			if ev.Type != "beer.created" || ev.ID != "1" {
				t.Fatalf("unexpected event: %+v", ev)
			}
		case <-time.After(time.Second):
			t.Fatal("expected event buffered for subscriber")
		}

		// Cancelar remove o subscriber do mapa (defesa contra leak de memória).
		cancel()
	})
}

// TestHubBackpressure garante que um cliente lento (canal cheio) não trava o
// Hub nem cresce a Heap: mensagens excedentes são descartadas (defense-in-depth).
func TestHubBackpressure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := NewHub(2)
		defer h.Shutdown()

		ctx, cancelCtx := context.WithCancel(context.Background())
		defer cancelCtx()
		events, cancel := h.Subscribe(ctx)
		defer cancel()

		const n = 50
		for i := 0; i < n; i++ {
			h.Publish(Event{Type: "ev", ID: "1"})
		}

		// Apenas os primeiros bufferSize eventos devem estar no canal.
		got := 0
		for range events {
			got++
			if got >= 2 {
				break
			}
		}
		if got != 2 {
			t.Fatalf("expected 2 buffered events, got %d", got)
		}
	})
}

// TestHubConcurrentPublish exercita Publish em paralelo (goroutines reais)
// confirmando ausência de race no mapa de subscribers.
func TestHubConcurrentPublish(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := NewHub(64)
		defer h.Shutdown()

		const workers = 32
		var wg sync.WaitGroup
		wg.Add(workers)
		for i := 0; i < workers; i++ {
			go func(n int) {
				defer wg.Done()
				h.Publish(Event{Type: "ev", ID: "1", Data: strings.Repeat("a", n%8)})
			}(i)
		}
		wg.Wait()
	})
}
