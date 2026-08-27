package service

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argon2Memory      = 64 * 1024
	argon2Iterations  = 3
	argon2Parallelism = 2
	argon2SaltLength  = 16
	argon2KeyLength   = 32
)

type argon2Params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

func hashPassword(password string) (string, error) {
	params := argon2Params{
		memory:      argon2Memory,
		iterations:  argon2Iterations,
		parallelism: argon2Parallelism,
		saltLength:  argon2SaltLength,
		keyLength:   argon2KeyLength,
	}

	salt := make([]byte, params.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, params.iterations, params.memory, params.parallelism, params.keyLength)
	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		params.memory,
		params.iterations,
		params.parallelism,
		encodedSalt,
		encodedHash,
	), nil
}

func verifyPassword(password string, encodedHash string) (bool, error) {
	params, salt, expectedHash, err := decodeHash(encodedHash)
	if err != nil {
		return false, err
	}

	hash := argon2.IDKey([]byte(password), salt, params.iterations, params.memory, params.parallelism, params.keyLength)
	return subtle.ConstantTimeCompare(hash, expectedHash) == 1, nil
}

func decodeHash(encodedHash string) (argon2Params, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return argon2Params{}, nil, nil, errors.New("invalid password hash format")
	}

	params, err := decodeParams(parts[3])
	if err != nil {
		return argon2Params{}, nil, nil, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argon2Params{}, nil, nil, fmt.Errorf("decode salt: %w", err)
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return argon2Params{}, nil, nil, fmt.Errorf("decode hash: %w", err)
	}

	params.saltLength = uint32(len(salt))
	params.keyLength = uint32(len(hash))

	return params, salt, hash, nil
}

func decodeParams(encodedParams string) (argon2Params, error) {
	values := map[string]string{}
	for _, field := range strings.Split(encodedParams, ",") {
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			return argon2Params{}, errors.New("invalid argon2 params")
		}
		values[key] = value
	}

	memory, err := parseUint32(values["m"])
	if err != nil {
		return argon2Params{}, err
	}
	iterations, err := parseUint32(values["t"])
	if err != nil {
		return argon2Params{}, err
	}
	parallelism, err := parseUint8(values["p"])
	if err != nil {
		return argon2Params{}, err
	}
	if memory == 0 || iterations == 0 || parallelism == 0 {
		return argon2Params{}, errors.New("invalid argon2 params")
	}

	return argon2Params{
		memory:      memory,
		iterations:  iterations,
		parallelism: parallelism,
	}, nil
}

func parseUint32(value string) (uint32, error) {
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse argon2 param: %w", err)
	}

	return uint32(parsed), nil
}

func parseUint8(value string) (uint8, error) {
	parsed, err := strconv.ParseUint(value, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("parse argon2 param: %w", err)
	}

	return uint8(parsed), nil
}
