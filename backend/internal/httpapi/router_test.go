package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVersionEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1", nil)
	rec := httptest.NewRecorder()
	NewRouter(nil, "").ServeHTTP(rec, req)
	if rec.Code != http.StatusOK { t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code) }
	if rec.Header().Get("Cache-Control") != "no-store" { t.Fatal("expected no-store cache policy") }
}
