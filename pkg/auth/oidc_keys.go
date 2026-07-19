package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// parseJWKS converte o documento JWKS (mapa genérico) num mapa kid->*rsa.PublicKey.
// Suporta apenas RSA (RS256), que é o que Google e Apple usam para id_tokens.
func parseJWKS(raw map[string]interface{}) (map[string]*rsa.PublicKey, error) {
	keysRaw, ok := raw["keys"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("jwks sem array 'keys'")
	}
	set := make(map[string]*rsa.PublicKey, len(keysRaw))
	for _, k := range keysRaw {
		km, ok := k.(map[string]interface{})
		if !ok {
			continue
		}
		alg, _ := km["alg"].(string)
		if alg != "" && alg != "RS256" {
			continue // ignora chaves não-RS256
		}
		kid, _ := km["kid"].(string)
		if kid == "" {
			continue
		}
		nB64, _ := km["n"].(string)
		eB64, _ := km["e"].(string)
		if nB64 == "" || eB64 == "" {
			continue
		}
		pub, err := jwkToRSA(nB64, eB64)
		if err != nil {
			return nil, fmt.Errorf("chave %q inválida: %w", kid, err)
		}
		set[kid] = pub
	}
	if len(set) == 0 {
		return nil, fmt.Errorf("nenhuma chave RS256 utilizável no JWKS")
	}
	return set, nil
}

// jwkToRSA reconstrói uma chave pública RSA a partir dos campos base64url n/e.
func jwkToRSA(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, fmt.Errorf("n inválido: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, fmt.Errorf("e inválido: %w", err)
	}
	if len(eBytes) < 1 || len(eBytes) > 4 {
		return nil, fmt.Errorf("e com tamanho inválido")
	}
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	pub := &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: e,
	}
	return pub, nil
}

// parseMaxAge extrai "max-age=NNN" de um header Cache-Control.
func parseMaxAge(cc string) (time.Duration, bool) {
	for _, part := range strings.Split(cc, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "max-age=") {
			val := strings.TrimPrefix(part, "max-age=")
			if n, err := strconv.Atoi(val); err == nil && n >= 0 {
				return time.Duration(n) * time.Second, true
			}
		}
	}
	return 0, false
}

func readAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

// compile-time: garantir que jwt.MapClaims é o tipo usado no hook de teste.
var _ = jwt.MapClaims{}
