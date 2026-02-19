package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Auth struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	keysDir    string
}

func New(keysDir string) (*Auth, error) {
	a := &Auth{keysDir: keysDir}
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return nil, err
	}
	privPath := filepath.Join(keysDir, "master.key")
	pubPath := filepath.Join(keysDir, "master.pub")

	if _, err := os.Stat(privPath); os.IsNotExist(err) {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(privPath, priv, 0600); err != nil {
			return nil, err
		}
		if err := os.WriteFile(pubPath, pub, 0644); err != nil {
			return nil, err
		}
		a.privateKey = priv
		a.publicKey = pub
	} else {
		priv, err := os.ReadFile(privPath)
		if err != nil {
			return nil, err
		}
		a.privateKey = ed25519.PrivateKey(priv)
		a.publicKey = a.privateKey.Public().(ed25519.PublicKey)
	}
	return a, nil
}

func (a *Auth) IssueToken(deviceID string) (string, error) {
	payload := fmt.Sprintf("%s:%d", deviceID, time.Now().Unix())
	sig := ed25519.Sign(a.privateKey, []byte(payload))
	token := base64.URLEncoding.EncodeToString([]byte(payload)) + "." + base64.URLEncoding.EncodeToString(sig)
	return token, nil
}

func (a *Auth) VerifyToken(token string) (string, bool) {
	parts := splitToken(token)
	if parts == nil {
		return "", false
	}
	payload, err := base64.URLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	sig, err := base64.URLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false
	}
	if !ed25519.Verify(a.publicKey, payload, sig) {
		return "", false
	}
	for i := len(payload) - 1; i >= 0; i-- {
		if payload[i] == ':' {
			return string(payload[:i]), true
		}
	}
	return "", false
}

func TokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func splitToken(token string) []string {
	for i := len(token) - 1; i >= 0; i-- {
		if token[i] == '.' {
			return []string{token[:i], token[i+1:]}
		}
	}
	return nil
}
