package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"

	"github.com/caoyb888/zhiyuche/apps/api/internal/config"
	"github.com/caoyb888/zhiyuche/apps/api/pkg/httpx"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{Env: "test"}
	cfg.HTTP.Addr = ":0"
	cfg.HTTP.CORSOrigins = []string{"http://localhost:20173"}
	return New(Deps{Cfg: cfg, Log: zerolog.Nop()})
}

func TestHealth(t *testing.T) {
	s := newTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatalf("missing X-Request-ID header")
	}
	var env struct {
		httpx.Envelope
		Data healthResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Code != httpx.CodeOK || env.Data.Status != "ok" || env.Data.Env != "test" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
}

func TestReadyWithoutDeps(t *testing.T) {
	s := newTestServer(t)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestNoRoute(t *testing.T) {
	s := newTestServer(t)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var env httpx.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Code != httpx.CodeNotFound {
		t.Fatalf("code = %d, want %d", env.Code, httpx.CodeNotFound)
	}
}

func TestCORSPreflight(t *testing.T) {
	s := newTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://localhost:20173")
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:20173" {
		t.Fatalf("allow-origin = %q", got)
	}
}
