package response

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
)

// errorEnvelope é uma struct de stack (não escapa para a Heap) usada para
// codificar a resposta de erro sem alocar um map[string]string por request.
type errorEnvelope struct {
	Error  string `json:"error"`
	Detail string `json:"detail,omitempty"`
}

// SendResponse escreve payload JSON com Content-Type apropriado.
// A compressão (gzip/brotli) é aplicada pelo CompressionMiddleware.
func SendResponse(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

// SendError escreve um erro JSON enxuto. Usa uma struct de stack em vez de
// map[string]string, evitando 1 alocação de mapa no heap por resposta de erro
// (hot path de todos os 4xx/5xx). O detalhe opcional expõe a causa raiz
// (ex: erro de DB) para facilitar o diagnóstico sem esconder a mensagem.
func SendError(w http.ResponseWriter, message string, statusCode int, detail ...string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	env := errorEnvelope{Error: message}
	if len(detail) > 0 && detail[0] != "" {
		env.Detail = detail[0]
	}
	_ = json.NewEncoder(w).Encode(env)
}

// SelectFields projeta apenas os campos solicitados de cada elemento de uma
// fatia, reduzindo o tamanho do JSON enviado ao cliente móvel em telas de
// listagem (ex: evita trafegar descrições longas). Se fields estiver vazio,
// retorna o payload original (sem alocação extra de projeção).
//
// Uso: /api/v1/beers?fields=id,name,image_url
//
// Defesa (Pilar 4): limita o número de campos pedidos para evitar abuse de
// CPU/reflexão e cardinalidade de projeção (cap de 32 campos).
func SelectFields(payload any, fields string) any {
	fields = strings.TrimSpace(fields)
	if fields == "" {
		return payload
	}

	const maxFields = 32
	want := make(map[string]struct{}, 8)
	for _, f := range strings.Split(fields, ",") {
		if f = strings.TrimSpace(f); f == "" {
			continue
		}
		if len(want) >= maxFields {
			break
		}
		want[strings.ToLower(f)] = struct{}{}
	}
	if len(want) == 0 {
		return payload
	}

	rv := reflect.ValueOf(payload)
	if rv.Kind() != reflect.Slice {
		return payload
	}

	out := make([]any, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		if elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		if elem.Kind() != reflect.Struct {
			// Elemento não-estrutura: mantém como está.
			out = append(out, elem.Interface())
			continue
		}
		proj := make(map[string]any, len(want))
		t := elem.Type()
		for j := 0; j < t.NumField(); j++ {
			f := t.Field(j)
			// Respeita o nome JSON (ou o nome do campo) para casar com o cliente.
			name := jsonName(f)
			if _, ok := want[strings.ToLower(name)]; ok {
				proj[name] = elem.Field(j).Interface()
			}
		}
		out = append(out, proj)
	}
	return out
}

// jsonName extrai o nome do campo conforme a tag `json`, sem opções de omit.
func jsonName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" {
		return f.Name
	}
	if idx := strings.IndexByte(tag, ','); idx >= 0 {
		tag = tag[:idx]
	}
	if tag == "" {
		return f.Name
	}
	return tag
}
