package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"time"

	"beer-review-app/pkg/errors"
)

// Implementação HS256 100% stdlib (Pilar 1: Zero-Dependency). Substitui a
// dependência externa github.com/golang-jwt/jwt/v5, mantendo a mesma API
// pública (GenerateToken/ValidateToken) e a mesma semântica de claims.

const (
	algHS256    = "HS256"
	typJWT      = "JWT"
	tokenTTL    = 24 * time.Hour
	issuerClaim = "beer-review-app"
)

// jwtSecret é carregado de JWT_SECRET (ou JWTSecret). Sem fallback inseguro:
// se a variável não estiver definida, o servidor falha cedo em vez de usar
// uma chave hardcoded conhecida.
var jwtSecret = loadJWTSecret()

func loadJWTSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = os.Getenv("JWTSecret")
	}
	if secret == "" {
		slog.Error("JWT_SECRET environment variable is not set; refusing to start with an insecure default")
		// Em Go 1.26 não usamos panic silencioso: retornamos erro em Generate/Validate.
		return nil
	}
	return []byte(secret)
}

// header e payload são as structs de claims assinadas. user_id e exp seguem
// o padrão esperado pelos clients; iss reforça a procedência.
type claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	Exp    int64  `json:"exp"`
	Iss    string `json:"iss"`
}

func base64url(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func base64urlDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// GenerateToken cria um JWT HS256 assinado com o segredo configurado.
func GenerateToken(userID, role string) (string, error) {
	if jwtSecret == nil {
		return "", errors.ErrMissingSecret
	}

	payload, err := json.Marshal(claims{
		UserID: userID,
		Role:   role,
		Exp:    time.Now().Add(tokenTTL).Unix(),
		Iss:    issuerClaim,
	})
	if err != nil {
		return "", errors.NewAppError(500, "failed to encode token claims", err)
	}

	header := base64url([]byte(`{"alg":"HS256","typ":"JWT"}`))
	body := base64url(payload)
	signingInput := header + "." + body

	sig := signHMAC(signingInput)
	return signingInput + "." + sig, nil
}

// ValidateToken verifica a assinatura HMAC-SHA256 e a expiração, devolvendo o
// user_id em caso de sucesso.
func ValidateToken(tokenString string) (string, string, error) {
	if jwtSecret == nil {
		return "", "", errors.ErrMissingSecret
	}

	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return "", "", errors.NewAppError(401, "invalid token format", nil)
	}

	signingInput := parts[0] + "." + parts[1]
	if want := signHMAC(signingInput); !hmac.Equal([]byte(want), []byte(parts[2])) {
		return "", "", errors.NewAppError(401, "invalid token signature", nil)
	}

	payload, err := base64urlDecode(parts[1])
	if err != nil {
		return "", "", errors.NewAppError(401, "invalid token payload", err)
	}

	var c claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return "", "", errors.NewAppError(401, "invalid token claims", err)
	}

	if c.Iss != issuerClaim {
		return "", "", errors.NewAppError(401, "invalid token issuer", nil)
	}
	if time.Now().Unix() > c.Exp {
		return "", "", errors.NewAppError(401, "token expired", nil)
	}

	return c.UserID, c.Role, nil
}

// signHMAC calcula a assinatura base64url(HMAC-SHA256) do input. O slice de
// saída do HMAC é efêmero e recolhido logo após a codificação.
func signHMAC(signingInput string) string {
	mac := hmac.New(sha256.New, jwtSecret)
	mac.Write([]byte(signingInput))
	return base64url(mac.Sum(nil))
}

// JWTSecretForTest recarrega o segredo a partir do ambiente. Exposto apenas
// para testes (que definem JWT_SECRET via t.Setenv depois do init do package).
func JWTSecretForTest() {
	jwtSecret = loadJWTSecret()
}
