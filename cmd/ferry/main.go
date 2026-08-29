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
	listen  string
	dataDir string
}

func parseConfig(arguments []string) (config, error) {
	flags := flag.NewFlagSet("ferry", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var value config
	flags.StringVar(&value.listen, "listen", "127.0.0.1:8080", "HTTP listen address")
	flags.StringVar(&value.dataDir, "data-dir", "./ferry-data", "directory for the SQLite database and uploaded files")
	if err := flags.Parse(arguments); err != nil {
		return config{}, err
	}
	if flags.NArg() != 0 {
		return config{}, fmt.Errorf("unexpected positional arguments")
	}
	if err := validateLoopbackAddress(value.listen); err != nil {
		return config{}, err
	}
	if value.dataDir == "" {
		return config{}, fmt.Errorf("data-dir must not be empty")
	}
	return value, nil
}

func validateLoopbackAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("listen must be a loopback host and numeric port: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("listen port must be between 1 and 65535")
	}
	ip := net.ParseIP(host)
	if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("listen host must be localhost or a loopback IP address")
	}
	return nil
}

func run(ctx context.Context, value config) error {
	if err := validateLoopbackAddress(value.listen); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", value.listen)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer listener.Close()
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok || !address.IP.IsLoopback() {
		return fmt.Errorf("resolved listen address must be loopback")
	}

	store, err := ferry.OpenStore(ctx, value.dataDir)
	if err != nil {
		return err
	}
	defer store.Close()

	server := &http.Server{
		Addr:              value.listen,
		Handler:           ferry.NewHandler(store, log.Default()),
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
