package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pulseorperish/internal/delete"
	"pulseorperish/internal/monitor"
	"pulseorperish/internal/state"

	"github.com/rs/zerolog"
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	st := state.NewStore(filepath.Join(t.TempDir(), "state"))
	del := delete.NewSafeDeleter(zerolog.Nop(), false)
	m := monitor.NewService(zerolog.Nop(), st, del, time.Minute, false, filepath.Join(t.TempDir(), "data"))
	if err := m.LoadInitialState(); err != nil {
		t.Fatal(err)
	}
	s := NewServer(zerolog.Nop(), "secret", m, "test", "2026-07-10", "abc123")
	return s.Router()
}

func TestHealthNoAuth(t *testing.T) {
	h := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestIndexNoAuth(t *testing.T) {
	h := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if contentType := w.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
		t.Fatalf("expected html content type, got %q", contentType)
	}
	if !strings.Contains(body, "@media (prefers-color-scheme: dark)") {
		t.Fatal("expected index page to support system dark mode")
	}
	if !strings.Contains(body, "color-scheme: light dark;") {
		t.Fatal("expected index page to advertise light and dark color schemes")
	}
}

func TestStatusNoAuth(t *testing.T) {
	h := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAliveRequiresPassword(t *testing.T) {
	h := newTestServer(t)
	r := httptest.NewRequest(http.MethodPost, "/alive", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func assertAliveSuccess(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json response, got %v", err)
	}
	v, ok := payload["dryRun"]
	if !ok {
		t.Fatal("expected dryRun field in /alive response")
	}
	if b, ok := v.(bool); !ok || b {
		t.Fatalf("expected dryRun=false in test server, got %#v", v)
	}
	v, ok = payload["timeRemainingMinutes"]
	if !ok {
		t.Fatal("expected timeRemainingMinutes field in /alive response")
	}
	if minutes, ok := v.(float64); !ok || minutes <= 0 {
		t.Fatalf("expected positive timeRemainingMinutes in /alive response, got %#v", v)
	}
}

func TestAliveWithPasswordField(t *testing.T) {
	h := newTestServer(t)
	r := httptest.NewRequest(http.MethodPost, "/alive", bytes.NewBufferString(`{"password":"secret"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assertAliveSuccess(t, w)
}

func TestAliveWithForm(t *testing.T) {
	h := newTestServer(t)
	r := httptest.NewRequest(http.MethodPost, "/alive", strings.NewReader("password=secret"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assertAliveSuccess(t, w)
}

func TestAliveWithBearer(t *testing.T) {
	h := newTestServer(t)
	r := httptest.NewRequest(http.MethodPost, "/alive", nil)
	r.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assertAliveSuccess(t, w)
}
