package uuid

import (
	"crypto/rand"
	"encoding/binary"
	"time"
)

// Pacote uuid provê geração de UUID versão 7 (RFC 9562) usando apenas a
// biblioteca padrão do Go. O uuid v7 é time-ordered: embute um timestamp de 48
// bits no início, permitindo ordenação cronológica natural de IDs (ideal para
// feeds sociais móveis) e evitando IDs sequenciais expostos (enumeração de
// /beers/1,2,3). Mantém o projeto Zero-Dependency (a lib google/uuid v1.6.0 não
// expõe NewV7).

// NewV7 gera um UUIDv7 (RFC 9562 §5.7) com 74 bits de aleatoridade e 48 bits de
// timestamp unix milissegundo. Em caso de falha de leitura de entropia, devolve
// erro (nunca um UUID vazio/silenzioso).
func NewV7() (string, error) {
	var randBytes [10]byte
	if _, err := rand.Read(randBytes[:]); err != nil {
		return "", err
	}

	now := time.Now().UnixMilli()
	var uuid [16]byte

	// unix_ts_ms (48 bits) nos primeiros 6 bytes.
	binary.BigEndian.PutUint16(uuid[0:2], uint16(now>>32))
	binary.BigEndian.PutUint32(uuid[2:6], uint32(now))

	// rand_a (12 bits) + rand_b (62 bits) preenchem o resto.
	uuid[6] = 0x70 | (randBytes[0] & 0x0f) // bits 4-7 = version 0b0111 (v7)
	uuid[7] = randBytes[1]
	copy(uuid[8:16], randBytes[2:10])

	// variant (RFC 4122): bits 6-7 = 0b10.
	uuid[8] = 0x80 | (uuid[8] & 0x3f)

	return format(uuid[:]), nil
}

// MustNewV7 devolve um UUIDv7 ou panic se a entropia falhar. Use apenas em
// caminhos onde a falha de CSRPNG é irrecuperável (arranque).
func MustNewV7() string {
	id, err := NewV7()
	if err != nil {
		panic("uuid: falha ao gerar UUIDv7: " + err.Error())
	}
	return id
}

// format codifica 16 bytes no formato canónico 8-4-4-4-12.
func format(b []byte) string {
	const hexd = "0123456789abcdef"
	out := make([]byte, 36)
	encode := func(dst, src int) {
		v := b[src]
		out[dst] = hexd[v>>4]
		out[dst+1] = hexd[v&0x0f]
	}
	encode(0, 0)
	encode(2, 1)
	encode(4, 2)
	encode(6, 3)
	out[8] = '-'
	encode(9, 4)
	encode(11, 5)
	out[13] = '-'
	encode(14, 6)
	encode(16, 7)
	out[18] = '-'
	encode(19, 8)
	encode(21, 9)
	out[23] = '-'
	encode(24, 10)
	encode(26, 11)
	encode(28, 12)
	encode(30, 13)
	encode(32, 14)
	encode(34, 15)
	return string(out)
}
