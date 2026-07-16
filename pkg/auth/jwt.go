package auth

import (
	"log/slog"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"beer-review-app/pkg/errors"
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

func GenerateToken(userID string) (string, error) {
	if jwtSecret == nil {
		return "", errors.ErrMissingSecret
	}
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func ValidateToken(tokenString string) (string, error) {
	if jwtSecret == nil {
		return "", errors.ErrMissingSecret
	}
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims["user_id"].(string), nil
	}

	return "", err
}
