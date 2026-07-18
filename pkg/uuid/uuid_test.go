package uuid

import (
	"strings"
	"testing"
	"time"
)

// TestNewV7Format confirma que o UUIDv7 respeita o formato canónico 8-4-4-4-12
// e carimbo de versão/variant corretos (RFC 9562).
func TestNewV7Format(t *testing.T) {
	id, err := NewV7()
	if err != nil {
		t.Fatalf("NewV7() erro inesperado: %v", err)
	}
	if len(id) != 36 {
		t.Fatalf("comprimento = %d, queria 36", len(id))
	}
	if id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		t.Fatalf("separadores inválidos: %q", id)
	}
	// Versão 7 => nibble em id[14] deve ser '7'.
	if id[14] != '7' {
		t.Fatalf("version nibble = %q, queria '7'", id[14])
	}
	// Variant RFC 4122 => nibble em id[19] deve ser 8, 9, a ou b.
	switch id[19] {
	case '8', '9', 'a', 'b':
	default:
		t.Fatalf("variant nibble = %q, queria 8-9/a-b", id[19])
	}
}

// TestNewV7Unique garante ausência de colisão em lote.
func TestNewV7Unique(t *testing.T) {
	const n = 10000
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		id, err := NewV7()
		if err != nil {
			t.Fatalf("NewV7() erro: %v", err)
		}
		if _, ok := seen[id]; ok {
			t.Fatalf("colisão detectada: %q", id)
		}
		seen[id] = struct{}{}
	}
}

// TestNewV7TimeOrdered verifica que UUIDs v7 adjacentes são ordenáveis por
// tempo (propriedade central da RFC 9562 para feeds sociais).
func TestNewV7TimeOrdered(t *testing.T) {
	first, _ := NewV7()
	// Pequena pausa para garantir timestamp distinto (resolução ms).
	time.Sleep(2 * time.Millisecond)
	second, _ := NewV7()
	if strings.Compare(first, second) >= 0 {
		t.Fatalf("não time-ordered: %q >= %q", first, second)
	}
}

// TestMustNewV7NaoPanica no caminho feliz.
func TestMustNewV7NaoPanica(t *testing.T) {
	if id := MustNewV7(); len(id) != 36 {
		t.Fatalf("MustNewV7 comprimento = %d", len(id))
	}
}
