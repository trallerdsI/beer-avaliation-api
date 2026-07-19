package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Verificação OIDC (RFC 6749 / OpenID Connect) de id_tokens externos
// (Google, Apple) usando golang-jwt/v5. O nosso JWT de sessão interno
// continua a ser HS256 (pkg/auth/jwt.go); aqui validamos apenas tokens
// RS256 assinados pelos IdPs. No fim do fluxo emitimos o nosso JWT HS256,
// pelo que o middleware Auth existente não precisa de mudar.

// jwksURL por issuer conhecido. Sem hardcode de segredos; apenas os
// endpoints públicos de descoberta de chaves dos provedores.
var defaultJWKSURLs = map[string]string{
	"https://accounts.google.com": "https://www.googleapis.com/oauth2/v3/certs",
	"https://appleid.apple.com":   "https://appleid.apple.com/auth/keys",
}

// OIDCVerifier valida id_tokens de um issuer, com cache de JWKS em memória
// (respeita Cache-Control: max-age para suportar rotação de chaves sem
// bater no endpoint a cada login).
type OIDCVerifier struct {
	issuer   string
	audience string
	jwksURL  string

	mu             sync.RWMutex
	keyFunc        jwt.Keyfunc
	cachedAt       time.Time
	cacheMaxAge    time.Duration
	httpClient     *http.Client
	fetchJWKS      func(ctx context.Context, url string) (jwt.MapClaims, error)  // hook de teste
	verifyOverride func(ctx context.Context, idToken string) (OIDCClaims, error) // hook de teste
}

// NewOIDCVerifier cria um verificador para o issuer/audience dados.
// jwksURL é opcional: se vazio, usa o endpoint conhecido do issuer.
func NewOIDCVerifier(issuer, audience, jwksURL string) *OIDCVerifier {
	if jwksURL == "" {
		jwksURL = defaultJWKSURLs[issuer]
	}
	return &OIDCVerifier{
		issuer:   issuer,
		audience: audience,
		jwksURL:  jwksURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Claims do id_token OIDC que extraímos.
type OIDCClaims struct {
	Sub      string
	Email    string
	Username string
}

// Verify valida a assinatura RS256 e as claims obrigatórias (iss, aud, exp,
// alg RS256). Em cache miss de chave, refaz o fetch do JWKS (rotação).
func (v *OIDCVerifier) Verify(ctx context.Context, idToken string) (OIDCClaims, error) {
	if v.verifyOverride != nil {
		return v.verifyOverride(ctx, idToken)
	}
	if v.jwksURL == "" {
		return OIDCClaims{}, fmt.Errorf("issuer %q sem JWKS URL configurado", v.issuer)
	}
	if err := v.ensureKeys(ctx); err != nil {
		return OIDCClaims{}, err
	}

	v.mu.RLock()
	kf := v.keyFunc
	v.mu.RUnlock()

	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"RS256"}), // bloqueia "none" e HMAC (alg-swap)
		jwt.WithExpirationRequired(),            // exp obrigatório
	)
	tok, err := parser.Parse(idToken, kf)
	if err != nil {
		// Tenta rotação de chave: invalidate cache e refaz fetch.
		if v.refreshKeys(ctx) == nil {
			v.mu.RLock()
			kf = v.keyFunc
			v.mu.RUnlock()
			tok, err = parser.Parse(idToken, kf)
		}
		if err != nil {
			return OIDCClaims{}, fmt.Errorf("id_token inválido: %w", err)
		}
	}

	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return OIDCClaims{}, fmt.Errorf("claims inesperadas no id_token")
	}

	// Valida issuer e audience explicitamente (defesa em profundidade).
	if iss, _ := claims["iss"].(string); iss != v.issuer {
		return OIDCClaims{}, fmt.Errorf("issuer não corresponde: %q", iss)
	}
	aud, _ := claims["aud"].(string)
	if aud != v.audience {
		// Apple envia aud como []interface{} por vezes.
		if auds, ok := claims["aud"].([]interface{}); ok {
			matched := false
			for _, a := range auds {
				if s, ok := a.(string); ok && s == v.audience {
					matched = true
					break
				}
			}
			if !matched {
				return OIDCClaims{}, fmt.Errorf("audience não corresponde")
			}
		} else {
			return OIDCClaims{}, fmt.Errorf("audience não corresponde: %q", aud)
		}
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return OIDCClaims{}, fmt.Errorf("sub em falta no id_token")
	}
	email, _ := claims["email"].(string)
	username, _ := claims["name"].(string)
	if username == "" {
		username = email
	}

	return OIDCClaims{Sub: sub, Email: email, Username: username}, nil
}

// InvalidateCache força o refetch do JWKS no próximo Verify. Usado quando se
// deteta revogação/rotação de chave (ex.: kid desconhecido) ou para testes.
func (v *OIDCVerifier) InvalidateCache() {
	v.mu.Lock()
	v.keyFunc = nil
	v.cachedAt = time.Time{}
	v.mu.Unlock()
}

// SetVerifyOverride injeta uma função de verificação alternativa (apenas para
// testes, evitando rede contra IdPs reais).
func (v *OIDCVerifier) SetVerifyOverride(fn func(ctx context.Context, idToken string) (OIDCClaims, error)) {
	v.verifyOverride = fn
}
func (v *OIDCVerifier) ensureKeys(ctx context.Context) error {
	v.mu.RLock()
	has := v.keyFunc != nil
	fresh := time.Since(v.cachedAt) < v.cacheMaxAge
	v.mu.RUnlock()
	if has && fresh {
		return nil
	}
	return v.refreshKeys(ctx)
}

// refreshKeys faz download do JWKS e reconstrói a keyfunc.
func (v *OIDCVerifier) refreshKeys(ctx context.Context) error {
	var raw map[string]interface{}
	if v.fetchJWKS != nil {
		// hook de teste: devolve o JWKS como claims (mock).
		m, err := v.fetchJWKS(ctx, v.jwksURL)
		if err != nil {
			return err
		}
		raw = m
	} else {
		body, err := v.fetchJWKSRaw(ctx)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return fmt.Errorf("jwks inválido: %w", err)
		}
	}

	set, err := parseJWKS(raw)
	if err != nil {
		return err
	}
	kf := func(t *jwt.Token) (interface{}, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, fmt.Errorf("kid em falta no header do token")
		}
		key, ok := set[kid]
		if !ok {
			return nil, fmt.Errorf("chave %q não encontrada no JWKS", kid)
		}
		return key, nil
	}

	v.mu.Lock()
	v.keyFunc = kf
	v.cachedAt = time.Now()
	// Default 1h se o JWKS não indicar max-age.
	v.cacheMaxAge = time.Hour
	if cc, ok := raw["cache-control"].(string); ok {
		// alguns IdPs incluem; parsing simples de max-age=.
		if maxAge, found := parseMaxAge(cc); found {
			v.cacheMaxAge = maxAge
		}
	}
	v.mu.Unlock()
	slog.DebugContext(ctx, "JWKS recarregado", "issuer", v.issuer, "keys", len(set))
	return nil
}

func (v *OIDCVerifier) fetchJWKSRaw(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("falha a obter JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS retornou status %d", resp.StatusCode)
	}
	return readAll(resp.Body)
}
