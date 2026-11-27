package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type healthResponse struct {
	Status string `json:"status"`
	Service string `json:"service"`
	Time string `json:"time"`
}

func NewRouter(db *pgxpool.Pool) http.Handler {
	mux := http.NewServeMux()
	auth := authHandler{db:db}
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if err := db.Ping(ctx); err != nil { writeJSON(w,http.StatusServiceUnavailable,map[string]string{"status":"unavailable"}); return }
		writeJSON(w, http.StatusOK, healthResponse{Status:"ok",Service:"private-platform-api",Time:time.Now().UTC().Format(time.RFC3339)})
	})
	mux.HandleFunc("GET /v1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"name":"Private Platform API","version":"0.2.0","status":"development"})
	})
	mux.HandleFunc("POST /v1/auth/register",auth.register)
	mux.HandleFunc("POST /v1/auth/login",auth.login)
	mux.Handle("GET /v1/auth/me",auth.requireAuth(http.HandlerFunc(auth.me)))
	mux.Handle("POST /v1/auth/logout",auth.requireAuth(http.HandlerFunc(auth.logout)))
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
