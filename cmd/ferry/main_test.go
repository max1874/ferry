package main

import (
	"context"
	"net"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestDefaultConfigBindsLoopback(t *testing.T) {
	value, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if value.listen != "127.0.0.1:8080" {
		t.Fatalf("default listen = %q", value.listen)
	}
	if value.dataDir != "./ferry-data" {
		t.Fatalf("default data dir = %q", value.dataDir)
	}
	if value.lan {
		t.Fatal("LAN mode was enabled by default")
	}
}

func TestConfigRequiresExplicitLANModeAndPrivateAddress(t *testing.T) {
	for _, address := range []string{"10.0.0.13:8080", "192.168.1.20:8080", "169.254.1.2:8080", "[fd00::1]:8080"} {
		t.Run("accept "+address, func(t *testing.T) {
			value, err := parseConfig([]string{"-lan", "-listen", address})
			if err != nil || !value.lan || value.listen != address {
				t.Fatalf("config = %#v, error = %v", value, err)
			}
		})
	}
	for _, address := range []string{"10.0.0.13:8080", "0.0.0.0:8080"} {
		if _, err := parseConfig([]string{"-listen", address}); err == nil {
			t.Fatalf("accepted %q without -lan", address)
		}
	}
	for _, address := range []string{"0.0.0.0:8080", "[::]:8080", "8.8.8.8:8080", "example.com:8080", ":8080"} {
		if _, err := parseConfig([]string{"-lan", "-listen", address}); err == nil {
			t.Fatalf("accepted non-private LAN address %q", address)
		}
	}
}

func TestConfigValidatesPublishedHostAgainstListenerBoundary(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "10.0.0.2", "169.254.1.2", "fd00::1"} {
		arguments := []string{"-lan", "-listen", "10.0.0.13:42817", "-published-host", host}
		if _, err := parseConfig(arguments); err != nil {
			t.Fatalf("rejected published host %q: %v", host, err)
		}
	}
	for _, host := range []string{"0.0.0.0", "::", "8.8.8.8", "203.0.113.10", "example.com"} {
		arguments := []string{"-lan", "-listen", "10.0.0.13:42817", "-published-host", host}
		if _, err := parseConfig(arguments); err == nil {
			t.Fatalf("accepted published host %q", host)
		}
	}
	if _, err := parseConfig([]string{"-published-host", "10.0.0.2"}); err == nil {
		t.Fatal("accepted private published host without LAN mode")
	}
}

func TestConfigRejectsUnexpectedArguments(t *testing.T) {
	if _, err := parseConfig([]string{"extra"}); err == nil {
		t.Fatal("parseConfig() accepted positional argument")
	}
}

func TestConfigAcceptsOnlyLoopbackListeners(t *testing.T) {
	for _, address := range []string{"localhost:8080", "LOCALHOST:8080", "127.0.0.2:8080", "[::1]:8080"} {
		t.Run("accept "+address, func(t *testing.T) {
			value, err := parseConfig([]string{"-listen", address})
			if err != nil {
				t.Fatal(err)
			}
			if value.listen != address {
				t.Fatalf("listen = %q", value.listen)
			}
		})
	}

	for _, address := range []string{"", ":8080", "0.0.0.0:8080", "10.0.0.13:8080", "localhost.evil:8080", "localhost:http", "localhost:0", "localhost:65536"} {
		t.Run("reject "+address, func(t *testing.T) {
			if _, err := parseConfig([]string{"-listen", address}); err == nil || !strings.Contains(err.Error(), "listen") {
				t.Fatalf("parseConfig(%q) error = %v", address, err)
			}
		})
	}
}

func TestConfigPairsTLSCertificateAndKey(t *testing.T) {
	value, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if value.tls {
		t.Fatal("TLS was enabled by default")
	}
	for _, arguments := range [][]string{
		{"-tls-cert", "cert.pem"},
		{"-tls-key", "key.pem"},
	} {
		if _, err := parseConfig(arguments); err == nil {
			t.Fatalf("parseConfig(%v) accepted a half-configured certificate", arguments)
		}
	}
	value, err = parseConfig([]string{"-tls-cert", "cert.pem", "-tls-key", "key.pem"})
	if err != nil {
		t.Fatal(err)
	}
	if !value.tls {
		t.Fatal("an operator-supplied certificate did not enable TLS")
	}
}

func TestCertificateHostsCoverEveryReachableAddress(t *testing.T) {
	loopback := config{listen: "127.0.0.1:8080", dataDir: "."}
	hosts := certificateHosts(loopback, net.ParseIP("127.0.0.1"))
	for _, want := range []string{"localhost", "127.0.0.1", "::1"} {
		if !slices.Contains(hosts, want) {
			t.Fatalf("hosts = %v, missing %q", hosts, want)
		}
	}
	// Without LAN mode the certificate must not claim addresses the listener
	// refuses to bind, so the set stays exactly the loopback identities.
	if len(hosts) != 3 {
		t.Fatalf("loopback hosts = %v, expected only loopback identities", hosts)
	}

	published := config{listen: "10.0.0.13:42817", dataDir: ".", lan: true, publishedHost: "192.168.1.20"}
	hosts = certificateHosts(published, net.ParseIP("10.0.0.13"))
	for _, want := range []string{"localhost", "127.0.0.1", "::1", "10.0.0.13", "192.168.1.20"} {
		if !slices.Contains(hosts, want) {
			t.Fatalf("hosts = %v, missing %q", hosts, want)
		}
	}
	// The certificate is public to everyone who opens the page. A listener
	// binds one address, so enumerating the machine's other interfaces would
	// publish its VPN, container and virtual-machine subnets for nothing.
	if len(hosts) != 5 {
		t.Fatalf("hosts = %v, expected only the reachable addresses", hosts)
	}
}

func TestRunRejectsNonLoopbackListenerWithoutCreatingData(t *testing.T) {
	dataDir := t.TempDir()
	err := run(context.Background(), config{listen: "0.0.0.0:8080", dataDir: dataDir})
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("run() error = %v", err)
	}
	entries, err := os.ReadDir(dataDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("data directory after rejection = %v, error = %v", entries, err)
	}
}
