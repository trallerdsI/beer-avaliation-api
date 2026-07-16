package realtime

import (
	"context"
	"fmt"
	"net/http"

	"log/slog"
)

// SSEHandler transmite eventos do Hub para o cliente via Server-Sent Events.
// Substitui polling repetitivo do app móvel por um stream persistente e
// eficiente, reduzindo consumo de bateria e banda (sem requisições periódicas).
//
// O handler respeita o cancelamento do contexto da requisição (cliente
// desconecta) e faz flush a cada evento para entrega em tempo real.
func SSEHandler(h *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		// Cabeçalhos SSE: sem cache, stream contínuo, desligar buffering.
		h.SetNoCache(w)

		ctx := r.Context()

		// Envia o comentário inicial para abrir o stream imediatamente.
		fmt.Fprintf(w, ": connected\n\n")
		flusher.Flush()

		events, cancel := h.Subscribe(ctx)
		defer cancel()

		slog.InfoContext(ctx, "sse client subscribed")

		for {
			select {
			case <-ctx.Done():
				slog.InfoContext(ctx, "sse client disconnected")
				return
			case ev, ok := <-events:
				if !ok {
					return
				}
				writeSSE(w, ev)
				flusher.Flush()
			}
		}
	}
}

// writeSSE formata um evento no protocolo SSE (data: <json>\n\n).
func writeSSE(w http.ResponseWriter, ev Event) {
	// Event é pequeno e de curta duração: codificado diretamente no buffer
	// de escrita (stack-friendly), sem alocação intermediária desnecessária.
	payload, err := marshalEvent(ev)
	if err != nil {
		slog.ErrorContext(context.Background(), "sse marshal failed", "err", err)
		return
	}
	fmt.Fprintf(w, "event: %s\n", ev.Type)
	fmt.Fprintf(w, "id: %s\n", ev.ID)
	fmt.Fprintf(w, "data: %s\n\n", payload)
}
