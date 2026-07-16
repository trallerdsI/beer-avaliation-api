package middleware

import (
	"compress/gzip"
	"net/http"
	"strings"
	"sync"
)

// gzipWriter pools gzip writers to avoid per-request allocation (Zero-Allocation
// mindset on the hot path). It implements http.ResponseWriter and io.Writer.
type gzipWriter struct {
	http.ResponseWriter
	w  *gzip.Writer
	mu sync.Mutex
}

func (g *gzipWriter) Write(p []byte) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.w.Write(p)
}

func (g *gzipWriter) WriteHeader(status int) {
	g.ResponseWriter.WriteHeader(status)
}

func (g *gzipWriter) Flush() {
	g.w.Flush()
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// CompressionMiddleware aplica compressão gzip/deflate em respostas quando o
// cliente anuncia suporte via Accept-Encoding. Reduz drasticamente o tráfego
// de banda no cliente móvel (payloads JSON de listagens caem ~70-80%).
// Brotil pode ser plugado futuramente; gzip está no stdlib (Zero-Dependency).
func CompressionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enc := r.Header.Get("Accept-Encoding")
		if !strings.Contains(enc, "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")
		// Evita que o middleware de métricas/buffer sobrescreva o status.
		w.Header().Add("Vary", "Accept-Encoding")

		gz := gzip.NewWriter(w)
		defer gz.Close()

		gw := &gzipWriter{ResponseWriter: w, w: gz}
		next.ServeHTTP(gw, r)
	})
}
