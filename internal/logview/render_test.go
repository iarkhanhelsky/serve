package logview

import (
	"strings"
	"testing"
)

func TestRenderModes(t *testing.T) {
	t.Parallel()

	evt := AccessEvent{}
	evt.Request.Method = "GET"
	evt.Request.URI = "/healthz"
	evt.Request.ClientIP = "127.0.0.1"
	evt.Status = 200
	evt.Size = 123
	evt.Duration = 0.012

	if got := RenderCompact(evt, "127.0.0.1:3000"); got == "" {
		t.Fatal("compact render should not be empty")
	}
	if got := RenderPretty(evt, "127.0.0.1:3000"); got == "" {
		t.Fatal("pretty render should not be empty")
	}
	if got := RenderJSON(evt); got == "" {
		t.Fatal("json render should not be empty")
	}
}

func TestRenderPrettyNoColorWhenDisabled(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "xterm-256color")

	evt := AccessEvent{}
	evt.Request.Method = "GET"
	evt.Request.URI = "/"
	evt.Request.ClientIP = "127.0.0.1"
	evt.Status = 200

	got := RenderPretty(evt, "")
	if strings.Contains(got, "\033[") {
		t.Fatal("pretty render should not include ANSI escapes when NO_COLOR is set")
	}
}

func TestRenderPrettyColorizedByStatusClass(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")

	cases := []struct {
		name   string
		status int
		color  string
	}{
		{name: "2xx", status: 200, color: "\033[32m"},
		{name: "3xx", status: 302, color: "\033[36m"},
		{name: "4xx", status: 404, color: "\033[33m"},
		{name: "5xx", status: 500, color: "\033[31m"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			evt := AccessEvent{}
			evt.Request.Method = "GET"
			evt.Request.URI = "/"
			evt.Request.ClientIP = "127.0.0.1"
			evt.Status = tc.status

			got := RenderPretty(evt, "")
			if !strings.Contains(got, tc.color) {
				t.Fatalf("expected color prefix %q in pretty output, got: %q", tc.color, got)
			}
		})
	}
}
