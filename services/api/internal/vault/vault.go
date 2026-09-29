// Package vault encrypts credentials and prompts before persistence.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

var ErrDecrypt = errors.New("protected data cannot be decrypted")

type Vault struct {
	keys    map[int]cipher.AEAD
	Current int
}

// Parse accepts a JSON map of positive versions to base64-encoded 32-byte keys.
func Parse(raw string, current int) (*Vault, error) {
	var encoded map[int]string
	if json.Unmarshal([]byte(raw), &encoded) != nil || current < 1 {
		return nil, errors.New("invalid encryption key configuration")
	}
	v := &Vault{keys: make(map[int]cipher.AEAD), Current: current}
	for version, value := range encoded {
		key, err := base64.StdEncoding.DecodeString(value)
		if err != nil || len(key) != 32 || version < 1 {
			return nil, errors.New("encryption keys must be versioned 256-bit keys")
		}
		block, _ := aes.NewCipher(key)
		v.keys[version], _ = cipher.NewGCM(block)
	}
	if v.keys[current] == nil {
		return nil, errors.New("current encryption key version is missing")
	}
	return v, nil
}
func AAD(owner, provider, id, purpose string) []byte {
	return []byte(fmt.Sprintf("open-flow:v1:%s:%s:%s:%s", owner, provider, id, purpose))
}
func (v *Vault) Seal(plaintext, aad []byte) ([]byte, int, error) {
	a := v.keys[v.Current]
	nonce := make([]byte, a.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, 0, errors.New("encryption unavailable")
	}
	return a.Seal(nonce, nonce, plaintext, aad), v.Current, nil
}
func (v *Vault) Open(payload []byte, version int, aad []byte) ([]byte, error) {
	a := v.keys[version]
	if a == nil || len(payload) < a.NonceSize()+a.Overhead() {
		return nil, ErrDecrypt
	}
	value, err := a.Open(nil, payload[:a.NonceSize()], payload[a.NonceSize():], aad)
	if err != nil {
		return nil, ErrDecrypt
	}
	return value, nil
}
