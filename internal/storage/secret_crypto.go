package storage

import (
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"unicode/utf8"

	"golang.org/x/crypto/chacha20poly1305"
)

var (
	errInvalidSecret        = errors.New("invalid_secret")
	errSecretAuthentication = errors.New("secret_authentication_failed")
)

// secretContext binds ciphertext to one installation, row, and version. Names
// are deliberately excluded: renaming metadata does not change secret identity.
type secretContext struct {
	installationID  string
	secretID        string
	resourceVersion int64
	keyVersion      int64
}

func (c secretContext) associatedData() ([]byte, error) {
	if c.installationID == "" || c.secretID == "" || len(c.installationID) > 128 || len(c.secretID) > 128 || c.resourceVersion < 1 || c.keyVersion < 1 {
		return nil, errSecretAuthentication
	}
	data := []byte("modelcairn/secret/v1\x00")
	for _, value := range []string{c.installationID, c.secretID} {
		data = binary.BigEndian.AppendUint32(data, uint32(len(value)))
		data = append(data, value...)
	}
	data = binary.BigEndian.AppendUint64(data, uint64(c.resourceVersion))
	data = binary.BigEndian.AppendUint64(data, uint64(c.keyVersion))
	return data, nil
}

func sealSecret(key, plaintext []byte, context secretContext) (nonce, ciphertext []byte, err error) {
	if len(plaintext) < 8 || len(plaintext) > 16384 || !utf8.Valid(plaintext) {
		return nil, nil, errInvalidSecret
	}
	ad, err := context.associatedData()
	if err != nil {
		return nil, nil, err
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, nil, errSecretAuthentication
	}
	nonce = make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, errors.New("secret_randomness_unavailable")
	}
	return nonce, aead.Seal(nil, nonce, plaintext, ad), nil
}

func openSecret(key, nonce, ciphertext []byte, context secretContext) ([]byte, error) {
	if len(nonce) != chacha20poly1305.NonceSizeX || len(ciphertext) < 8+chacha20poly1305.Overhead || len(ciphertext) > 16384+chacha20poly1305.Overhead {
		return nil, errSecretAuthentication
	}
	ad, err := context.associatedData()
	if err != nil {
		return nil, errSecretAuthentication
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, errSecretAuthentication
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, ad)
	if err != nil {
		return nil, errSecretAuthentication
	}
	if !utf8.Valid(plaintext) {
		clear(plaintext)
		return nil, errSecretAuthentication
	}
	return plaintext, nil
}

func secretFingerprint(key, plaintext []byte, installationID string) (string, error) {
	if len(key) != chacha20poly1305.KeySize || installationID == "" {
		return "", errSecretAuthentication
	}
	derived, err := hkdf.Key(sha256.New, key, []byte(installationID), "modelcairn/secret-fingerprint/v1", 32)
	if err != nil {
		return "", errSecretAuthentication
	}
	defer clear(derived)
	mac := hmac.New(sha256.New, derived)
	_, _ = mac.Write(plaintext)
	return "mc_fp_" + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:12]), nil
}
