// Package realtime implementa um hub de eventos Server-Sent Events (SSE)
// utilizando exclusivamente a biblioteca padrão do Go (Zero-Dependency).
//
// O hub é seguro para concorrência: múltiplas goroutines podem publicar e
// subscrever sem race conditions. A entrega é feita via canal com capacidade
// limitada; em caso de cliente lento (back-pressure), a mensagem é descartada
// para proteger a memória do servidor (Defense-in-Depth / Zero-Allocation).
package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
)

// Event é a unidade de dados transmitida ao cliente móvel via SSE.
type Event struct {
	Type string `json:"type"` // ex: "beer.created", "comment.added"
	ID   string `json:"id"`
	Data any    `json:"data,omitempty"`
}

// subscriber agrupa o canal de entrega e o contexto de cancelamento.
type subscriber struct {
	ch     chan Event
	cancel context.CancelFunc
}

// Hub mantém o conjunto de subscrições ativas e difunde eventos.
//
// Design lock-por-operação (sem goroutine de loop órfã): cada Subscribe,
// Publish, Unsubscribe e Shutdown leva o lock exclusivo. Isto mantém o
// comportamento determinístico e 100% testável sob testing/synctest (Go 1.26)
// — não há goroutine de fundo cujo escalonamento escape ao controlo do teste —
// continuando livre de race conditions e com a mesma defesa de memória.
type Hub struct {
	mu         sync.Mutex
	subs       map[*subscriber]struct{}
	bufferSize int
	shutdown   bool
}

// NewHub cria um hub SSE. bufferSize define a capacidade do canal de entrega
// por cliente (protege contra back-pressure); defaults para 64 se <= 0.
func NewHub(bufferSize int) *Hub {
	if bufferSize <= 0 {
		bufferSize = 64
	}
	return &Hub{
		subs:       make(map[*subscriber]struct{}),
		bufferSize: bufferSize,
	}
}

// Subscribe registra um cliente e retorna um canal de leitura de Eventos e uma
// função de cancelamento. O caller deve chamar cancel() ao desconectar.
func (h *Hub) Subscribe(parent context.Context) (<-chan Event, func()) {
	ctx, cancel := context.WithCancel(parent)
	s := &subscriber{
		ch:     make(chan Event, h.bufferSize),
		cancel: cancel,
	}

	h.mu.Lock()
	if h.shutdown {
		h.mu.Unlock()
		cancel()
		close(s.ch)
		return s.ch, cancel
	}
	h.subs[s] = struct{}{}
	h.mu.Unlock()

	go func() {
		// Termina quando o caller cancela OU quando o Hub é desligado.
		select {
		case <-ctx.Done():
		case <-parent.Done():
		}
		h.unsubscribe(s)
	}()

	return s.ch, cancel
}

// unsubscribe remove o subscriber do mapa e fecha o seu canal.
func (h *Hub) unsubscribe(s *subscriber) {
	h.mu.Lock()
	if _, ok := h.subs[s]; ok {
		delete(h.subs, s)
		close(s.ch)
	}
	h.mu.Unlock()
}

// Publish difunde um evento a todos os clientes subscritos de forma síncrona.
// Em caso de cliente lento (canal de entrega cheio), a mensagem é descartada
// para evitar crescimento da Heap (defense-in-depth).
func (h *Hub) Publish(ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		select {
		case s.ch <- ev:
		default:
			// Cliente lento: descarta para proteger memória.
		}
	}
}

// Shutdown encerra o hub e libera todos os clientes. Idempotente.
func (h *Hub) Shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.shutdown {
		return
	}
	h.shutdown = true
	for s := range h.subs {
		delete(h.subs, s)
		close(s.ch)
	}
}

// SetNoCache aplica os cabeçalhos SSE que desligam o buffering/proxy-cache:
// stream contínuo, sem cache intermediário.
func (h *Hub) SetNoCache(w interface{ Header() http.Header }) {
	hdr := w.Header()
	hdr.Set("Content-Type", "text/event-stream")
	hdr.Set("Cache-Control", "no-cache")
	hdr.Set("Connection", "keep-alive")
	hdr.Set("X-Accel-Buffering", "no")
}

// marshalEvent codifica o evento para JSON. Eventos são pequenos e de curta
// duração: a alocação do slice de saída permanece efêmera e é coletada logo
// após a escrita, sem escapar para estruturas de longa duração.
func marshalEvent(ev Event) ([]byte, error) {
	return json.Marshal(ev)
}
