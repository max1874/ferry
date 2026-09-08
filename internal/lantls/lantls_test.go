package lantls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestEnsureIssuesAConstrainedChainForTheRequestedHosts(t *testing.T) {
	dir := t.TempDir()
	material, err := Ensure(dir, []string{"localhost", "127.0.0.1", "192.168.1.20"})
	if err != nil {
		t.Fatal(err)
	}
	leaf := material.Certificate.Leaf
	if leaf == nil {
		t.Fatal("material carries no parsed leaf certificate")
	}
	for _, name := range []string{"localhost"} {
		if !contains(leaf.DNSNames, name) {
			t.Fatalf("leaf DNS names = %v, missing %q", leaf.DNSNames, name)
		}
	}
	for _, address := range []string{"127.0.0.1", "192.168.1.20"} {
		if !containsIP(leaf.IPAddresses, net.ParseIP(address)) {
			t.Fatalf("leaf IP addresses = %v, missing %s", leaf.IPAddresses, address)
		}
	}
	if len(leaf.ExtKeyUsage) != 1 || leaf.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth {
		t.Fatalf("leaf extended key usage = %v", leaf.ExtKeyUsage)
	}
	// Apple rejects TLS server certificates that live longer than 398 days.
	if span := leaf.NotAfter.Sub(leaf.NotBefore); span > 398*24*time.Hour {
		t.Fatalf("leaf validity = %s, longer than the 398 day platform ceiling", span)
	}
	if material.CAFingerprint == "" {
		t.Fatal("material carries no CA fingerprint")
	}
	if len(material.CAPEM) == 0 {
		t.Fatal("material carries no CA certificate to trust")
	}
}

// A device that trusts the Ferry CA must not thereby trust it to vouch for the
// public internet. The CA is the only thing a user installs, so the limit has
// to hold on the CA itself, not on the leaf Ferry happens to issue today.
func TestCAcannotCertifyPublicAddresses(t *testing.T) {
	dir := t.TempDir()
	material, err := Ensure(dir, []string{"192.168.1.20"})
	if err != nil {
		t.Fatal(err)
	}
	ca := readAuthority(t, dir)
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(material.CAPEM) {
		t.Fatal("CA certificate PEM did not parse")
	}

	if _, err := material.Certificate.Leaf.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err != nil {
		t.Fatalf("legitimate private leaf did not verify: %v", err)
	}

	forgeries := []struct {
		name     string
		dnsNames []string
		ips      []net.IP
	}{
		{name: "public domain", dnsNames: []string{"example.com"}},
		{name: "public address", ips: []net.IP{net.ParseIP("8.8.8.8")}},
		{name: "private name outside localhost", dnsNames: []string{"ferry.internal"}},
	}
	for _, forgery := range forgeries {
		t.Run(forgery.name, func(t *testing.T) {
			forged := signWithAuthority(t, ca, forgery.dnsNames, forgery.ips)
			if _, err := forged.Verify(x509.VerifyOptions{
				Roots:     pool,
				KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			}); err == nil {
				t.Fatalf("CA certified %v / %v outside its name constraints", forgery.dnsNames, forgery.ips)
			}
		})
	}
}

func TestEnsureReusesMaterialAcrossRestarts(t *testing.T) {
	dir := t.TempDir()
	hosts := []string{"localhost", "127.0.0.1", "192.168.1.20"}
	first, err := Ensure(dir, hosts)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Ensure(dir, hosts)
	if err != nil {
		t.Fatal(err)
	}
	if first.Certificate.Leaf.SerialNumber.Cmp(second.Certificate.Leaf.SerialNumber) != 0 {
		t.Fatal("restart re-signed a leaf certificate that already covered every host")
	}
	if first.CAFingerprint != second.CAFingerprint {
		t.Fatal("restart replaced the CA the user already trusted")
	}
	// Trust is only ever established by hand, so a new CA has to be announced
	// and an unchanged one must not be.
	if !first.CAIssued {
		t.Fatal("the first start did not report that it issued a CA")
	}
	if second.CAIssued {
		t.Fatal("a restart reported issuing a CA it actually reused")
	}
}

// A new DHCP lease must cost a new leaf certificate but never a new CA: the
// user trusted the CA on every device, and that work must not be repeated.
func TestEnsureResignsLeafForNewAddressAndKeepsCA(t *testing.T) {
	dir := t.TempDir()
	first, err := Ensure(dir, []string{"192.168.1.20"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Ensure(dir, []string{"192.168.1.77"})
	if err != nil {
		t.Fatal(err)
	}
	if first.CAFingerprint != second.CAFingerprint {
		t.Fatal("address change replaced the CA instead of re-signing the leaf")
	}
	if first.Certificate.Leaf.SerialNumber.Cmp(second.Certificate.Leaf.SerialNumber) == 0 {
		t.Fatal("address change reused a leaf certificate that does not cover the new address")
	}
	if !containsIP(second.Certificate.Leaf.IPAddresses, net.ParseIP("192.168.1.77")) {
		t.Fatalf("re-signed leaf addresses = %v", second.Certificate.Leaf.IPAddresses)
	}
}

// Replacing a CA silently would invalidate the trust the user established on
// every device while the server kept reporting success.
func TestEnsureRefusesToReplaceAnUnreadableCA(t *testing.T) {
	dir := t.TempDir()
	if _, err := Ensure(dir, []string{"192.168.1.20"}); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(filepath.Join(dir, caCertFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, caCertFile), []byte("not a certificate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Ensure(dir, []string{"192.168.1.20"}); err == nil {
		t.Fatal("Ensure() accepted a corrupt CA certificate")
	}
	current, err := os.ReadFile(filepath.Join(dir, caCertFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(current) == string(original) {
		t.Fatal("test did not actually corrupt the CA certificate")
	}

	if err := os.Remove(filepath.Join(dir, caKeyFile)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, caCertFile), original, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Ensure(dir, []string{"192.168.1.20"}); err == nil {
		t.Fatal("Ensure() accepted a CA certificate whose private key is gone")
	}
}

// A leaf carries no trust of its own, so an unusable one is re-signed rather
// than reported as a failure the user has to resolve.
func TestEnsureResignsAnUnreadableLeaf(t *testing.T) {
	dir := t.TempDir()
	first, err := Ensure(dir, []string{"192.168.1.20"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, leafCertFile), []byte("not a certificate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := Ensure(dir, []string{"192.168.1.20"})
	if err != nil {
		t.Fatalf("Ensure() refused to re-sign a corrupt leaf: %v", err)
	}
	if first.CAFingerprint != second.CAFingerprint {
		t.Fatal("re-signing a leaf replaced the CA")
	}
}

func TestEnsureKeepsPrivateKeysUnreadableToOtherUsers(t *testing.T) {
	dir := t.TempDir()
	if _, err := Ensure(dir, []string{"192.168.1.20"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{caKeyFile, leafKeyFile} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if mode := info.Mode().Perm(); mode&0o077 != 0 {
			t.Fatalf("%s mode = %04o, readable outside the owner", name, mode)
		}
	}
}

func TestEnsureRequiresAtLeastOneHost(t *testing.T) {
	if _, err := Ensure(t.TempDir(), nil); err == nil {
		t.Fatal("Ensure() accepted an empty host list")
	}
	if _, err := Ensure(t.TempDir(), []string{"", "  "}); err == nil {
		t.Fatal("Ensure() accepted only blank hosts")
	}
}

// The point of all of this is one property: a client holding the CA completes a
// real handshake against the address it dialled, with no exception granted.
func TestMaterialServesAVerifiedHandshake(t *testing.T) {
	dir := t.TempDir()
	material, err := Ensure(dir, []string{"127.0.0.1", "localhost"})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ferry"))
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{material.Certificate}, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	defer server.Close()

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(material.CAPEM) {
		t.Fatal("CA certificate PEM did not parse")
	}
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}}
	response, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("client trusting the Ferry CA failed the handshake: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ferry" {
		t.Fatalf("body = %q", body)
	}

	untrusted := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}}
	if _, err := untrusted.Get(server.URL); err == nil {
		t.Fatal("a client that does not trust the Ferry CA completed the handshake")
	}
}

func TestLoadPairReadsOperatorSuppliedMaterial(t *testing.T) {
	dir := t.TempDir()
	if _, err := Ensure(dir, []string{"127.0.0.1"}); err != nil {
		t.Fatal(err)
	}
	material, err := LoadPair(filepath.Join(dir, leafCertFile), filepath.Join(dir, leafKeyFile))
	if err != nil {
		t.Fatal(err)
	}
	if len(material.CAPEM) != 0 {
		t.Fatal("operator-supplied material offered a CA for devices to trust")
	}
	if _, err := LoadPair(filepath.Join(dir, leafCertFile), filepath.Join(dir, caKeyFile)); err == nil {
		t.Fatal("LoadPair() accepted a certificate and key that do not match")
	}
}

func readAuthority(t *testing.T, dir string) *authority {
	t.Helper()
	ca, err := readCA(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ca == nil {
		t.Fatal("no CA in the material directory")
	}
	return ca
}

// signWithAuthority mints a certificate the CA would never issue itself, which
// is exactly what an attacker holding the CA key would try.
func signWithAuthority(t *testing.T, ca *authority, dnsNames []string, ips []net.IP) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serial, err := newSerial()
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "forged"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		IPAddresses:           ips,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.certificate, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return certificate
}

func contains(values []string, want string) bool {
	return slices.Contains(values, want)
}

func containsIP(values []net.IP, want net.IP) bool {
	return slices.ContainsFunc(values, want.Equal)
}
