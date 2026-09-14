package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/max1874/ferry/internal/ferry"
)

type config struct {
	listen        string
	dataDir       string
	publishedHost string
	trustedOrigin string
	lan           bool
}

func parseConfig(arguments []string) (config, error) {
	flags := flag.NewFlagSet("ferry", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var value config
	flags.StringVar(&value.listen, "listen", "127.0.0.1:8080", "HTTP listen address")
	flags.StringVar(&value.dataDir, "data-dir", "./ferry-data", "directory for the SQLite database and uploaded files")
	flags.StringVar(&value.publishedHost, "published-host", "", "optional host IP that publishes this listener")
	flags.StringVar(&value.trustedOrigin, "trusted-origin", "", "optional reverse-proxy origin, such as https://ferry.example.com")
	flags.BoolVar(&value.lan, "lan", false, "allow an authenticated HTTP listener on private LAN addresses")
	if err := flags.Parse(arguments); err != nil {
		return config{}, err
	}
	if flags.NArg() != 0 {
		return config{}, fmt.Errorf("unexpected positional arguments")
	}
	if value.trustedOrigin != "" {
		origin, err := ferry.ParseTrustedOrigin(value.trustedOrigin)
		if err != nil {
			return config{}, err
		}
		value.trustedOrigin = origin.String()
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
	if strings.EqualFold(host, "localhost") || ip != nil && (ip.IsLoopback() || allowLAN && (allowedLANIP(ip) || ip.IsUnspecified())) {
		return nil
	}
	if allowLAN {
		return fmt.Errorf("listen host must be localhost, loopback, a private/link-local IP address, or 0.0.0.0/[::]")
	}
	return fmt.Errorf("listen host must be localhost or a loopback IP address; use -lan for a private LAN listener")
}

func allowedLANIP(ip net.IP) bool {
	return ip.IsPrivate() || ip.IsLinkLocalUnicast()
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
	if !ok || !(address.IP.IsLoopback() || value.lan && (allowedLANIP(address.IP) || address.IP.IsUnspecified())) {
		return fmt.Errorf("resolved listen address is outside the allowed listener boundary")
	}

	store, err := ferry.OpenStore(ctx, value.dataDir)
	if err != nil {
		return err
	}
	defer store.Close()
	if value.lan {
		log.Printf("LAN mode uses unencrypted HTTP; use only on a trusted network")
	}
	if address.IP.IsUnspecified() {
		log.Printf("listening on every network interface; the host firewall and any port publishing decide who can reach Ferry")
	}
	if value.trustedOrigin != "" {
		log.Printf("accepting browser requests for reverse-proxy origin %s", value.trustedOrigin)
	}

	server := &http.Server{
		Addr:              value.listen,
		Handler:           ferry.NewHandler(store, ferry.HandlerOptions{AllowLANHosts: value.lan, TrustedOrigin: value.trustedOrigin, Logger: log.Default()}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       60 * time.Second,
	}

	result := make(chan error, 1)
	go func() {
		log.Printf("Ferry is running at http://%s", listener.Addr())
		result <- server.Serve(listener)
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
