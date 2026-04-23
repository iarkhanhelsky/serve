package server_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iarkhanhelsky/serve/internal/server"
	"github.com/iarkhanhelsky/serve/internal/types"
)

func TestStaticAndBrowse(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	listen := ":" + freePort(t)
	logPath := filepath.Join(t.TempDir(), "access.log")
	opts := types.RunOptions{Root: dir, Listen: listen, LogFile: logPath}
	runServer(t, opts)

	resp := mustGet(t, "http://127.0.0.1"+listen+"/hello.txt")
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d want 200", resp.StatusCode)
	}
	if string(body) != "hello" {
		t.Fatalf("body: got %q want hello", string(body))
	}

	resp = mustGet(t, "http://127.0.0.1"+listen+"/")
	indexBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("index status: got %d want 200", resp.StatusCode)
	}
	if len(indexBody) == 0 {
		t.Fatal("expected browse index content")
	}
}

func TestReverseProxyAndNoWebsocket(t *testing.T) {
	var upstreamReqID atomic.Value
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamReqID.Store(r.Header.Get("X-Request-Id"))
		w.Header().Set("X-Upstream", "ok")
		_, _ = w.Write([]byte("proxied"))
	}))
	defer upstream.Close()

	_, port, _ := net.SplitHostPort(upstream.Listener.Addr().String())
	listen := ":" + freePort(t)
	logPath := filepath.Join(t.TempDir(), "access.log")
	opts := types.RunOptions{Listen: listen, Upstream: "127.0.0.1:" + port, LogFile: logPath}
	runServer(t, opts)

	resp := mustGet(t, "http://127.0.0.1"+listen+"/")
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "proxied" {
		t.Fatalf("proxy body: got %q want proxied", string(body))
	}
	reqID := resp.Header.Get("X-Request-Id")
	if reqID == "" {
		t.Fatal("expected X-Request-Id response header from proxy")
	}
	v := upstreamReqID.Load()
	if v == nil || v.(string) == "" {
		t.Fatal("expected X-Request-Id request header to be injected upstream")
	}

	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1"+listen+"/ws", nil)
	if err != nil {
		t.Fatalf("build websocket-like request: %v", err)
	}
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	wsResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("websocket-like request: %v", err)
	}
	defer wsResp.Body.Close()
	if wsResp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("expected websocket disabled status 501, got %d", wsResp.StatusCode)
	}
}

func TestAccessLogContract(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	listen := ":" + freePort(t)
	logPath := filepath.Join(t.TempDir(), "access.log")
	opts := types.RunOptions{Root: dir, Listen: listen, LogFile: logPath}
	runServer(t, opts)

	resp := mustGet(t, "http://127.0.0.1"+listen+"/hello.txt")
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(logPath)
		if err == nil && len(b) > 0 {
			line := b
			scanner := bufio.NewScanner(strings.NewReader(string(b)))
			if scanner.Scan() {
				line = append([]byte(nil), scanner.Bytes()...)
			}
			var evt map[string]any
			if err := json.Unmarshal(line, &evt); err != nil {
				t.Fatalf("expected JSON log line: %v", err)
			}
			if _, ok := evt["ts"]; !ok {
				t.Fatal("expected ts in access event")
			}
			if _, ok := evt["request"]; !ok {
				t.Fatal("expected request in access event")
			}
			if _, ok := evt["status"]; !ok {
				t.Fatal("expected status in access event")
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("expected access log at %s", logPath)
}

func runServer(t *testing.T, opts types.RunOptions) {
	t.Helper()
	if opts.LogFile == "" {
		opts.LogFile = filepath.Join(t.TempDir(), "access.log")
	}
	if err := os.WriteFile(opts.LogFile, nil, 0o644); err != nil {
		t.Fatalf("prepare log file: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go func() {
		err := server.Run(ctx, opts)
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Logf("server run error: %v", err)
		}
	}()

	waitForReady(t, "http://127.0.0.1"+opts.Listen+"/")
}

func waitForReady(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("server did not become ready: %s", url)
}

func mustGet(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("http get %s: %v", url, err)
	}
	return resp
}

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate port: %v", err)
	}
	defer l.Close()
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		t.Fatalf("parse free port: %v", err)
	}
	return port
}
