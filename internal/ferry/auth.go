package ferry

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"
)

const (
	PairingCodeLifetime = 10 * time.Minute
	deviceTokenBytes    = 32
	pairingCodeBytes    = 2
	pairingCodeLimit    = 60_000
	pairingCodeSpace    = 10_000
)

type PairingCode struct {
	Code      string `json:"code"`
	ExpiresAt string `json:"expires_at"`
}

type PairingManager struct {
	mu     sync.Mutex
	codes  map[[sha256.Size]byte]pairingCodeRecord
	now    func() time.Time
	random io.Reader
}

type pairingCodeRecord struct {
	expiresAt time.Time
	issuerID  string
	reserved  bool
}

type PairingReservation struct {
	manager *PairingManager
	hash    [sha256.Size]byte
	record  pairingCodeRecord
}

func NewPairingManager() *PairingManager {
	return newPairingManager(time.Now, rand.Reader)
}

func newPairingManager(now func() time.Time, random io.Reader) *PairingManager {
	return &PairingManager{codes: make(map[[sha256.Size]byte]pairingCodeRecord), now: now, random: random}
}

func (p *PairingManager) NewCode(issuerID string) (PairingCode, error) {
	if issuerID != "" && !validID(issuerID) {
		return PairingCode{}, fmt.Errorf("pairing code issuer is invalid")
	}
	for range 16 {
		raw := make([]byte, pairingCodeBytes)
		if _, err := io.ReadFull(p.random, raw); err != nil {
			return PairingCode{}, fmt.Errorf("generate pairing code: %w", err)
		}
		sample := int(binary.BigEndian.Uint16(raw))
		if sample >= pairingCodeLimit {
			continue
		}
		code := fmt.Sprintf("%04d", sample%pairingCodeSpace)
		expiresAt := p.now().UTC().Add(PairingCodeLifetime).Truncate(time.Second)
		hash := sha256.Sum256([]byte(code))
		p.mu.Lock()
		p.removeExpiredLocked()
		_, exists := p.codes[hash]
		if !exists {
			p.codes[hash] = pairingCodeRecord{expiresAt: expiresAt, issuerID: issuerID}
		}
		p.mu.Unlock()
		if !exists {
			return PairingCode{Code: code, ExpiresAt: expiresAt.Format(time.RFC3339)}, nil
		}
	}
	return PairingCode{}, fmt.Errorf("generate unique pairing code")
}

func (p *PairingManager) Consume(code string) bool {
	reservation, ok := p.Reserve(code)
	if ok {
		reservation.Commit()
	}
	return ok
}

func (p *PairingManager) Reserve(code string) (*PairingReservation, bool) {
	code, ok := normalizePairingCode(code)
	if !ok {
		return nil, false
	}
	hash := sha256.Sum256([]byte(code))
	p.mu.Lock()
	defer p.mu.Unlock()
	record, exists := p.codes[hash]
	if !exists || record.reserved {
		return nil, false
	}
	if !p.now().Before(record.expiresAt) {
		delete(p.codes, hash)
		return nil, false
	}
	record.reserved = true
	p.codes[hash] = record
	return &PairingReservation{manager: p, hash: hash, record: record}, true
}

func (r *PairingReservation) IssuerID() string {
	return r.record.issuerID
}

func (r *PairingReservation) Commit() {
	r.manager.mu.Lock()
	defer r.manager.mu.Unlock()
	if record, exists := r.manager.codes[r.hash]; exists && record == r.record {
		delete(r.manager.codes, r.hash)
	}
}

func (r *PairingReservation) Rollback() {
	r.manager.mu.Lock()
	defer r.manager.mu.Unlock()
	if record, exists := r.manager.codes[r.hash]; exists && record == r.record && r.manager.now().Before(record.expiresAt) {
		record.reserved = false
		r.manager.codes[r.hash] = record
	}
}

func (p *PairingManager) RevokeIssuer(issuerID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for hash, record := range p.codes {
		if record.issuerID == issuerID {
			delete(p.codes, hash)
		}
	}
}

func (p *PairingManager) removeExpiredLocked() {
	now := p.now()
	for hash, record := range p.codes {
		if !now.Before(record.expiresAt) {
			delete(p.codes, hash)
		}
	}
}

func normalizePairingCode(value string) (string, bool) {
	if len(value) != 4 {
		return "", false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return "", false
		}
	}
	return value, true
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
