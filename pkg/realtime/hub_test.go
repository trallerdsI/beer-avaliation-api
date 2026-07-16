package realtime

import (
	"context"
	"testing"

	"testing/synctest"
)

// TestHubPublishSubscribe verifica entrega determinística de um evento a um
// subscritor, sem time.Sleep. Subscribe regista de forma síncrona (lock
// per-operação), pelo que a publicação seguinte é sempre recebida.
func TestHubPublishSubscribe(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := NewHub(8)
		defer h.Shutdown()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		events, unsub := h.Subscribe(ctx)
		defer unsub()

		want := Event{Type: "beer.created", ID: "b1", Data: map[string]any{"name": "IPA"}}
		h.Publish(want)

		got := <-events
		if got.Type != want.Type || got.ID != want.ID {
			t.Fatalf("evento inesperado: %+v", got)
		}
	})
}

// TestHubBackpressureDrop valida que, sob back-pressure (canal cheio e sem
// leitura), Publish não bloqueia: o cliente lento apenas perde o evento,
// protegendo a memória do servidor (defense-in-depth).
func TestHubBackpressureDrop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := NewHub(1)
		defer h.Shutdown()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		_, unsub := h.Subscribe(ctx)
		defer unsub()

		// Enche o canal do cliente (não lemos) e publica em excesso.
		h.Publish(Event{Type: "x", ID: "1"})
		h.Publish(Event{Type: "x", ID: "2"}) // descartado sem bloquear
		// Sem deadlock: o back-pressure funcionou (teste determinístico).
	})
}

// TestHubUnsubscribeClosed valida que, após cancelar a subscrição, o canal é
// fechado exatamente uma vez e Publish subsequente não entrega ao cliente.
func TestHubUnsubscribeClosed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := NewHub(4)
		defer h.Shutdown()

		ctx, cancel := context.WithCancel(context.Background())
		events, unsub := h.Subscribe(ctx)
		unsub()
		cancel()

		if _, ok := <-events; ok {
			t.Fatal("canal deveria estar fechado após unsubscribe")
		}

		// Publicar após unsubscribe não pode entregar (mapa já não tem o sub).
		h.Publish(Event{Type: "y", ID: "9"})
	})
}
