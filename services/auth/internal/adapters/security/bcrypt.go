package security

import (
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/jobtrack-cst/jobtrack/services/auth/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/ports"
	"golang.org/x/crypto/bcrypt"
)

// Bcrypt hashes passwords with golang.org/x/crypto/bcrypt (decision log, 5 Nov 2026).
type Bcrypt struct {
	cost int
	// dummyHash lets Compare("") cost the same as a real check.
	dummyHash []byte
}

var _ ports.PasswordHasher = (*Bcrypt)(nil)

// NewBcrypt uses bcrypt.DefaultCost in production; tests pass bcrypt.MinCost for speed.
func NewBcrypt(cost int) (*Bcrypt, error) {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return nil, errors.New("bcrypt cost out of range")
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return nil, errors.New("generate dummy password failed")
	}
	dummy, err := bcrypt.GenerateFromPassword([]byte(hex.EncodeToString(random)), cost)
	if err != nil {
		return nil, errors.New("generate dummy hash failed")
	}
	return &Bcrypt{cost: cost, dummyHash: dummy}, nil
}

func (b *Bcrypt) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), b.cost)
	if err != nil {
		return "", errors.New("hash password failed")
	}
	return string(hash), nil
}

// Compare treats an empty hash as "user not found": it still runs bcrypt
// against the dummy hash, so response time does not reveal registered emails.
func (b *Bcrypt) Compare(hash, password string) error {
	if hash == "" {
		_ = bcrypt.CompareHashAndPassword(b.dummyHash, []byte(password))
		return domain.ErrInvalidCredentials
	}
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return domain.ErrInvalidCredentials
	}
	if err != nil {
		// A malformed stored hash is a data problem, not a wrong password.
		return errors.New("compare password failed")
	}
	return nil
}
