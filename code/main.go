package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const defaultRingBytes = 4 * 1024 * 1024

type config struct {
	socketPath string
	command    string
	workdir    string
	version    string
	ringBytes  int
}

func (cfg config) validate() error {
	if !filepath.IsAbs(cfg.socketPath) {
		return errors.New("socket path must be absolute")
	}
	if !filepath.IsAbs(cfg.command) {
		return errors.New("terminal command must be absolute")
	}
	if !filepath.IsAbs(cfg.workdir) {
		return errors.New("working directory must be absolute")
	}
	if cfg.ringBytes < 64*1024 || cfg.ringBytes > 64*1024*1024 {
		return errors.New("ring buffer must be between 64 KiB and 64 MiB")
	}
	return nil
}

func run(cfg config) error {
	if err := cfg.validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.socketPath), 0o755); err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}
	if err := os.Remove(cfg.socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale socket: %w", err)
	}

	listener, err := net.Listen("unix", cfg.socketPath)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.socketPath, err)
	}
	defer listener.Close()
	defer os.Remove(cfg.socketPath)
	// The capability-free UID 0 vsock bridge and the unprivileged Herdr user
	// share only this guest-local socket. The guest has no other login users.
	if err := os.Chmod(cfg.socketPath, 0o666); err != nil {
		return fmt.Errorf("set socket permissions: %w", err)
	}

	session := newTerminalSession(cfg.command, nil, cfg.workdir, cfg.version, cfg.ringBytes)
	session.start()
	defer session.close()

	terminalHTTP, err := newTerminalServer(session)
	if err != nil {
		return err
	}
	httpServer := &http.Server{
		Handler:           terminalHTTP.handler(),
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	serverError := make(chan error, 1)
	go func() {
		serverError <- httpServer.Serve(listener)
	}()

	signalContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serverError:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-signalContext.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownContext)
	}
}

func main() {
	var cfg config
	flag.StringVar(&cfg.socketPath, "socket", "/run/spr-herdr/ui.sock", "guest-local Unix socket")
	flag.StringVar(&cfg.command, "command", "/usr/local/bin/herdr", "Herdr executable")
	flag.StringVar(&cfg.workdir, "workdir", "/home/herdr/workspace", "initial Herdr working directory")
	flag.StringVar(&cfg.version, "version", "unknown", "packaged Herdr version")
	flag.IntVar(&cfg.ringBytes, "ring-bytes", defaultRingBytes, "terminal replay ring size")
	flag.Parse()

	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
