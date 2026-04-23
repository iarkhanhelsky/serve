package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrowseDirsFirst(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "b-dir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "a-file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "a-dir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	h := newStaticHandler(root)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	body := rec.Body.String()
	firstDir := strings.Index(body, ">a-dir<")
	secondDir := strings.Index(body, ">b-dir<")
	filePos := strings.Index(body, ">a-file.txt<")
	if firstDir < 0 || secondDir < 0 || filePos < 0 {
		t.Fatalf("expected all entries in browse output, got: %s", body)
	}
	if !(firstDir < secondDir && secondDir < filePos) {
		t.Fatalf("expected dirs first ordering, got body: %s", body)
	}
}

func TestStaticFileServed(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	h := newStaticHandler(root)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/hello.txt", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	b, _ := io.ReadAll(rec.Body)
	if string(b) != "hello" {
		t.Fatalf("unexpected body: %s", string(b))
	}
}
