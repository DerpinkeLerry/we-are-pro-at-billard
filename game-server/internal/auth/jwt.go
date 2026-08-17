package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
)

type Claims struct {
	Iss                string `json:"iss"`
	Aud                string `json:"aud"`
	Sub                string `json:"sub"`
	Iat                int64  `json:"iat"`
	Nbf                int64  `json:"nbf"`
	Exp                int64  `json:"exp"`
	JTI                string `json:"jti"`
	PrincipalType      string `json:"principalType"`
	PrincipalID        string `json:"principalId"`
	Nickname           string `json:"nickname"`
	LobbyID            string `json:"lobbyId"`
	LobbyCode          string `json:"lobbyCode"`
	LobbyName          string `json:"lobbyName"`
	CueSkin            string `json:"cueSkin"`
	ShotTimerSeconds   int    `json:"shotTimerSeconds"`
	RulesetVersion     string `json:"rulesetVersion"`
	TableConfigVersion string `json:"tableConfigVersion"`
}

type Validator struct {
	secret []byte
	mu     sync.Mutex
	used   map[string]int64
}

func NewValidator(secret string) *Validator {
	return &Validator{secret: []byte(secret), used: map[string]int64{}}
}

func (v *Validator) Validate(token string) (Claims, error) {
	var c Claims
	if len(v.secret) < 32 {
		return c, errors.New("join secret too short")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return c, errors.New("invalid token")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return c, errors.New("invalid signature encoding")
	}
	mac := hmac.New(sha256.New, v.secret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return c, errors.New("bad signature")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return c, errors.New("bad payload")
	}
	if err = json.Unmarshal(body, &c); err != nil {
		return c, errors.New("bad claims")
	}
	now := time.Now().Unix()
	if c.Iss != "pool-web" || c.Aud != "pool-game" || c.Sub == "" || c.LobbyID == "" || c.JTI == "" {
		return c, errors.New("invalid claims")
	}
	if c.Exp < now || c.Nbf > now+5 || c.Iat > now+5 {
		return c, errors.New("expired_or_not_yet_valid")
	}
	if c.ShotTimerSeconds != 0 && c.ShotTimerSeconds != 30 && c.ShotTimerSeconds != 45 && c.ShotTimerSeconds != 60 {
		return c, errors.New("invalid timer")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	for j, exp := range v.used {
		if exp < now {
			delete(v.used, j)
		}
	}
	if _, ok := v.used[c.JTI]; ok {
		return c, errors.New("replayed token")
	}
	v.used[c.JTI] = c.Exp
	return c, nil
}
