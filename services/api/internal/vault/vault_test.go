package vault

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"testing"
)

func TestOwnerBindingRotationAndWrongKey(t *testing.T) {
	k1 := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	k2 := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32))
	old, _ := Parse(fmt.Sprintf(`{"1":%q}`, k1), 1)
	rotating, _ := Parse(fmt.Sprintf(`{"1":%q,"2":%q}`, k1, k2), 2)
	aad := AAD("owner", "gemini", "id", "credential")
	ciphertext, version, err := old.Seal([]byte("secret"), aad)
	if err != nil || bytes.Contains(ciphertext, []byte("secret")) {
		t.Fatal("plaintext exposed")
	}
	if value, err := rotating.Open(ciphertext, version, aad); err != nil || string(value) != "secret" {
		t.Fatal(err)
	}
	for _, binding := range [][]byte{AAD("other", "gemini", "id", "credential"), AAD("owner", "other", "id", "credential"), AAD("owner", "gemini", "other", "credential")} {
		if _, err := old.Open(ciphertext, version, binding); err == nil {
			t.Fatal("accepted wrong binding")
		}
	}
	wrong, _ := Parse(fmt.Sprintf(`{"1":%q}`, k2), 1)
	if _, err := wrong.Open(ciphertext, version, aad); err != ErrDecrypt {
		t.Fatal(err)
	}
	ciphertext[20] ^= 1
	if _, err := old.Open(ciphertext, version, aad); err != ErrDecrypt {
		t.Fatal("accepted tampering")
	}
	if _, err := Parse(`{"1":"invalid"}`, 1); err == nil {
		t.Fatal("invalid key accepted")
	}
}
