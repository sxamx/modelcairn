package adminauth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	MinMemoryKiB     = 19456
	MaxMemoryKiB     = 65536
	MinIterations    = 2
	MaxIterations    = 6
	Parallelism      = 1
	SaltBytes        = 16
	HashBytes        = 32
	MaxPasswordBytes = 1024
	MinPasswordRunes = 12
	maxPHCBytes      = 256
)

var (
	ErrInvalidPassword   = errors.New("invalid_password")
	ErrInvalidParameters = errors.New("invalid_password_parameters")
	ErrInvalidPHC        = errors.New("invalid_password_phc")
)

type Parameters struct {
	MemoryKiB  uint32
	Iterations uint32
}

func (p Parameters) Validate() error {
	if p.MemoryKiB < MinMemoryKiB || p.MemoryKiB > MaxMemoryKiB || p.Iterations < MinIterations || p.Iterations > MaxIterations {
		return ErrInvalidParameters
	}
	return nil
}

func ValidatePassword(password []byte) error {
	if len(password) > MaxPasswordBytes || !utf8.Valid(password) || utf8.RuneCount(password) < MinPasswordRunes {
		return ErrInvalidPassword
	}
	return nil
}

func Hash(password []byte, parameters Parameters) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	if err := parameters.Validate(); err != nil {
		return "", err
	}
	salt := make([]byte, SaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", errors.New("password_random_unavailable")
	}
	digest := argon2.IDKey(password, salt, parameters.Iterations, parameters.MemoryKiB, Parallelism, HashBytes)
	defer clear(digest)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, parameters.MemoryKiB, parameters.Iterations, Parallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(digest)), nil
}

// Verify validates every PHC bound before invoking Argon2id, preventing corrupt
// persisted data from requesting excessive memory or CPU.
func Verify(password []byte, encoded string) (bool, error) {
	if err := ValidatePassword(password); err != nil {
		return false, err
	}
	p, salt, expected, err := ParsePHC(encoded)
	if err != nil {
		return false, err
	}
	digest := argon2.IDKey(password, salt, p.Iterations, p.MemoryKiB, Parallelism, HashBytes)
	defer clear(digest)
	return subtle.ConstantTimeCompare(digest, expected) == 1, nil
}

func ParsePHC(encoded string) (Parameters, []byte, []byte, error) {
	if len(encoded) == 0 || len(encoded) > maxPHCBytes {
		return Parameters{}, nil, nil, ErrInvalidPHC
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return Parameters{}, nil, nil, ErrInvalidPHC
	}
	params := strings.Split(parts[3], ",")
	if len(params) != 3 || !strings.HasPrefix(params[0], "m=") || !strings.HasPrefix(params[1], "t=") || params[2] != "p=1" {
		return Parameters{}, nil, nil, ErrInvalidPHC
	}
	memory, e1 := strconv.ParseUint(strings.TrimPrefix(params[0], "m="), 10, 32)
	iterations, e2 := strconv.ParseUint(strings.TrimPrefix(params[1], "t="), 10, 32)
	p := Parameters{MemoryKiB: uint32(memory), Iterations: uint32(iterations)}
	if e1 != nil || e2 != nil || p.Validate() != nil {
		return Parameters{}, nil, nil, ErrInvalidPHC
	}
	salt, e1 := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	digest, e2 := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if e1 != nil || e2 != nil || len(salt) != SaltBytes || len(digest) != HashBytes {
		return Parameters{}, nil, nil, ErrInvalidPHC
	}
	canonical := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=1$%s$%s", p.MemoryKiB, p.Iterations,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(digest))
	if canonical != encoded {
		return Parameters{}, nil, nil, ErrInvalidPHC
	}
	return p, salt, digest, nil
}
