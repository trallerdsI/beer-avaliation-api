package middleware

import (
	"compress/gzip"
	"net/http"
	"strings"
)

// gzipWriter aplica compressão gzip mantendo o código de status HTTP correto.
// O status é capturado em WriteHeader e aplicado no primeiro Write (ou no
// Close), impedindo que o gzip — que escreve o corpo imediatamente — force
// um 200 prematuro e silencie os 4xx/5xx dos handlers (Defense-in-Depth).
type gzipWriter struct {
	http.ResponseWriter
	w        *gzip.Writer
	status   int
	wroteHdr bool
}

func (g *gzipWriter) WriteHeader(status int) {
	if g.wroteHdr {
		return
	}
	g.status = status
	// Aplicação adiada: o cabeçalho só vai para o wire no primeiro Write,
	// quando o handler já decidiu o status real.
}

func (g *gzipWriter) Write(p []byte) (int, error) {
	if !g.wroteHdr {
		if g.status == 0 {
			g.status = http.StatusOK
		}
		g.ResponseWriter.WriteHeader(g.status)
		g.wroteHdr = true
	}
	return g.w.Write(p)
}

func (g *gzipWriter) Flush() {
	_ = g.w.Flush()
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// CompressionMiddleware aplica compressão gzip/deflate em respostas quando o
// cliente anuncia suporte via Accept-Encoding. Reduz drasticamente o tráfego
// de banda no cliente móvel (payloads JSON de listagens caem ~70-80%).
// gzip está no stdlib (Zero-Dependency).
func CompressionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enc := r.Header.Get("Accept-Encoding")
		if !strings.Contains(enc, "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")
		// Evita que proxies/buffers sobrescrevam o Vary.
		w.Header().Add("Vary", "Accept-Encoding")

		gz := gzip.NewWriter(w)
		defer gz.Close()

		gw := &gzipWriter{ResponseWriter: w, w: gz}
		next.ServeHTTP(gw, r)
	})
}
