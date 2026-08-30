package ferry

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestAccessPasswordVerifierMatchesOnlyExactPassword(t *testing.T) {
	verifier, err := newAccessPasswordVerifierWithRandom("correct horse 🔒", bytes.NewReader(make([]byte, accessPasswordSaltBytes)))
	if err != nil {
		t.Fatal(err)
	}
	if !verifyAccessPassword("correct horse 🔒", verifier) {
		t.Fatal("exact password was rejected")
	}
	for _, password := range []string{"", "correct horse", "Correct horse 🔒", " correct horse 🔒", "correct horse 🔒 "} {
		if verifyAccessPassword(password, verifier) {
			t.Fatalf("accepted %q", password)
		}
	}
}

func TestAccessPasswordValidationAndRandomFailure(t *testing.T) {
	for _, password := range []string{"", strings.Repeat("a", maxAccessPasswordBytes+1), string([]byte{0xff})} {
		if _, err := newAccessPasswordVerifier(password); err == nil {
			t.Fatalf("accepted invalid password %q", password)
		}
	}
	failed := errors.New("entropy failed")
	if _, err := newAccessPasswordVerifierWithRandom("valid", entropyFailReader{failed}); !errors.Is(err, failed) {
		t.Fatalf("random failure = %v", err)
	}
}

func TestDeviceTokenRequiresCanonical256Bits(t *testing.T) {
	token, hash, err := newDeviceToken()
	if err != nil {
		t.Fatal(err)
	}
	parsed, ok := hashDeviceToken(token)
	if !ok || parsed != hash {
		t.Fatal("generated token did not round trip")
	}
	for _, invalid := range []string{"", token + "=", token[:len(token)-1], "not-a-token"} {
		if _, ok := hashDeviceToken(invalid); ok {
			t.Fatalf("accepted token %q", invalid)
		}
	}
}

type entropyFailReader struct{ err error }

func (r entropyFailReader) Read([]byte) (int, error) { return 0, r.err }
