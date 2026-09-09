package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims carried by the access token.
type Claims struct {
	TenantID string `json:"tid"`
	Username string `json:"usr"`
	IsSuper  bool   `json:"sup,omitempty"`
	jwt.RegisteredClaims
}

type accessToken struct {
	Token     string
	ExpiresAt time.Time
	JTI       string
}

func issueAccess(secret string, ttl time.Duration, userID, tenantID uuid.UUID, username string, isSuper bool) (accessToken, error) {
	now := time.Now()
	jti := uuid.NewString()
	claims := Claims{
		TenantID: tenantID.String(),
		Username: username,
		IsSuper:  isSuper,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    "zhiyuche",
		},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		return accessToken{}, fmt.Errorf("sign token: %w", err)
	}
	return accessToken{Token: s, ExpiresAt: claims.ExpiresAt.Time, JTI: jti}, nil
}

var errInvalidToken = errors.New("invalid token")

func parseAccess(secret, raw string) (*Claims, error) {
	tok, err := jwt.ParseWithClaims(raw, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errInvalidToken
		}
		return []byte(secret), nil
	}, jwt.WithIssuer("zhiyuche"), jwt.WithLeeway(30*time.Second))
	if err != nil {
		return nil, err
	}
	claims, ok := tok.Claims.(*Claims)
	if !ok || !tok.Valid {
		return nil, errInvalidToken
	}
	return claims, nil
}
