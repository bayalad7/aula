package main

import (
	"context"
	"crypto/ed25519"
	"errors"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/security"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/transport"
)

// startConsole runs a real console (TLS + authentication) on loopback and
// returns its address and public key.
func startConsole(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	id, err := security.CreateIdentity(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := security.LoadOrCreateCertificate(dir)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	srv := transport.NewServer(transport.Options{TLS: security.ServerTLS(cert), Logger: quiet})
	console := &session.Console{Channel: "AULA_101", Registry: session.NewRegistry(0), Identity: id, CertSHA256: cert.SHA256, Logger: quiet}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = srv.Serve(ctx, ln) }()
	go func() { defer wg.Done(); _ = console.Run(ctx, srv.Events()) }()
	t.Cleanup(func() { cancel(); wg.Wait() })
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)), security.EncodePublicKey(id.Public())
}

// The agent starts without configuration; the installer then writes it. The
// agent must pick it up and connect without being restarted, and go back to
// "unconfigured" if the file disappears.
func TestConfigWrittenWhileRunningIsPickedUp(t *testing.T) {
	addr, key := startConsole(t)

	var mu sync.Mutex
	current := Config{}
	reload := func() (Config, ed25519.PublicKey, error) {
		mu.Lock()
		defer mu.Unlock()
		pub, err := current.Validate()
		return current, pub, err
	}
	_, _, firstErr := reload()
	a := NewApp(Config{}, nil, firstErr, localHello(), nil, errors.New("sin portapapeles"), quiet)
	rec := &recorder{}
	a.emit = rec.emit
	a.reload = reload
	a.reloadEvery = 20 * time.Millisecond
	a.start(context.Background())
	defer a.stop()

	if s := a.View().Status; s.State != StateUnconfigured {
		t.Fatalf("initial state = %s", s.State)
	}
	mu.Lock()
	current = Config{Channel: "AULA_101", ConsolePublicKey: key, Direct: addr} // the installer writes config.json
	mu.Unlock()
	waitState(t, a, StateConnected)

	mu.Lock()
	current = Config{} // config removed
	mu.Unlock()
	waitState(t, a, StateUnconfigured)
}

func waitState(t *testing.T, a *App, want string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if a.View().Status.State == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("state = %s, want %s", a.View().Status.State, want)
}
