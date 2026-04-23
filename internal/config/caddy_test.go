package config

import (
	"strings"
	"testing"

	"github.com/iarkhanhelsky/serve/internal/types"
)

func TestBuildCaddyfileStatic(t *testing.T) {
	t.Parallel()
	opts := types.RunOptions{
		Root:   "/tmp/path with spaces",
		Listen: ":8000",
	}
	b, err := BuildConfigJSON(opts, "/tmp/access log.json")
	if err != nil {
		t.Fatalf("build config: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, `"handler":"file_server"`) {
		t.Fatal("expected browse file server stanza")
	}
	if !strings.Contains(got, `"/tmp/path with spaces"`) {
		t.Fatal("expected root path in JSON config")
	}
	if !strings.Contains(got, `"X-Request-Id":["{http.request.id}"]`) {
		t.Fatal("expected request id header injection in static mode")
	}
	if !strings.Contains(got, `"Cache-Control":["no-store, max-age=0"]`) {
		t.Fatal("expected no-store cache-control in static mode")
	}
}

func TestBuildCaddyfileProxy(t *testing.T) {
	t.Parallel()
	opts := types.RunOptions{
		Listen:   ":8000",
		Upstream: "127.0.0.1:3000",
	}
	b, err := BuildConfigJSON(opts, "/tmp/access.json")
	if err != nil {
		t.Fatalf("build config: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, `"handler":"reverse_proxy"`) {
		t.Fatal("expected reverse proxy stanza")
	}
	if !strings.Contains(got, `"dial":"127.0.0.1:3000"`) {
		t.Fatal("expected upstream dial target")
	}
	if !strings.Contains(got, `"X-Request-Id":["{http.request.id}"]`) {
		t.Fatal("expected request id header injection in proxy mode")
	}
	if !strings.Contains(got, `"Cache-Control":["no-store, max-age=0"]`) {
		t.Fatal("expected no-store cache-control in proxy mode")
	}
}
