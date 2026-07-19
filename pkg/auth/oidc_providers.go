package auth

import (
	"os"
	"strings"
	"sync"
)

// OIDCConfig guarda os verifiers por provider. Carregado de env vars no
// arranque. Sem hardcode de segredos: apenas issuer/audience/jwksUrl públicos.
type OIDCConfig struct {
	mu        sync.RWMutex
	verifiers map[string]*OIDCVerifier
}

var oidcConfig = NewOIDCConfigFromEnv()

// NewOIDCConfigFromEnv constrói os verifiers a partir de env vars:
//
//	OAUTH_GOOGLE_AUDIENCE, OAUTH_APPLE_AUDIENCE (opcional OAUTH_*_JWKS_URL)
func NewOIDCConfigFromEnv() *OIDCConfig {
	c := &OIDCConfig{verifiers: make(map[string]*OIDCVerifier)}
	c.register("google", "https://accounts.google.com", os.Getenv("OAUTH_GOOGLE_AUDIENCE"), os.Getenv("OAUTH_GOOGLE_JWKS_URL"))
	c.register("apple", "https://appleid.apple.com", os.Getenv("OAUTH_APPLE_AUDIENCE"), os.Getenv("OAUTH_APPLE_JWKS_URL"))
	return c
}

func (c *OIDCConfig) register(provider, issuer, audience, jwksURL string) {
	if audience == "" {
		return // provider não configurado
	}
	c.verifiers[provider] = NewOIDCVerifier(issuer, audience, jwksURL)
}

// Verifier devolve o verifier de um provider, ou nil se desconhecido/não configurado.
func (c *OIDCConfig) Verifier(provider string) *OIDCVerifier {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.verifiers[strings.ToLower(provider)]
}

// RegisterVerifier injeta/sobrescreve um verifier (usado em arranque
// customizado e em testes).
func (c *OIDCConfig) RegisterVerifier(provider, issuer, audience, jwksURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.verifiers[strings.ToLower(provider)] = NewOIDCVerifier(issuer, audience, jwksURL)
}

// OIDC devolve a config global (carregada no arranque).
func OIDC() *OIDCConfig { return oidcConfig }
