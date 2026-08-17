package http

import (
	"fmt"
	"net/http"
)

// setPaginationLinks aplica o cabeçalho Link (RFC 8288 — Web Linking) com as
// relações de paginação, permitindo ao cliente Flutter fazer infinite scroll
// apenas seguindo a URL de rel="next" (sem recomputar a fórmula de páginas).
//
// Omitte rel="next" na última página (page*pageSize >= total). Inclui rel="last"
// sempre que haja mais de uma página. As URLs são absolutas (RFC 8288 §3.2),
// construídas a partir do host da requisição com scheme https (produção);
//
// O cabeçalho não afeta o ETag por conteúdo (RFC 9111): este já incorpora page/
// total no body, logo páginas distintas têm ETags distintos naturalmente.
func setPaginationLinks(w http.ResponseWriter, r *http.Request, page, pageSize, total int, path string) {
	if total <= 0 || pageSize <= 0 {
		return
	}
	lastPage := (total + pageSize - 1) / pageSize
	if page < 1 {
		page = 1
	}
	if page >= lastPage {
		// Última página: não há next.
		return
	}

	scheme := "https"
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") == "http" {
		scheme = "http"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto == "http" || proto == "https" {
		scheme = proto
	}
	base := fmt.Sprintf("%s://%s", scheme, r.Host)

	var links []string
	if page < lastPage {
		links = append(links, fmt.Sprintf("<%s%s?page=%d&pageSize=%d>; rel=\"next\"",
			base, path, page+1, pageSize))
	}
	if lastPage > 1 {
		links = append(links, fmt.Sprintf("<%s%s?page=%d&pageSize=%d>; rel=\"last\"",
			base, path, lastPage, pageSize))
	}

	if len(links) > 0 {
		w.Header().Set("Link", joinLinks(links))
	}
}

// joinLinks junta as relações Link por vírgula (RFC 8288).
func joinLinks(links []string) string {
	out := links[0]
	for _, l := range links[1:] {
		out += ", " + l
	}
	return out
}
