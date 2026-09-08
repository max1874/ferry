// Package lantls manages the TLS material a self-hosted Ferry server presents
// on a local network.
//
// Browsers expose the clipboard API only to secure contexts, and http:// on a
// LAN address is never one, so clipboard sync through the web UI requires
// HTTPS. No public CA issues certificates for 192.168.x.x, so Ferry keeps a
// local CA in its data directory and signs a leaf certificate for the addresses
// it answers on. A device trusts the CA once; the leaf is re-signed whenever
// the address set changes, so a new DHCP lease does not cost the user another
// round of trusting certificates on every device.
//
// The CA carries critical name constraints and a serverAuth extended key usage,
// so trusting it does not also grant the key the power to impersonate a public
// site to that device. Its private key is still a secret: anyone holding it can
// impersonate any private address to every device that trusted it.
package lantls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	caValidity = 10 * 365 * 24 * time.Hour
	// Apple refuses TLS server certificates valid for longer than 398 days.
	// The limit is documented for certificates chaining to a system root, but
	// staying under it costs nothing and removes a platform-specific failure.
	leafValidity = 398 * 24 * time.Hour
	// Certificates are only examined at startup, so renew early enough that a
	// server restarted at any point within the window keeps serving.
	renewWindow = 30 * 24 * time.Hour
)

const (
	caCertFile   = "ca.crt"
	caKeyFile    = "ca.key"
	leafCertFile = "server.crt"
	leafKeyFile  = "server.key"
)

// Material is the TLS material a Ferry listener serves with.
type Material struct {
	// Certificate is the leaf certificate and its private key.
	Certificate tls.Certificate
	// CAPEM is the CA certificate in PEM form, for devices to trust. It is
	// empty when the material came from an operator-supplied certificate.
	CAPEM []byte
	// CAFingerprint is the SHA-256 digest of the CA certificate, so a user can
	// compare what a device offers to trust against what the server logged.
	CAFingerprint string
	// CAIssued reports that this call minted a new CA. Every device that
	// trusted the previous one has to trust this one instead, so the caller
	// must say so rather than let the devices fail later without explanation.
	CAIssued bool
}

// LoadPair reads an operator-supplied certificate and key. Ferry never manages
// or renews these; the operator owns their lifecycle.
func LoadPair(certFile, keyFile string) (*Material, error) {
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("lantls: load certificate: %w", err)
	}
	return &Material{Certificate: certificate}, nil
}

// Ensure returns TLS material from dir that covers every host, creating the CA
// and re-signing the leaf certificate as needed. Hosts may be DNS names or IP
// addresses.
func Ensure(dir string, hosts []string) (*Material, error) {
	dnsNames, ips := splitHosts(hosts)
	if len(dnsNames) == 0 && len(ips) == 0 {
		return nil, errors.New("lantls: at least one host is required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("lantls: create %s: %w", dir, err)
	}
	now := time.Now()
	ca, issued, err := ensureCA(dir, now)
	if err != nil {
		return nil, err
	}
	certificate, err := ensureLeaf(dir, ca, dnsNames, ips, now)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(ca.certificate.Raw)
	return &Material{
		Certificate:   *certificate,
		CAPEM:         ca.certificatePEM,
		CAFingerprint: formatFingerprint(digest[:]),
		CAIssued:      issued,
	}, nil
}

type authority struct {
	certificate    *x509.Certificate
	certificatePEM []byte
	key            *ecdsa.PrivateKey
}

func ensureCA(dir string, now time.Time) (*authority, bool, error) {
	existing, err := readCA(dir)
	if err != nil {
		return nil, false, err
	}
	if existing != nil && usable(existing.certificate, now) {
		return existing, false, nil
	}
	created, err := writeCA(dir, now)
	if err != nil {
		return nil, false, err
	}
	return created, true, nil
}

// readCA loads the stored CA. Missing files yield a nil authority so the caller
// creates one. Unreadable or malformed files are an error rather than a silent
// regeneration: replacing a CA invalidates the trust the user established on
// every device, and that must not happen without them noticing.
func readCA(dir string) (*authority, error) {
	certificatePEM, err := os.ReadFile(filepath.Join(dir, caCertFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lantls: read CA certificate: %w", err)
	}
	keyPEM, err := os.ReadFile(filepath.Join(dir, caKeyFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("lantls: %s exists without %s; remove %s and restart to issue a new CA", caCertFile, caKeyFile, dir)
	}
	if err != nil {
		return nil, fmt.Errorf("lantls: read CA key: %w", err)
	}
	certificate, err := parseCertificate(certificatePEM)
	if err != nil {
		return nil, fmt.Errorf("lantls: parse CA certificate: %w", err)
	}
	key, err := parseKey(keyPEM)
	if err != nil {
		return nil, fmt.Errorf("lantls: parse CA key: %w", err)
	}
	if !certificate.IsCA {
		return nil, fmt.Errorf("lantls: %s is not a CA certificate", caCertFile)
	}
	return &authority{certificate: certificate, certificatePEM: certificatePEM, key: key}, nil
}

func writeCA(dir string, now time.Time) (*authority, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("lantls: generate CA key: %w", err)
	}
	serial, err := newSerial()
	if err != nil {
		return nil, err
	}
	// Backdate slightly so a device whose clock trails the server still accepts
	// the certificate, and derive the expiry from that start so the validity
	// span is exactly caValidity rather than caValidity plus the backdating.
	notBefore := now.Add(-time.Hour)
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"Ferry"},
			CommonName:   "Ferry local CA",
		},
		NotBefore:             notBefore,
		NotAfter:              notBefore.Add(caValidity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
		// Constrain the CA to the addresses Ferry can legitimately answer on.
		// Critical, per RFC 5280: a client that cannot evaluate the constraint
		// must reject the certificate rather than trust it without limits.
		PermittedDNSDomainsCritical: true,
		PermittedDNSDomains:         []string{"localhost"},
		PermittedIPRanges:           constrainedIPRanges(),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("lantls: create CA certificate: %w", err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("lantls: parse new CA certificate: %w", err)
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := writeMaterial(dir, caCertFile, certificatePEM, 0o644); err != nil {
		return nil, err
	}
	keyPEM, err := encodeKey(key)
	if err != nil {
		return nil, err
	}
	if err := writeMaterial(dir, caKeyFile, keyPEM, 0o600); err != nil {
		return nil, err
	}
	return &authority{certificate: certificate, certificatePEM: certificatePEM, key: key}, nil
}

func ensureLeaf(dir string, ca *authority, dnsNames []string, ips []net.IP, now time.Time) (*tls.Certificate, error) {
	existing, err := readLeaf(dir)
	if err != nil {
		return nil, err
	}
	if existing != nil && leafCovers(existing.Leaf, ca, dnsNames, ips, now) {
		return existing, nil
	}
	return writeLeaf(dir, ca, dnsNames, ips, now)
}

// readLeaf loads the stored leaf certificate. Unlike the CA, a leaf that is
// missing, malformed, or mismatched is re-signed without complaint: it carries
// no trust of its own, so replacing it costs the user nothing.
func readLeaf(dir string) (*tls.Certificate, error) {
	certificatePEM, err := os.ReadFile(filepath.Join(dir, leafCertFile))
	if err != nil {
		return nil, nil
	}
	keyPEM, err := os.ReadFile(filepath.Join(dir, leafKeyFile))
	if err != nil {
		return nil, nil
	}
	certificate, err := tls.X509KeyPair(certificatePEM, keyPEM)
	if err != nil {
		return nil, nil
	}
	leaf, err := parseCertificate(certificatePEM)
	if err != nil {
		return nil, nil
	}
	certificate.Leaf = leaf
	return &certificate, nil
}

func writeLeaf(dir string, ca *authority, dnsNames []string, ips []net.IP, now time.Time) (*tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("lantls: generate certificate key: %w", err)
	}
	serial, err := newSerial()
	if err != nil {
		return nil, err
	}
	// The span must stay at or under leafValidity: Apple rejects TLS server
	// certificates that live longer than 398 days, and backdating the start
	// without moving the expiry would push it an hour past the ceiling.
	notBefore := now.Add(-time.Hour)
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"Ferry"},
			CommonName:   "Ferry",
		},
		NotBefore:             notBefore,
		NotAfter:              notBefore.Add(leafValidity),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		IPAddresses:           ips,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.certificate, &key.PublicKey, ca.key)
	if err != nil {
		return nil, fmt.Errorf("lantls: create certificate: %w", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("lantls: parse new certificate: %w", err)
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := writeMaterial(dir, leafCertFile, certificatePEM, 0o644); err != nil {
		return nil, err
	}
	keyPEM, err := encodeKey(key)
	if err != nil {
		return nil, err
	}
	if err := writeMaterial(dir, leafKeyFile, keyPEM, 0o600); err != nil {
		return nil, err
	}
	// Only the leaf goes on the wire. The CA is a root: a client either has it
	// in its trust store or the handshake fails, and sending it would make a
	// fresh start present a different chain than every later start.
	return &tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  key,
		Leaf:        leaf,
	}, nil
}

func leafCovers(leaf *x509.Certificate, ca *authority, dnsNames []string, ips []net.IP, now time.Time) bool {
	if leaf == nil || !usable(leaf, now) {
		return false
	}
	if err := leaf.CheckSignatureFrom(ca.certificate); err != nil {
		return false
	}
	for _, name := range dnsNames {
		if !slices.Contains(leaf.DNSNames, name) {
			return false
		}
	}
	for _, ip := range ips {
		if !slices.ContainsFunc(leaf.IPAddresses, ip.Equal) {
			return false
		}
	}
	return true
}

func usable(certificate *x509.Certificate, now time.Time) bool {
	return !now.Before(certificate.NotBefore) && now.Before(certificate.NotAfter.Add(-renewWindow))
}

// constrainedIPRanges is the address space a Ferry CA may certify: loopback and
// the private and link-local ranges a LAN server actually lives in.
func constrainedIPRanges() []*net.IPNet {
	ranges := []string{
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
	permitted := make([]*net.IPNet, 0, len(ranges))
	for _, entry := range ranges {
		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			panic("lantls: malformed constrained range " + entry)
		}
		permitted = append(permitted, network)
	}
	return permitted
}

func splitHosts(hosts []string) ([]string, []net.IP) {
	var dnsNames []string
	var ips []net.IP
	for _, host := range hosts {
		host = strings.TrimSpace(strings.Trim(host, "[]"))
		if host == "" {
			continue
		}
		if ip := net.ParseIP(host); ip != nil {
			if !slices.ContainsFunc(ips, ip.Equal) {
				ips = append(ips, ip)
			}
			continue
		}
		host = strings.ToLower(host)
		if !slices.Contains(dnsNames, host) {
			dnsNames = append(dnsNames, host)
		}
	}
	return dnsNames, ips
}

func newSerial() (*big.Int, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("lantls: generate serial number: %w", err)
	}
	return serial, nil
}

func parseCertificate(certificatePEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certificatePEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("no CERTIFICATE block")
	}
	return x509.ParseCertificate(block.Bytes)
}

func parseKey(keyPEM []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(keyPEM)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, errors.New("no PRIVATE KEY block")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("unexpected key type %T", parsed)
	}
	return key, nil
}

func encodeKey(key *ecdsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("lantls: encode key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// writeMaterial replaces a file atomically so a crash mid-write cannot leave a
// half-written certificate that the next start would refuse to parse.
func writeMaterial(dir, name string, content []byte, mode os.FileMode) error {
	target := filepath.Join(dir, name)
	temporary, err := os.CreateTemp(dir, name+".*")
	if err != nil {
		return fmt.Errorf("lantls: create %s: %w", name, err)
	}
	defer os.Remove(temporary.Name())
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return fmt.Errorf("lantls: set mode on %s: %w", name, err)
	}
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		return fmt.Errorf("lantls: write %s: %w", name, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("lantls: close %s: %w", name, err)
	}
	if err := os.Rename(temporary.Name(), target); err != nil {
		return fmt.Errorf("lantls: replace %s: %w", name, err)
	}
	return nil
}

func formatFingerprint(digest []byte) string {
	encoded := strings.ToUpper(hex.EncodeToString(digest))
	var builder strings.Builder
	for i := 0; i < len(encoded); i += 2 {
		if i > 0 {
			builder.WriteByte(':')
		}
		builder.WriteString(encoded[i : i+2])
	}
	return builder.String()
}
