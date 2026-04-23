package server

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/iarkhanhelsky/serve/internal/types"
)

const (
	headerRequestID  = "X-Request-Id"
	headerCacheCtl   = "Cache-Control"
	headerPragma     = "Pragma"
	headerExpires    = "Expires"
	headerUpgrade    = "Upgrade"
	noCacheValue     = "no-store, max-age=0"
	logTimeFormat    = time.RFC3339Nano
	websocketUpgrade = "websocket"
)

// Run starts the stdlib HTTP server and blocks until ctx is canceled.
func Run(ctx context.Context, opts types.RunOptions) error {
	logFile, err := os.OpenFile(opts.LogFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("open access log: %w", err)
	}
	defer logFile.Close()

	var nextID uint64
	handler, err := buildHandler(opts)
	if err != nil {
		return err
	}
	handler = withRequestHeaders(handler)
	handler = withAccessLog(handler, logFile, opts, &nextID)

	srv := &http.Server{
		Addr:              opts.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if listenErr := srv.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
			errCh <- listenErr
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}
		return nil
	case err := <-errCh:
		if err == nil {
			return nil
		}
		return fmt.Errorf("listen server: %w", err)
	}
}

func buildHandler(opts types.RunOptions) (http.Handler, error) {
	if opts.Upstream != "" {
		if _, _, err := net.SplitHostPort(opts.Upstream); err != nil {
			return nil, fmt.Errorf("invalid upstream address %q: %w", opts.Upstream, err)
		}
		return newProxyHandler(opts.Upstream), nil
	}
	return newStaticHandler(opts.Root), nil
}

func withRequestHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get(headerRequestID)
		if reqID == "" {
			reqID = randomRequestID()
			r.Header.Set(headerRequestID, reqID)
		}

		w.Header().Set(headerRequestID, reqID)
		w.Header().Set(headerCacheCtl, noCacheValue)
		w.Header().Set(headerPragma, "no-cache")
		w.Header().Set(headerExpires, "0")

		next.ServeHTTP(w, r)
	})
}

func withAccessLog(next http.Handler, f *os.File, opts types.RunOptions, nextID *uint64) http.Handler {
	enc := json.NewEncoder(f)
	var mu sync.Mutex
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(lw, r)

		evt := map[string]any{
			"ts": start.Format(logTimeFormat),
			"request": map[string]any{
				"remote_ip": clientIP(r.RemoteAddr),
				"method":    r.Method,
				"uri":       requestURI(r),
				"headers": map[string]any{
					headerRequestID: []string{requestID(r, nextID)},
				},
			},
			"status":   lw.statusCode,
			"size":     lw.bytesWritten,
			"duration": time.Since(start).Seconds(),
		}
		if opts.Upstream != "" {
			evt["resp_headers"] = map[string]any{
				"X-Serve-Upstream": []string{opts.Upstream},
			}
		}
		mu.Lock()
		_ = enc.Encode(evt)
		mu.Unlock()
	})
}

func newProxyHandler(upstream string) http.Handler {
	target := &url.URL{
		Scheme: "http",
		Host:   upstream,
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		originalDirector(r)
		r.Header.Set(headerRequestID, firstOrDefault(r.Header.Values(headerRequestID)))
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.EqualFold(r.Header.Get(headerUpgrade), websocketUpgrade) {
			http.Error(w, "websocket proxying disabled", http.StatusNotImplemented)
			return
		}
		proxy.ServeHTTP(w, r)
	})
}

func clientIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

func requestURI(r *http.Request) string {
	if r.URL.RawQuery == "" {
		return r.URL.Path
	}
	return r.URL.Path + "?" + r.URL.RawQuery
}

func requestID(r *http.Request, nextID *uint64) string {
	value := firstOrDefault(r.Header.Values(headerRequestID))
	if value != "" {
		return value
	}
	id := atomic.AddUint64(nextID, 1)
	return fmt.Sprintf("req-%d", id)
}

func firstOrDefault(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func randomRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "req-fallback"
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (w *loggingResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *loggingResponseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += n
	return n, err
}
