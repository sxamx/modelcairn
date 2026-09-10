package adminauth

import (
	"errors"
	"strings"
	"testing"
)

func TestPasswordHashAndVerify(t *testing.T) {
	p := []byte("contraseña segura 2026")
	encoded, err := Hash(p, Parameters{MemoryKiB: MinMemoryKiB, Iterations: MinIterations})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encoded, string(p)) {
		t.Fatal("PHC contains password")
	}
	ok, err := Verify(p, encoded)
	if err != nil || !ok {
		t.Fatalf("verify=%v err=%v", ok, err)
	}
	ok, err = Verify([]byte("contraseña errónea"), encoded)
	if err != nil || ok {
		t.Fatalf("wrong verify=%v err=%v", ok, err)
	}
	if encoded2, err := Hash(p, Parameters{MemoryKiB: MinMemoryKiB, Iterations: MinIterations}); err != nil || encoded2 == encoded {
		t.Fatal("hashes do not use unique random salts")
	}
}

func TestPasswordAndParameterBounds(t *testing.T) {
	invalid := [][]byte{[]byte("short"), append([]byte("valid-prefix"), 0xff), []byte(strings.Repeat("a", MaxPasswordBytes+1))}
	for _, password := range invalid {
		if err := ValidatePassword(password); !errors.Is(err, ErrInvalidPassword) {
			t.Fatalf("accepted invalid password: %v", err)
		}
	}
	if err := ValidatePassword([]byte(strings.Repeat("界", MinPasswordRunes))); err != nil {
		t.Fatal(err)
	}
	for _, p := range []Parameters{{MinMemoryKiB - 1, 2}, {MaxMemoryKiB + 1, 2}, {MinMemoryKiB, 1}, {MinMemoryKiB, 7}} {
		if err := p.Validate(); !errors.Is(err, ErrInvalidParameters) {
			t.Fatalf("parameters accepted: %+v", p)
		}
	}
}

func TestPHCRejectedBeforeDerivation(t *testing.T) {
	invalid := []string{
		"", "$argon2id$v=19$m=999999,t=2,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"$argon2id$v=19$m=19456,t=2,p=2$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"$argon2i$v=19$m=19456,t=2,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		strings.Repeat("x", maxPHCBytes+1),
	}
	for _, encoded := range invalid {
		if _, err := Verify([]byte("irrelevant password"), encoded); !errors.Is(err, ErrInvalidPHC) {
			t.Fatalf("PHC accepted: %v", err)
		}
	}
}
