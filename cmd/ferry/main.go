package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/max1874/ferry/internal/ferry"
	"github.com/max1874/ferry/internal/lantls"
)

type config struct {
	listen        string
	dataDir       string
	publishedHost string
	lan           bool
	tls           bool
	tlsCert       string
	tlsKey        string
}

func parseConfig(arguments []string) (config, error) {
	flags := flag.NewFlagSet("ferry", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var value config
	flags.StringVar(&value.listen, "listen", "127.0.0.1:8080", "HTTP listen address")
	flags.StringVar(&value.dataDir, "data-dir", "./ferry-data", "directory for the SQLite database and uploaded files")
	flags.StringVar(&value.publishedHost, "published-host", "", "optional host IP that publishes this listener")
	flags.BoolVar(&value.lan, "lan", false, "allow an authenticated HTTP listener on private LAN addresses")
	flags.BoolVar(&value.tls, "tls", false, "serve HTTPS; without -tls-cert Ferry keeps a local CA under <data-dir>/tls")
	flags.StringVar(&value.tlsCert, "tls-cert", "", "PEM certificate file; implies -tls and must be paired with -tls-key")
	flags.StringVar(&value.tlsKey, "tls-key", "", "PEM private key file for -tls-cert")
	if err := flags.Parse(arguments); err != nil {
		return config{}, err
	}
	if flags.NArg() != 0 {
		return config{}, fmt.Errorf("unexpected positional arguments")
	}
	if err := validateListenAddress(value.listen, value.lan); err != nil {
		return config{}, err
	}
	if value.publishedHost != "" {
		ip := net.ParseIP(value.publishedHost)
		if ip == nil || !(ip.IsLoopback() || value.lan && allowedLANIP(ip)) {
			return config{}, fmt.Errorf("published-host must be a loopback or private/link-local IP address")
		}
	}
	if value.dataDir == "" {
		return config{}, fmt.Errorf("data-dir must not be empty")
	}
	if (value.tlsCert == "") != (value.tlsKey == "") {
		return config{}, fmt.Errorf("tls-cert and tls-key must be provided together")
	}
	if value.tlsCert != "" {
		value.tls = true
	}
	return value, nil
}

func validateListenAddress(address string, allowLAN bool) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("listen must be a loopback host and numeric port: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("listen port must be between 1 and 65535")
	}
	ip := net.ParseIP(host)
	if strings.EqualFold(host, "localhost") || ip != nil && (ip.IsLoopback() || allowLAN && allowedLANIP(ip)) {
		return nil
	}
	if allowLAN {
		return fmt.Errorf("listen host must be localhost, loopback, or a private/link-local IP address")
	}
	return fmt.Errorf("listen host must be localhost or a loopback IP address; use -lan for a private LAN listener")
}

func allowedLANIP(ip net.IP) bool {
	return ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

func tlsMaterial(value config, bound net.IP) (*lantls.Material, error) {
	if value.tlsCert != "" {
		return lantls.LoadPair(value.tlsCert, value.tlsKey)
	}
	return lantls.Ensure(filepath.Join(value.dataDir, "tls"), certificateHosts(value, bound))
}

func caCertificate(material *lantls.Material) []byte {
	if material == nil {
		return nil
	}
	return material.CAPEM
}

// certificateHosts is every address a device can actually reach this listener
// at: the address it bound, the address a container publishes on its behalf,
// and the loopback identities.
//
// It deliberately does not enumerate the machine's other interfaces. A Ferry
// listener binds exactly one address, so certifying the rest would not make the
// server reachable there — it would only publish the host's VPN, container and
// virtual-machine subnets to everyone who opens the page.
func certificateHosts(value config, bound net.IP) []string {
	hosts := []string{"localhost", "127.0.0.1", "::1"}
	if bound != nil && !bound.IsUnspecified() {
		hosts = append(hosts, bound.String())
	}
	if value.publishedHost != "" {
		hosts = append(hosts, value.publishedHost)
	}
	return hosts
}

func run(ctx context.Context, value config) error {
	if err := validateListenAddress(value.listen, value.lan); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", value.listen)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer listener.Close()
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok || !(address.IP.IsLoopback() || value.lan && allowedLANIP(address.IP)) {
		return fmt.Errorf("resolved listen address is outside the allowed listener boundary")
	}

	var material *lantls.Material
	if value.tls {
		material, err = tlsMaterial(value, address.IP)
		if err != nil {
			return err
		}
	}

	store, err := ferry.OpenStore(ctx, value.dataDir)
	if err != nil {
		return err
	}
	defer store.Close()
	if value.lan && material == nil {
		log.Printf("LAN mode without -tls uses unencrypted HTTP; the access password and device tokens travel in the clear")
	}

	server := &http.Server{
		Addr: value.listen,
		Handler: ferry.NewHandler(store, ferry.HandlerOptions{
			AllowLANHosts: value.lan,
			Logger:        log.Default(),
			CACertificate: caCertificate(material),
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       60 * time.Second,
	}
	scheme := "http"
	if material != nil {
		scheme = "https"
		server.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{material.Certificate},
			MinVersion:   tls.VersionTLS12,
		}
	}

	result := make(chan error, 1)
	go func() {
		log.Printf("Ferry is running at %s://%s", scheme, listener.Addr())
		if len(caCertificate(material)) > 0 {
			if material.CAIssued {
				log.Printf("Issued a new local certificate authority; every device has to trust this one before it can connect")
			}
			log.Printf("Trust this server's CA once per device: %s://%s/ferry-ca.crt", scheme, listener.Addr())
			log.Printf("CA SHA-256 fingerprint: %s", material.CAFingerprint)
		}
		if material == nil {
			result <- server.Serve(listener)
			return
		}
		result <- server.ServeTLS(listener, "", "")
	}()

	select {
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("shut down server: %w", err)
		}
		err := <-result
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func main() {
	value, err := parseConfig(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, value); err != nil {
		log.Fatal(err)
	}
}
