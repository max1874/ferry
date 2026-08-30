package ferry

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"unicode/utf8"
)

const (
	deviceTokenBytes         = 32
	accessPasswordSaltBytes  = 16
	accessPasswordHashBytes  = 32
	accessPasswordIterations = 210_000
	maxAccessPasswordBytes   = 256
)

type AccessPasswordVerifier struct {
	Salt       []byte
	Hash       []byte
	Iterations int
}

func newAccessPasswordVerifier(password string) (AccessPasswordVerifier, error) {
	return newAccessPasswordVerifierWithRandom(password, rand.Reader)
}

func newAccessPasswordVerifierWithRandom(password string, random io.Reader) (AccessPasswordVerifier, error) {
	if !validAccessPassword(password) {
		return AccessPasswordVerifier{}, fmt.Errorf("password must be valid UTF-8 and at most %d bytes", maxAccessPasswordBytes)
	}
	salt := make([]byte, accessPasswordSaltBytes)
	if _, err := io.ReadFull(random, salt); err != nil {
		return AccessPasswordVerifier{}, fmt.Errorf("generate password salt: %w", err)
	}
	hash, err := pbkdf2.Key(sha256.New, password, salt, accessPasswordIterations, accessPasswordHashBytes)
	if err != nil {
		return AccessPasswordVerifier{}, fmt.Errorf("derive password verifier: %w", err)
	}
	return AccessPasswordVerifier{Salt: salt, Hash: hash, Iterations: accessPasswordIterations}, nil
}

func verifyAccessPassword(password string, verifier AccessPasswordVerifier) bool {
	if !validAccessPassword(password) || len(verifier.Salt) != accessPasswordSaltBytes ||
		len(verifier.Hash) != accessPasswordHashBytes || verifier.Iterations <= 0 {
		return false
	}
	hash, err := pbkdf2.Key(sha256.New, password, verifier.Salt, verifier.Iterations, len(verifier.Hash))
	return err == nil && subtle.ConstantTimeCompare(hash, verifier.Hash) == 1
}

func validAccessPassword(password string) bool {
	return password != "" && len(password) <= maxAccessPasswordBytes && utf8.ValidString(password)
}

func newDeviceToken() (string, [sha256.Size]byte, error) {
	raw := make([]byte, deviceTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", [sha256.Size]byte{}, fmt.Errorf("generate device token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, sha256.Sum256([]byte(token)), nil
}

func hashDeviceToken(token string) ([sha256.Size]byte, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != deviceTokenBytes || base64.RawURLEncoding.EncodeToString(raw) != token {
		return [sha256.Size]byte{}, false
	}
	return sha256.Sum256([]byte(token)), true
}
