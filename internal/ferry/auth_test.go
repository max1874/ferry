package ferry

import (
	"bytes"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPairingCodeIsFourDigitsAndSingleUse(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	manager := newPairingManager(func() time.Time { return now }, bytes.NewReader(make([]byte, pairingCodeBytes)))
	code, err := manager.NewCode("")
	if err != nil {
		t.Fatal(err)
	}
	if code.Code != "0000" || code.ExpiresAt != "2026-08-29T12:10:00Z" {
		t.Fatalf("code = %#v", code)
	}
	if !manager.Consume("0000") {
		t.Fatal("first consume failed")
	}
	if manager.Consume(code.Code) {
		t.Fatal("pairing code was reusable")
	}
}

func TestPairingCodeExpiresAndRejectsEquivalentLookingInput(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 900_000_000, time.UTC)
	manager := newPairingManager(func() time.Time { return now }, bytes.NewReader(make([]byte, pairingCodeBytes)))
	code, err := manager.NewCode("")
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"000", "00000", "000A", "００００", " 0000", "0000\n", "\u20030000\u2003"} {
		if manager.Consume(invalid) {
			t.Fatalf("pairing code accepted %q", invalid)
		}
	}
	if code.ExpiresAt != "2026-08-29T12:10:00Z" {
		t.Fatalf("expires_at = %q", code.ExpiresAt)
	}
	now = time.Date(2026, 8, 29, 12, 10, 0, 0, time.UTC)
	if manager.Consume(code.Code) {
		t.Fatal("pairing code accepted at expiry boundary")
	}
}

func TestConcurrentPairingCodeClaimHasOneWinner(t *testing.T) {
	manager := newPairingManager(time.Now, bytes.NewReader(make([]byte, pairingCodeBytes)))
	code, err := manager.NewCode("")
	if err != nil {
		t.Fatal(err)
	}
	var winners atomic.Int32
	var group sync.WaitGroup
	for range 32 {
		group.Add(1)
		go func() {
			defer group.Done()
			if manager.Consume(code.Code) {
				winners.Add(1)
			}
		}()
	}
	group.Wait()
	if winners.Load() != 1 {
		t.Fatalf("winners = %d", winners.Load())
	}
}

func TestPairingCodeCollisionRetriesInsteadOfAliasing(t *testing.T) {
	random := append(make([]byte, pairingCodeBytes*2), bytes.Repeat([]byte{1}, pairingCodeBytes)...)
	manager := newPairingManager(time.Now, bytes.NewReader(random))
	first, err := manager.NewCode("")
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.NewCode("")
	if err != nil {
		t.Fatal(err)
	}
	if first.Code == second.Code || !manager.Consume(first.Code) || !manager.Consume(second.Code) {
		t.Fatalf("first = %q, second = %q", first.Code, second.Code)
	}
}

func TestPairingCodeRejectsBiasedRandomTail(t *testing.T) {
	random := bytes.NewReader([]byte{0xff, 0xff, 0x00, 0x01})
	manager := newPairingManager(time.Now, random)
	code, err := manager.NewCode("")
	if err != nil {
		t.Fatal(err)
	}
	if code.Code != "0001" {
		t.Fatalf("code = %q", code.Code)
	}
}

func TestPairingCodePreservesUpperFourDigitBoundary(t *testing.T) {
	manager := newPairingManager(time.Now, bytes.NewReader([]byte{0xea, 0x5f})) // 59,999 maps to 9,999.
	code, err := manager.NewCode("")
	if err != nil {
		t.Fatal(err)
	}
	if code.Code != "9999" {
		t.Fatalf("code = %q", code.Code)
	}
}

func TestPairingCodeFailsWhenRandomSourceNeverProducesUsableSample(t *testing.T) {
	manager := newPairingManager(time.Now, bytes.NewReader(bytes.Repeat([]byte{0xff}, pairingCodeBytes*16)))
	if _, err := manager.NewCode(""); err == nil {
		t.Fatal("pairing code generation unexpectedly succeeded")
	}
}

func TestRevokingIssuerInvalidatesItsUnconsumedCodes(t *testing.T) {
	manager := newPairingManager(time.Now, bytes.NewReader(make([]byte, pairingCodeBytes)))
	issuerID := "0123456789abcdef0123456789abcdef"
	code, err := manager.NewCode(issuerID)
	if err != nil {
		t.Fatal(err)
	}
	manager.RevokeIssuer(issuerID)
	if manager.Consume(code.Code) {
		t.Fatal("revoked issuer's pairing code remained usable")
	}
}

func TestPairingReservationRollsBackAfterFailure(t *testing.T) {
	manager := newPairingManager(time.Now, bytes.NewReader(make([]byte, pairingCodeBytes)))
	code, err := manager.NewCode("")
	if err != nil {
		t.Fatal(err)
	}
	reservation, ok := manager.Reserve(code.Code)
	if !ok {
		t.Fatal("could not reserve new code")
	}
	if _, duplicate := manager.Reserve(code.Code); duplicate {
		t.Fatal("reserved code had a concurrent second winner")
	}
	reservation.Rollback()
	if !manager.Consume(code.Code) {
		t.Fatal("rolled-back code was permanently consumed")
	}
}

func TestRevocationWinsOverReservationRollback(t *testing.T) {
	manager := newPairingManager(time.Now, bytes.NewReader(make([]byte, pairingCodeBytes)))
	issuerID := "0123456789abcdef0123456789abcdef"
	code, err := manager.NewCode(issuerID)
	if err != nil {
		t.Fatal(err)
	}
	reservation, ok := manager.Reserve(code.Code)
	if !ok {
		t.Fatal("could not reserve new code")
	}
	manager.RevokeIssuer(issuerID)
	reservation.Rollback()
	if manager.Consume(code.Code) {
		t.Fatal("rollback restored a code whose issuer was revoked")
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
