package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	appErrors "beer-review-app/pkg/errors"
	"github.com/golang-jwt/jwt/v5"
)

func setupJWT(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-key")
	jwtSecret = loadJWTSecret()
}

func mustGenerate(t *testing.T, userID, role string) string {
	t.Helper()
	token, err := GenerateToken(userID, role)
	if err != nil {
		t.Fatalf("GenerateToken(%q, %q): %v", userID, role, err)
	}
	return token
}

func TestGenerateToken_RoundTrip(t *testing.T) {
	setupJWT(t)

	tests := []struct {
		name   string
		userID string
		role   string
	}{
		{"user with role", "user-123", "admin"},
		{"user without role", "user-456", ""},
		{"empty userID", "", "user"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			token, err := GenerateToken(tc.userID, tc.role)
			if err != nil {
				t.Fatalf("GenerateToken: %v", err)
			}

			uid, role, err := ValidateToken(token)
			if err != nil {
				t.Fatalf("ValidateToken: %v", err)
			}
			if uid != tc.userID {
				t.Errorf("userID = %q, want %q", uid, tc.userID)
			}
			if role != tc.role {
				t.Errorf("role = %q, want %q", role, tc.role)
			}
		})
	}
}

func TestGenerateToken_MissingSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	jwtSecret = nil
	defer func() { jwtSecret = loadJWTSecret() }()

	_, err := GenerateToken("user-1", "admin")
	if !errors.Is(err, appErrors.ErrMissingSecret) {
		t.Fatalf("expected ErrMissingSecret, got %v", err)
	}
}

func TestValidateToken(t *testing.T) {
	setupJWT(t)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	signHS256 := func(t *testing.T, claims Claims) string {
		t.Helper()
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		s, err := tok.SignedString(jwtSecret)
		if err != nil {
			t.Fatalf("sign HS256: %v", err)
		}
		return s
	}

	signRS256 := func(t *testing.T, claims Claims) string {
		t.Helper()
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		s, err := tok.SignedString(priv)
		if err != nil {
			t.Fatalf("sign RS256: %v", err)
		}
		return s
	}

	tamper := func(t *testing.T, token string) string {
		t.Helper()
		if len(token) < 2 {
			return token + "xx"
		}
		return token[:len(token)-2] + "xx"
	}

	tests := []struct {
		name        string
		token       func(t *testing.T) string
		wantUserID  string
		wantRole    string
		wantErrCode int
		wantDetail  string
	}{
		{
			name:       "valid token with role",
			token:      func(t *testing.T) string { return mustGenerate(t, "user-1", "admin") },
			wantUserID: "user-1",
			wantRole:   "admin",
		},
		{
			name:       "valid token without role",
			token:      func(t *testing.T) string { return mustGenerate(t, "user-2", "") },
			wantUserID: "user-2",
			wantRole:   "",
		},
		{
			name: "tampered token",
			token: func(t *testing.T) string {
				return tamper(t, mustGenerate(t, "user-1", ""))
			},
			wantErrCode: 401,
			wantDetail:  "invalid token signature",
		},
		{
			name: "wrong signing method RS256",
			token: func(t *testing.T) string {
				return signRS256(t, Claims{
					UserID: "user-1",
					Role:   "admin",
					RegisteredClaims: jwt.RegisteredClaims{
						ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
						Issuer:    issuerClaim,
					},
				})
			},
			wantErrCode: 401,
			wantDetail:  "invalid token signature",
		},
		{
			name: "expired token",
			token: func(t *testing.T) string {
				return signHS256(t, Claims{
					UserID: "user-1",
					Role:   "admin",
					RegisteredClaims: jwt.RegisteredClaims{
						ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
						Issuer:    issuerClaim,
					},
				})
			},
			wantErrCode: 401,
			wantDetail:  "invalid token signature",
		},
		{
			name: "wrong issuer",
			token: func(t *testing.T) string {
				return signHS256(t, Claims{
					UserID: "user-1",
					Role:   "admin",
					RegisteredClaims: jwt.RegisteredClaims{
						ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
						Issuer:    "wrong-issuer",
					},
				})
			},
			wantErrCode: 401,
			wantDetail:  "invalid token issuer",
		},
		{
			name:       "malformed empty string",
			token:      func(t *testing.T) string { return "" },
			wantErrCode: 401,
			wantDetail:  "invalid token signature",
		},
		{
			name:       "malformed single segment",
			token:      func(t *testing.T) string { return "abc" },
			wantErrCode: 401,
			wantDetail:  "invalid token signature",
		},
		{
			name:       "malformed two segments",
			token:      func(t *testing.T) string { return "abc.def" },
			wantErrCode: 401,
			wantDetail:  "invalid token signature",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			token := tc.token(t)
			userID, role, err := ValidateToken(token)

			if tc.wantErrCode != 0 {
				if err == nil {
					t.Fatalf("expected error with code %d, got nil", tc.wantErrCode)
				}
				var appErr *appErrors.AppError
				if !errors.As(err, &appErr) {
					t.Fatalf("expected *AppError, got %T: %v", err, err)
				}
				if appErr.Code != tc.wantErrCode {
					t.Errorf("code = %d, want %d", appErr.Code, tc.wantErrCode)
				}
				if appErr.Detail != tc.wantDetail {
					t.Errorf("detail = %q, want %q", appErr.Detail, tc.wantDetail)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if userID != tc.wantUserID {
				t.Errorf("userID = %q, want %q", userID, tc.wantUserID)
			}
			if role != tc.wantRole {
				t.Errorf("role = %q, want %q", role, tc.wantRole)
			}
		})
	}
}

func TestValidateToken_MissingSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	jwtSecret = nil
	defer func() { jwtSecret = loadJWTSecret() }()

	_, _, err := ValidateToken("x.y.z")
	if !errors.Is(err, appErrors.ErrMissingSecret) {
		t.Fatalf("expected ErrMissingSecret, got %v", err)
	}
}

func TestJWTSecretForTest(t *testing.T) {
	t.Setenv("JWT_SECRET", "reloaded-secret")
	JWTSecretForTest()

	token, err := GenerateToken("user-1", "user")
	if err != nil {
		t.Fatalf("GenerateToken after reload: %v", err)
	}

	uid, role, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken after reload: %v", err)
	}
	if uid != "user-1" || role != "user" {
		t.Errorf("roundtrip failed: userID=%q role=%q", uid, role)
	}
}
