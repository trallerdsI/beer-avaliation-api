package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"time"

	appErrors "beer-review-app/pkg/errors"
	"beer-review-app/pkg/uuid"
	"github.com/golang-jwt/jwt/v5"
)

const (
	accessTokenTTL  = 1 * time.Hour
	refreshTokenTTL = 30 * 24 * time.Hour
	issuerClaim     = "beer-review-app"
)

var jwtSecret = loadJWTSecret()

func loadJWTSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = os.Getenv("JWTSecret")
	}
	if secret == "" {
		if os.Getenv("TESTING") == "true" {
			return []byte("test-secret-do-not-use-in-production")
		}
		slog.Error("JWT_SECRET environment variable is not set; refusing to start with an insecure default")
		panic("JWT_SECRET environment variable is not set; refusing to start with an insecure default")
	}
	return []byte(secret)
}

type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

func GenerateToken(userID, role string) (string, error) {
	if len(jwtSecret) == 0 {
		return "", appErrors.ErrMissingSecret
	}

	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessTokenTTL)),
			Issuer:    issuerClaim,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func GenerateRefreshToken() (string, error) {
	return uuid.NewV7()
}

func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
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

func RefreshTokenTTL() time.Duration {
	return refreshTokenTTL
}

func JWTSecretForTest() {
	jwtSecret = loadJWTSecret()
}
