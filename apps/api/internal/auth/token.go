package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Claims struct {
	UserID int64  `json:"sub"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Expiry int64  `json:"exp"`
}

type TokenManager struct {
	key []byte
	now func() time.Time
}

func NewTokenManager(key string) TokenManager {
	return TokenManager{
		key: []byte(key),
		now: time.Now,
	}
}

func (m TokenManager) Issue(user User, ttl time.Duration) (string, error) {
	if len(m.key) < 16 {
		return "", errors.New("token key must be at least 16 bytes")
	}

	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	claims := Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		Expiry: m.now().Add(ttl).Unix(),
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	payload := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	return payload + "." + m.sign(payload), nil
}

func (m TokenManager) Verify(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, errors.New("invalid token")
	}

	payload := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(m.sign(payload))) {
		return Claims{}, errors.New("invalid token signature")
	}

	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, err
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return Claims{}, err
	}
	if header.Algorithm != "HS256" || header.Type != "JWT" {
		return Claims{}, errors.New("unsupported token header")
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, err
	}
	var claims Claims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return Claims{}, err
	}
	if claims.UserID <= 0 || claims.Email == "" || claims.Role == "" {
		return Claims{}, errors.New("invalid token claims")
	}
	if claims.Expiry <= m.now().Unix() {
		return Claims{}, errors.New("token expired")
	}

	return claims, nil
}

func (m TokenManager) sign(payload string) string {
	mac := hmac.New(sha256.New, m.key)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (c Claims) Subject() string {
	return strconv.FormatInt(c.UserID, 10)
}

func (c Claims) String() string {
	return fmt.Sprintf("%s:%s", c.Subject(), c.Role)
}
