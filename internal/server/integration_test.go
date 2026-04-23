package server_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/iarkhanhelsky/serve/internal/config"
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
	opts := types.RunOptions{Root: dir, Listen: listen}
	runServer(t, opts, logPath)

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

func TestReverseProxyAndWebsocket(t *testing.T) {
	var upstreamReqID atomic.Value
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamReqID.Store(r.Header.Get("X-Request-Id"))
		if websocket.IsWebSocketUpgrade(r) {
			upgrader := websocket.Upgrader{}
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			_ = conn.WriteMessage(mt, msg)
			return
		}
		w.Header().Set("X-Upstream", "ok")
		_, _ = w.Write([]byte("proxied"))
	}))
	defer upstream.Close()

	_, port, _ := net.SplitHostPort(upstream.Listener.Addr().String())
	listen := ":" + freePort(t)
	logPath := filepath.Join(t.TempDir(), "access.log")
	opts := types.RunOptions{Listen: listen, Upstream: "127.0.0.1:" + port}
	runServer(t, opts, logPath)

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

	wsURL := "ws://127.0.0.1" + listen + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()
	if err := conn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
		t.Fatalf("ws write: %v", err)
	}
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	if string(msg) != "ping" {
		t.Fatalf("ws echo: got %q want ping", string(msg))
	}
}

func runServer(t *testing.T, opts types.RunOptions, logPath string) {
	t.Helper()
	if err := os.WriteFile(logPath, nil, 0o644); err != nil {
		t.Fatalf("prepare log file: %v", err)
	}
	cfg, err := config.BuildConfigJSON(opts, logPath)
	if err != nil {
		t.Fatalf("build config: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go func() {
		err := server.Run(ctx, cfg)
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
