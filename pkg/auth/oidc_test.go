package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// gera par RSA em memória (sem internet) para assinar tokens de teste.
func genRSA(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gerar RSA: %v", err)
	}
	return k
}

// monta um JWKS (mapa) a partir de uma chave privada com kid dado.
func jwksFor(t *testing.T, kid string, pub *rsa.PublicKey) map[string]interface{} {
	t.Helper()
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString([]byte{0x01, 0x00, 0x01}) // 65537
	return map[string]interface{}{
		"keys": []interface{}{
			map[string]interface{}{
				"kty": "RSA",
				"alg": "RS256",
				"use": "sig",
				"kid": kid,
				"n":   n,
				"e":   e,
			},
		},
	}
}

const (
	testIssuer   = "https://accounts.google.com"
	testAudience = "cerveja-client-id.apps.googleusercontent.com"
)

// TestOIDCVerify_VetoresSupremo2026 cobre validação criptográfica offline,
// rotação de chaves e vetores de ataque (alg-swap, aud errado, exp).
func TestOIDCVerify_VetoresSupremo2026(t *testing.T) {
	priv := genRSA(t)
	kid1 := "key-1"

	// Servidor JWKS mockado (httptest) — sem rede externa.
	jwksMu := sync.Mutex{}
	currentJWKS := jwksFor(t, kid1, &priv.PublicKey)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jwksMu.Lock()
		defer jwksMu.Unlock()
		_ = json.NewEncoder(w).Encode(currentJWKS)
	}))
	defer srv.Close()

	v := NewOIDCVerifier(testIssuer, testAudience, srv.URL)
	v.httpClient.Timeout = 2 * time.Second

	sign := func(t *testing.T, alg jwt.SigningMethod, kid string, claims jwt.MapClaims) string {
		t.Helper()
		tok := jwt.NewWithClaims(alg, claims)
		if kid != "" {
			tok.Header["kid"] = kid
		}
		var key interface{} = priv
		if alg == jwt.SigningMethodNone {
			key = jwt.UnsafeAllowNoneSignatureType
		}
		s, err := tok.SignedString(key)
		if err != nil {
			t.Fatalf("assinar: %v", err)
		}
		return s
	}

	tests := []struct {
		name    string
		token   func(t *testing.T) string
		wantErr bool
		wantSub string
	}{
		{
			name: "Sucesso - Google valido RS256",
			token: func(t *testing.T) string {
				return sign(t, jwt.SigningMethodRS256, kid1, jwt.MapClaims{
					"iss":   testIssuer,
					"sub":   "google|123",
					"aud":   testAudience,
					"exp":   time.Now().Add(time.Hour).Unix(),
					"email": "user@cerveja.com",
					"name":  "Cerveja User",
				})
			},
			wantErr: false,
			wantSub: "google|123",
		},
		{
			name: "Erro - Alg Swap (none)",
			token: func(t *testing.T) string {
				return sign(t, jwt.SigningMethodNone, "", jwt.MapClaims{
					"iss": testIssuer,
					"sub": "google|999",
					"exp": time.Now().Add(time.Hour).Unix(),
				})
			},
			wantErr: true,
		},
		{
			name: "Erro - Aud errado (Spotify)",
			token: func(t *testing.T) string {
				return sign(t, jwt.SigningMethodRS256, kid1, jwt.MapClaims{
					"iss": testIssuer,
					"sub": "google|123",
					"aud": "spotify-client-id",
					"exp": time.Now().Add(time.Hour).Unix(),
				})
			},
			wantErr: true,
		},
		{
			name: "Erro - Token expirado",
			token: func(t *testing.T) string {
				return sign(t, jwt.SigningMethodRS256, kid1, jwt.MapClaims{
					"iss": testIssuer,
					"sub": "google|123",
					"aud": testAudience,
					"exp": time.Now().Add(-time.Hour).Unix(),
				})
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tok := tc.token(t)
			claims, err := v.Verify(context.Background(), tok)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("esperado erro, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("não esperado erro: %v", err)
			}
			if claims.Sub != tc.wantSub {
				t.Errorf("sub = %q, want %q", claims.Sub, tc.wantSub)
			}
		})
	}
}

// TestOIDCVerify_RotacaoChave valida que, após troca de chave no IdP, o
// verifier invalida o cache, refaz fetch e aceita token assinado pela nova.
func TestOIDCVerify_RotacaoChave(t *testing.T) {
	priv1 := genRSA(t)
	priv2 := genRSA(t)
	kid1, kid2 := "key-1", "key-2"

	jwksMu := sync.Mutex{}
	current := jwksFor(t, kid1, &priv1.PublicKey)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jwksMu.Lock()
		defer jwksMu.Unlock()
		_ = json.NewEncoder(w).Encode(current)
	}))
	defer srv.Close()

	v := NewOIDCVerifier(testIssuer, testAudience, srv.URL)
	v.httpClient.Timeout = 2 * time.Second

	signWith := func(priv *rsa.PrivateKey, kid string) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"iss": testIssuer,
			"sub": "google|rot",
			"aud": testAudience,
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		tok.Header["kid"] = kid
		s, err := tok.SignedString(priv)
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		return s
	}

	// 1. token com chave 1 funciona.
	if _, err := v.Verify(context.Background(), signWith(priv1, kid1)); err != nil {
		t.Fatalf("chave1 deveria validar: %v", err)
	}

	// 2. IdP rodou chaves: agora só a chave2 está no JWKS.
	jwksMu.Lock()
	current = jwksFor(t, kid2, &priv2.PublicKey)
	jwksMu.Unlock()

	// 3. token antigo (kid1) deve falhar a assinatura e disparar refetch.
	v.InvalidateCache()
	if _, err := v.Verify(context.Background(), signWith(priv1, kid1)); err == nil {
		t.Fatalf("esperado falha com chave roldada")
	}

	// 4. novo token com chave2 deve ser aceite após refresh do cache.
	if _, err := v.Verify(context.Background(), signWith(priv2, kid2)); err != nil {
		t.Fatalf("chave2 deveria validar após rotação: %v", err)
	}
}
