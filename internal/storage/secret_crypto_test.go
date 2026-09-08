package storage

import (
	"bytes"
	"errors"
	"testing"

	"golang.org/x/crypto/chacha20poly1305"
)

func TestSecretCipherAuthenticatesIdentityAndVersions(t *testing.T) {
	key := bytes.Repeat([]byte{42}, 32)
	value := []byte("canary-secret-value")
	ctx := secretContext{"installation", "secret", 1, 1}
	nonce, ciphertext, err := sealSecret(key, value, ctx)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := openSecret(key, nonce, ciphertext, ctx)
	if err != nil || !bytes.Equal(plain, value) {
		t.Fatal("round trip failed")
	}
	clear(plain)
	for _, changed := range []secretContext{
		{"other", "secret", 1, 1}, {"installation", "other", 1, 1},
		{"installation", "secret", 2, 1}, {"installation", "secret", 1, 2},
	} {
		if plain, err := openSecret(key, nonce, ciphertext, changed); !errors.Is(err, errSecretAuthentication) || plain != nil {
			t.Fatal("context substitution accepted")
		}
	}
	for index := range ciphertext {
		changed := bytes.Clone(ciphertext)
		changed[index] ^= 1
		if plain, err := openSecret(key, nonce, changed, ctx); !errors.Is(err, errSecretAuthentication) || plain != nil {
			t.Fatal("ciphertext tampering accepted")
		}
	}
	for _, badNonce := range [][]byte{nil, nonce[:23], append(bytes.Clone(nonce), 0)} {
		if _, err := openSecret(key, badNonce, ciphertext, ctx); !errors.Is(err, errSecretAuthentication) {
			t.Fatal("malformed nonce accepted")
		}
	}
	if _, err := openSecret(bytes.Repeat([]byte{43}, 32), nonce, ciphertext, ctx); !errors.Is(err, errSecretAuthentication) {
		t.Fatal("wrong key accepted")
	}
	otherNonce, otherCiphertext, err := sealSecret(key, value, ctx)
	if err != nil || bytes.Equal(nonce, otherNonce) || bytes.Equal(ciphertext, otherCiphertext) {
		t.Fatal("encryption did not use fresh randomness")
	}
}

func TestSecretValueByteLimits(t *testing.T) {
	key := make([]byte, 32)
	ctx := secretContext{"installation", "secret", 1, 1}
	for _, n := range []int{0, 7, 16385} {
		if _, _, err := sealSecret(key, bytes.Repeat([]byte{'x'}, n), ctx); !errors.Is(err, errInvalidSecret) {
			t.Fatalf("accepted invalid length %d", n)
		}
	}
	for _, value := range [][]byte{bytes.Repeat([]byte{'x'}, 8), bytes.Repeat([]byte{'x'}, 16384), []byte("😀😀")} {
		nonce, ciphertext, err := sealSecret(key, value, ctx)
		if err != nil {
			t.Fatal(err)
		}
		plain, err := openSecret(key, nonce, ciphertext, ctx)
		if err != nil || !bytes.Equal(plain, value) {
			t.Fatal("boundary round trip failed")
		}
	}
	if _, _, err := sealSecret(key, bytes.Repeat([]byte{255}, 8), ctx); !errors.Is(err, errInvalidSecret) {
		t.Fatal("invalid UTF-8 accepted")
	}
}

func TestFingerprintIsInstallationAndKeySpecific(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	value := []byte("canary-value")
	first, err := secretFingerprint(key, value, "one")
	if err != nil {
		t.Fatal(err)
	}
	same, _ := secretFingerprint(key, value, "one")
	otherInstallation, _ := secretFingerprint(key, value, "two")
	otherKey, _ := secretFingerprint(bytes.Repeat([]byte{2}, 32), value, "one")
	otherValue, _ := secretFingerprint(key, []byte("different-canary"), "one")
	// Independently computed with .NET HMACSHA256 and RFC 5869 extract/expand.
	if first != "mc_fp_9MURSNGD0h_uDamY" || first != same || first == otherInstallation || first == otherKey || first == otherValue {
		t.Fatal("fingerprint isolation failed")
	}
}

func TestSecretContextEncodingAndValidation(t *testing.T) {
	ctx := secretContext{"a", "bc", 1, 2}
	got, err := ctx.associatedData()
	want := append([]byte("modelcairn/secret/v1\x00"), []byte{
		0, 0, 0, 1, 'a', 0, 0, 0, 2, 'b', 'c',
		0, 0, 0, 0, 0, 0, 0, 1,
		0, 0, 0, 0, 0, 0, 0, 2,
	}...)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatal("associated-data encoding changed")
	}
	other, err := (secretContext{"ab", "c", 1, 2}).associatedData()
	if err != nil || bytes.Equal(got, other) {
		t.Fatal("ambiguous identity encoding")
	}
	for _, invalid := range []secretContext{
		{"", "secret", 1, 1}, {"installation", "", 1, 1},
		{string(bytes.Repeat([]byte{'a'}, 129)), "secret", 1, 1},
		{"installation", string(bytes.Repeat([]byte{'a'}, 129)), 1, 1},
		{"installation", "secret", 0, 1}, {"installation", "secret", 1, -1},
	} {
		if data, err := invalid.associatedData(); data != nil || !errors.Is(err, errSecretAuthentication) {
			t.Fatal("invalid context accepted")
		}
	}
}

func TestSecretCipherRejectsMalformedInputs(t *testing.T) {
	ctx := secretContext{"installation", "secret", 1, 1}
	key := make([]byte, 32)
	nonce, ciphertext, err := sealSecret(key, []byte("canary-secret"), ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{0, 31, 33} {
		badKey := make([]byte, size)
		if n, c, err := sealSecret(badKey, []byte("canary-secret"), ctx); n != nil || c != nil || !errors.Is(err, errSecretAuthentication) {
			t.Fatal("invalid encryption key accepted")
		}
		if plain, err := openSecret(badKey, nonce, ciphertext, ctx); plain != nil || !errors.Is(err, errSecretAuthentication) {
			t.Fatal("invalid decryption key accepted")
		}
		if fp, err := secretFingerprint(badKey, []byte("canary-secret"), "installation"); fp != "" || !errors.Is(err, errSecretAuthentication) {
			t.Fatal("invalid fingerprint key accepted")
		}
	}
	for _, size := range []int{0, 23, 16401} {
		if plain, err := openSecret(key, nonce, make([]byte, size), ctx); plain != nil || !errors.Is(err, errSecretAuthentication) {
			t.Fatal("malformed ciphertext accepted")
		}
	}
	changed := bytes.Clone(nonce)
	changed[0] ^= 1
	if plain, err := openSecret(key, changed, ciphertext, ctx); plain != nil || !errors.Is(err, errSecretAuthentication) {
		t.Fatal("nonce tampering accepted")
	}
}

func TestSecretCipherRejectsAuthenticatedInvalidUTF8(t *testing.T) {
	key := make([]byte, 32)
	ctx := secretContext{"installation", "secret", 1, 1}
	ad, err := ctx.associatedData()
	if err != nil {
		t.Fatal(err)
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	ciphertext := aead.Seal(nil, nonce, bytes.Repeat([]byte{255}, 8), ad)
	if plain, err := openSecret(key, nonce, ciphertext, ctx); plain != nil || !errors.Is(err, errSecretAuthentication) {
		t.Fatal("authenticated invalid UTF-8 accepted")
	}
}

func BenchmarkSecretSeal(b *testing.B) {
	for _, size := range []int{32, 16384} {
		name := "small"
		if size == 16384 {
			name = "maximum"
		}
		b.Run(name, func(b *testing.B) {
			key := make([]byte, 32)
			value := bytes.Repeat([]byte{'x'}, size)
			ctx := secretContext{"installation", "secret", 1, 1}
			b.ReportAllocs()
			b.SetBytes(int64(size))
			for b.Loop() {
				if _, _, err := sealSecret(key, value, ctx); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
