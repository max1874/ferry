package main

import (
	"context"
	"os"
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
	if value.pair {
		t.Fatal("pair recovery was enabled by default")
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

func TestConfigAcceptsExplicitPairRecovery(t *testing.T) {
	value, err := parseConfig([]string{"-pair"})
	if err != nil || !value.pair {
		t.Fatalf("config = %#v, error = %v", value, err)
	}
}

func TestPairingCodeIssuancePolicy(t *testing.T) {
	if !shouldIssuePairingCode(0, false) || shouldIssuePairingCode(1, false) || !shouldIssuePairingCode(1, true) {
		t.Fatal("pairing code issuance policy changed")
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
