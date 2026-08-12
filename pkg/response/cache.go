package response

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
)

// Pacote response/cache: helpers de HTTP Caching (RFC 9111) para reduzir o
// tráfego do app Flutter em redes móveis instáveis (3G/4G/5G). O servidor emite
// um ETag forte (hash do conteúdo/versão) e responde 304 Not Modified quando o
// cliente reenvia If-None-Match e o recurso não mudou — poupando banda e
// bateria. O 304 nunca tem body, logo é compatível com o CompressionMiddleware
// (o gzipWriter simplesmente não escreve corpo).

// ETagForVersion gera um ETag forte a partir de um valor de versão/timestamp
// estável (ex: updated_at). Usado em recursos indivíveis (detalhe de cerveja,
// perfil) onde updated_at reflete a última alteração.
func ETagForVersion(version string) string {
	return `"` + version + `"`
}

// ETagForContent gera um ETag forte (hash SHA-256) do payload serializado.
// Usado em listas (feed, /beers) cuja versão agregada é cara de calcular.
func ETagForContent(payload []byte) string {
	sum := sha256.Sum256(payload)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

// IfNoneMatchMatches devolve true se o header If-None-Match do cliente contém
// o ETag atual (validação fraca ou forte). RFC 9111 §3.2.
func IfNoneMatchMatches(r *http.Request, etag string) bool {
	inm := r.Header.Get("If-None-Match")
	if inm == "" {
		return false
	}
	// "*" casa com qualquer recurso existente.
	if inm == "*" {
		return true
	}
	// RFC 9111: comparação de ETag é ASCII case-sensitive; múltiplos valores
	// separados por vírgula são aceites.
	for _, cand := range splitETags(inm) {
		if cand == etag {
			return true
		}
	}
	return false
}

// splitETags separa a lista comma-separated de ETags do header.
func splitETags(inm string) []string {
	var out []string
	start := 0
	for i := 0; i < len(inm); i++ {
		if inm[i] == ',' {
			if t := trimSpace(inm[start:i]); t != "" {
				out = append(out, t)
			}
			start = i + 1
		}
	}
	if t := trimSpace(inm[start:]); t != "" {
		out = append(out, t)
	}
	return out
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// SetCacheHeaders aplica ETag e Cache-Control numa resposta de leitura. O
// maxAge (segundos) instrui caches intermediários (proxy/CDN/Flutter); mustRevalidate
// força revalidação no servidor após a expiração. O Vary: Accept-Encoding é
// sempre definido (RFC 9111) para que proxies/CDNs não sirvam a variante
// gzip de um cliente a outro que não a suporta.
func SetCacheHeaders(w http.ResponseWriter, etag string, maxAge int, mustRevalidate bool) {
	w.Header().Set("ETag", etag)
	w.Header().Add("Vary", "Accept-Encoding")
	cc := "public, max-age=" + strconv.Itoa(maxAge)
	if mustRevalidate {
		cc += ", must-revalidate"
	}
	w.Header().Set("Cache-Control", cc)
}

// SendNotModified responde 304 sem body (RFC 9111). Mantém o ETag para o
// cliente revalidar na próxima vez.
func SendNotModified(w http.ResponseWriter, etag string) {
	if etag != "" {
		w.Header().Set("ETag", etag)
	}
	w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
	w.WriteHeader(http.StatusNotModified)
}
