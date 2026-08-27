package service

import (
	"errors"
	"strings"
	"testing"
)

func TestArgon2idPasswordHashRoundTrip(t *testing.T) {
	hash, err := hashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Fatalf("expected argon2id hash, got %q", hash)
	}

	ok, err := verifyPassword("password123", hash)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected password to verify")
	}

	ok, err = verifyPassword("wrong-password", hash)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected wrong password to fail")
	}
}

func TestValidatePasswordRejectsMoreThan128Characters(t *testing.T) {
	if err := validatePassword(strings.Repeat("a", 128)); err != nil {
		t.Fatalf("expected 128 character password to be valid, got %v", err)
	}

	err := validatePassword(strings.Repeat("a", 129))
	if !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("expected ErrInvalidPassword, got %v", err)
	}
}
