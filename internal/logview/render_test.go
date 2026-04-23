package logview

import (
	"strings"
	"testing"
	"time"
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
	if got := formatByMode(evt, "status", "127.0.0.1:3000"); got == "" {
		t.Fatal("status mode should render compact fallback output")
	}
}

func TestParseAccessEventWithNumericTimestamp(t *testing.T) {
	line := []byte(`{"ts":1713900000.25,"request":{"method":"GET","uri":"/","remote_ip":"127.0.0.1","headers":{}},"status":200}`)
	evt, err := ParseAccessEvent(line)
	if err != nil {
		t.Fatalf("parse should succeed: %v", err)
	}
	if evt.Status != 200 {
		t.Fatalf("unexpected status: %d", evt.Status)
	}
	if evt.Timestamp.IsZero() {
		t.Fatal("expected parsed timestamp")
	}
}

func TestParseAccessEventWithStringTimestamp(t *testing.T) {
	line := []byte(`{"ts":"2026-04-23T20:24:15.932Z","request":{"method":"GET","uri":"/","remote_ip":"127.0.0.1","headers":{}},"status":200}`)
	evt, err := ParseAccessEvent(line)
	if err != nil {
		t.Fatalf("parse should succeed: %v", err)
	}
	if evt.Timestamp.IsZero() {
		t.Fatal("expected parsed timestamp")
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

func TestDashboardStateCountsAndRecent(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	state := newDashboardState(time.Unix(100, 0))
	for i := 0; i < 6; i++ {
		evt := AccessEvent{}
		evt.Request.Method = "GET"
		evt.Request.URI = "/healthz"
		evt.Request.ClientIP = "127.0.0.1"
		evt.Status = 200
		evt.Duration = 0.005
		state.addEvent(evt)
	}
	evtErr := AccessEvent{}
	evtErr.Request.Method = "POST"
	evtErr.Request.URI = "/api/error"
	evtErr.Request.ClientIP = "127.0.0.1"
	evtErr.Status = 500
	evtErr.Duration = 0.010
	state.addEvent(evtErr)

	frame := state.frame(time.Unix(112, 0), dashboardFrameOptions{
		colorize: supportsColorOutput(),
		listen:   ":8000",
		root:     "/tmp/site",
		upstream: "127.0.0.1:3000",
		mode:     "status",
	}, 120)
	if !strings.Contains(frame, "Session Status                online") {
		t.Fatalf("unexpected dashboard totals: %q", frame)
	}
	if !strings.Contains(frame, "2xx=6") || !strings.Contains(frame, "5xx=1") {
		t.Fatalf("unexpected status counters: %q", frame)
	}
	if !strings.Contains(frame, "Connections                   ttl     opn     p50     p90") {
		t.Fatalf("expected connections section, got: %q", frame)
	}
	if !strings.Contains(frame, "POST   /api/error") || !strings.Contains(frame, "500 Internal Server Error") {
		t.Fatalf("expected latest request in list, got frame: %q", frame)
	}
	if strings.Count(frame, "waiting for requests") != 0 {
		t.Fatalf("did not expect waiting message when requests exist, got frame: %q", frame)
	}
}

func TestDashboardFrameWaitingShownOnce(t *testing.T) {
	state := newDashboardState(time.Unix(100, 0))
	frame := state.frame(time.Unix(101, 0), dashboardFrameOptions{
		colorize: false,
		listen:   ":8000",
		root:     "/tmp/site",
		mode:     "status",
	}, 120)
	if strings.Count(frame, "waiting for requests") != 1 {
		t.Fatalf("expected single waiting line, got frame: %q", frame)
	}
}

func TestClampLine(t *testing.T) {
	if got := clampLine("abcdefghijklmnopqrstuvwxyz", 10); got != "abcdefg..." {
		t.Fatalf("unexpected clamp result: %q", got)
	}
	if got := clampLine("short", 10); got != "short" {
		t.Fatalf("short line should not change: %q", got)
	}
}
