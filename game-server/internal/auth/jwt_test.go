package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func signedToken(t *testing.T, secret string, c Claims) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, _ := json.Marshal(c)
	h := base64.RawURLEncoding.EncodeToString(header)
	p := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(h + "." + p))
	return h + "." + p + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func validClaims() Claims {
	now := time.Now().Unix()
	return Claims{Iss: "pool-web", Aud: "pool-game", Sub: "guest:123", Iat: now, Nbf: now - 1, Exp: now + 60, JTI: "one-use-token", PrincipalType: "guest", PrincipalID: "123", Nickname: "Tester", LobbyID: "lobby-id", LobbyCode: "ABC123", LobbyName: "Test", CueSkin: "classic-maple", ShotTimerSeconds: 45}
}

func TestValidateRejectsReplay(t *testing.T) {
	secret := strings.Repeat("s", 40)
	v := NewValidator(secret)
	tok := signedToken(t, secret, validClaims())
	if _, err := v.Validate(tok); err != nil {
		t.Fatalf("first validation failed: %v", err)
	}
	if _, err := v.Validate(tok); err == nil {
		t.Fatal("replayed join token accepted")
	}
}

func TestValidateRejectsTampering(t *testing.T) {
	secret := strings.Repeat("k", 40)
	v := NewValidator(secret)
	tok := signedToken(t, secret, validClaims())
	parts := strings.Split(tok, ".")
	payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var c Claims
	_ = json.Unmarshal(payload, &c)
	c.LobbyID = "attacker-lobby"
	mutated, _ := json.Marshal(c)
	parts[1] = base64.RawURLEncoding.EncodeToString(mutated)
	if _, err := v.Validate(strings.Join(parts, ".")); err == nil {
		t.Fatal("tampered JWT accepted")
	}
}

func TestValidateRejectsExpiredAndBadTimer(t *testing.T) {
	secret := strings.Repeat("x", 40)
	v := NewValidator(secret)
	c := validClaims()
	c.Exp = time.Now().Add(-time.Second).Unix()
	c.JTI = "expired"
	if _, err := v.Validate(signedToken(t, secret, c)); err == nil {
		t.Fatal("expired token accepted")
	}
	c = validClaims()
	c.JTI = "bad-timer"
	c.ShotTimerSeconds = 17
	if _, err := v.Validate(signedToken(t, secret, c)); err == nil {
		t.Fatal("unsupported shot timer accepted")
	}
}
