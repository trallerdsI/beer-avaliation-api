package auth

import (
	"errors"
	"log/slog"
	"os"
	"time"

	appErrors "beer-review-app/pkg/errors"
	"github.com/golang-jwt/jwt/v5"
)

const (
	tokenTTL    = 24 * time.Hour
	issuerClaim = "beer-review-app"
)

var jwtSecret = loadJWTSecret()

func loadJWTSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = os.Getenv("JWTSecret")
	}
	if secret == "" {
		slog.Error("JWT_SECRET environment variable is not set; refusing to start with an insecure default")
		return nil
	}
	return []byte(secret)
}

type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func GenerateToken(userID, role string) (string, error) {
	if len(jwtSecret) == 0 {
		return "", appErrors.ErrMissingSecret
	}

	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenTTL)),
			Issuer:    issuerClaim,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func ValidateToken(tokenString string) (string, string, error) {
	if len(jwtSecret) == 0 {
		return "", "", appErrors.ErrMissingSecret
	}

	var claims Claims
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return "", "", appErrors.NewAppError(401, "invalid token signature", err)
	}

	if claims.Issuer != issuerClaim {
		return "", "", appErrors.NewAppError(401, "invalid token issuer", nil)
	}

	return claims.UserID, claims.Role, nil
}

func JWTSecretForTest() {
	jwtSecret = loadJWTSecret()
}
