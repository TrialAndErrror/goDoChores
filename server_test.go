package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln
}

func TestNewServerSetsTimeouts(t *testing.T) {
	srv := newServer(http.NotFoundHandler())
	for name, d := range map[string]time.Duration{
		"ReadHeaderTimeout": srv.ReadHeaderTimeout,
		"ReadTimeout":       srv.ReadTimeout,
		"WriteTimeout":      srv.WriteTimeout,
		"IdleTimeout":       srv.IdleTimeout,
	} {
		if d <= 0 {
			t.Errorf("%s is not set", name)
		}
	}
	if srv.ReadHeaderTimeout > srv.ReadTimeout {
		t.Error("ReadHeaderTimeout should not exceed ReadTimeout")
	}
}

func TestGracefulShutdownLetsRunningRequestsFinish(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	srv := newServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		io.WriteString(w, "finished")
	}))
	ln := listen(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- serve(ctx, srv, ln, 5*time.Second) }()

	type result struct {
		body string
		err  error
	}
	resp := make(chan result, 1)
	go func() {
		r, err := http.Get("http://" + ln.Addr().String() + "/")
		if err != nil {
			resp <- result{err: err}
			return
		}
		defer r.Body.Close()
		b, err := io.ReadAll(r.Body)
		resp <- result{string(b), err}
	}()

	<-started
	cancel() // "SIGTERM" arrives while the request is still running

	select {
	case err := <-served:
		t.Fatalf("serve returned (%v) while a request was still running", err)
	case <-time.After(200 * time.Millisecond):
	}

	close(release)
	if r := <-resp; r.err != nil || r.body != "finished" {
		t.Errorf("in-flight request was cut off: body %q, err %v", r.body, r.err)
	}
	select {
	case err := <-served:
		if err != nil {
			t.Errorf("serve after clean shutdown: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("serve did not return after the request finished")
	}

	// The port is closed now.
	if c, err := net.DialTimeout("tcp", ln.Addr().String(), time.Second); err == nil {
		c.Close()
		t.Error("server still accepts connections after shutdown")
	}
}

func TestShutdownGivesUpOnStuckRequests(t *testing.T) {
	started := make(chan struct{})
	stuck := make(chan struct{})
	defer close(stuck)
	srv := newServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-stuck
	}))
	ln := listen(t)
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- serve(ctx, srv, ln, 300*time.Millisecond) }()

	clientErr := make(chan error, 1)
	go func() {
		r, err := http.Get("http://" + ln.Addr().String() + "/")
		if err == nil {
			r.Body.Close()
		}
		clientErr <- err
	}()
	<-started
	cancel()

	select {
	case err := <-served:
		if err == nil || !strings.Contains(err.Error(), "graceful shutdown") {
			t.Errorf("want a graceful-shutdown error, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown hung on a stuck request instead of giving up after the grace period")
	}
	select {
	case err := <-clientErr:
		if err == nil {
			t.Error("the stuck request should have been dropped")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stuck client connection was not closed")
	}
}

func TestServeReturnsErrorsFromTheServer(t *testing.T) {
	ln := listen(t)
	ln.Close() // serving on a closed listener fails straight away
	err := serve(context.Background(), newServer(http.NotFoundHandler()), ln, time.Second)
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestSlowClientsAreDisconnected(t *testing.T) {
	srv := newServer(http.NotFoundHandler())
	srv.ReadHeaderTimeout = 150 * time.Millisecond // shortened so the test is quick
	ln := listen(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go serve(ctx, srv, ln, time.Second)

	// Send the start of a request and then go quiet, like a slowloris client.
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: x\r\nX-Slow: "); err != nil {
		t.Fatal(err)
	}

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	start := time.Now()
	_, err = io.ReadAll(conn) // returns once the server closes the connection
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatal("server never closed the stalled connection")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("stalled connection lasted %v", elapsed)
	}

	// A normal client is still served promptly.
	resp, err := http.Get("http://" + ln.Addr().String() + "/")
	if err != nil {
		t.Fatalf("normal request failed: %v", err)
	}
	resp.Body.Close()
}
