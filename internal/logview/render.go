package logview

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// AccessEvent is a minimal projection of Caddy JSON access logs.
type AccessEvent struct {
	Timestamp time.Time `json:"ts"`
	Request   struct {
		ClientIP string `json:"remote_ip"`
		Method   string `json:"method"`
		URI      string `json:"uri"`
		Headers  struct {
			RequestID []string `json:"X-Request-Id"`
		} `json:"headers"`
	} `json:"request"`
	Status      int     `json:"status"`
	Size        int     `json:"size"`
	Duration    float64 `json:"duration"`
	RespHeaders struct {
		Upstream []string `json:"X-Serve-Upstream"`
	} `json:"resp_headers"`
	CommonLog string `json:"msg"`
}

func ParseAccessEvent(line []byte) (AccessEvent, error) {
	var evt AccessEvent
	err := json.Unmarshal(line, &evt)
	return evt, err
}

func RenderPretty(evt AccessEvent, configuredUpstream string) string {
	reqID := "-"
	if len(evt.Request.Headers.RequestID) > 0 {
		reqID = evt.Request.Headers.RequestID[0]
	}
	class := statusClass(evt.Status)
	class = colorizeStatusClass(class)
	return fmt.Sprintf(
		"%s %-4s %-6s %-40s %6d %7s %8s %-22s %s",
		reqID,
		evt.Request.Method,
		class,
		trimPath(evt.Request.URI, 40),
		evt.Status,
		humanBytes(evt.Size),
		humanDuration(evt.Duration),
		upstreamValue(evt, configuredUpstream),
		evt.Request.ClientIP,
	)
}

func RenderCompact(evt AccessEvent, configuredUpstream string) string {
	return fmt.Sprintf("%s %s %d %s %s", evt.Request.Method, evt.Request.URI, evt.Status, humanDuration(evt.Duration), upstreamValue(evt, configuredUpstream))
}

func RenderJSON(evt AccessEvent) string {
	b, _ := json.Marshal(evt)
	return string(b)
}

func colorizeStatusClass(class string) string {
	// Respect common no-color env flags to keep logs script-friendly.
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return class
	}
	switch class {
	case "2xx":
		return "\033[32m" + class + "\033[0m"
	case "3xx":
		return "\033[36m" + class + "\033[0m"
	case "4xx":
		return "\033[33m" + class + "\033[0m"
	case "5xx":
		return "\033[31m" + class + "\033[0m"
	default:
		return class
	}
}

func statusClass(status int) string {
	switch {
	case status >= http.StatusInternalServerError:
		return "5xx"
	case status >= http.StatusBadRequest:
		return "4xx"
	case status >= http.StatusMultipleChoices:
		return "3xx"
	default:
		return "2xx"
	}
}

func humanDuration(seconds float64) string {
	return (time.Duration(seconds * float64(time.Second))).String()
}

func humanBytes(size int) string {
	switch {
	case size > 1024*1024:
		return fmt.Sprintf("%.1fMB", float64(size)/(1024*1024))
	case size > 1024:
		return fmt.Sprintf("%.1fKB", float64(size)/1024)
	default:
		return fmt.Sprintf("%dB", size)
	}
}

func trimPath(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + strings.Repeat(".", 3)
}

func upstreamValue(evt AccessEvent, configuredUpstream string) string {
	if len(evt.RespHeaders.Upstream) == 0 {
		if configuredUpstream == "" {
			return "-"
		}
		return configuredUpstream
	}
	return evt.RespHeaders.Upstream[0]
}
