package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type TicketClaims struct {
	Origin    string `json:"origin"`
	Scope     string `json:"scope"`
	ExpiresAt int64  `json:"exp"`
}

func VerifyTicket(token string, secret string, origin string, scope string) (*TicketClaims, error) {
	if token == "" {
		return nil, errors.New("missing connector ticket")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, errors.New("invalid connector ticket format")
	}

	payloadSegment := parts[0]
	signatureSegment := parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payloadSegment))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(signatureSegment)) {
		return nil, errors.New("invalid connector ticket signature")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadSegment)
	if err != nil {
		return nil, fmt.Errorf("decode connector ticket payload: %w", err)
	}

	var claims TicketClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("parse connector ticket payload: %w", err)
	}

	if time.Now().Unix() >= claims.ExpiresAt {
		return nil, errors.New("connector ticket expired")
	}

	normalizedOrigin := normalizeOrigin(origin)
	if normalizedOrigin == "" || normalizeOrigin(claims.Origin) != normalizedOrigin {
		return nil, errors.New("connector ticket origin mismatch")
	}

	if claims.Scope != scope && claims.Scope != "all" {
		return nil, errors.New("connector ticket scope mismatch")
	}

	return &claims, nil
}

func normalizeOrigin(origin string) string {
	trimmed := strings.TrimSpace(origin)
	if trimmed == "" {
		return ""
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return trimmed
	}

	hostname := strings.TrimSuffix(parsed.Hostname(), ".")
	if hostname == "" {
		return trimmed
	}

	parsed.Host = hostname
	if port := parsed.Port(); port != "" {
		parsed.Host = hostname + ":" + port
	}

	return parsed.String()
}