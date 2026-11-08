// Package auth implements authentication: access JWT issuing and parsing,
// login sessions, refresh-token rotation and one-time email action tokens.
package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	RoleUser  = "user"
	RoleAdmin = "admin"

	ClientTypeWeb    = "web"
	ClientTypeMobile = "mobile"
)

// AccessClaims is the payload of the access JWT (docs/AUTH.md, «Токены»).
type AccessClaims struct {
	SID        uint64 `json:"sid"`
	ClientType string `json:"client_type"`
	Role       string `json:"role"`
	jwt.RegisteredClaims
}

// UserID returns users.id parsed from the standard "sub" claim.
func (c *AccessClaims) UserID() (uint32, error) {
	id, err := strconv.ParseUint(c.Subject, 10, 32)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("auth: invalid subject %q", c.Subject)
	}
	return uint32(id), nil
}

// JWTManager issues and verifies HS256 access tokens.
type JWTManager struct {
	secret    []byte
	accessTTL time.Duration
}

func NewJWTManager(secret string, accessTTL time.Duration) *JWTManager {
	return &JWTManager{secret: []byte(secret), accessTTL: accessTTL}
}

func (m *JWTManager) AccessTTL() time.Duration {
	return m.accessTTL
}

func (m *JWTManager) IssueAccess(userID uint32, sid uint64, clientType, role string) (string, error) {
	now := time.Now()
	claims := AccessClaims{
		SID:        sid,
		ClientType: clientType,
		Role:       role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(uint64(userID), 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// ParseAccess verifies the signature, the exp claim and the payload shape.
// It does not touch the database: the session check is a separate step.
func (m *JWTManager) ParseAccess(tokenStr string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(*jwt.Token) (any, error) {
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	if claims.SID == 0 {
		return nil, errors.New("auth: missing sid claim")
	}
	if claims.ClientType == "" || claims.Role == "" {
		return nil, errors.New("auth: missing client_type or role claim")
	}
	if _, err := claims.UserID(); err != nil {
		return nil, err
	}
	return claims, nil
}
