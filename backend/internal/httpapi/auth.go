package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string
const userIDKey contextKey = "userID"

type authHandler struct { db *pgxpool.Pool }
type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	DisplayName string `json:"display_name"`
}
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	DeviceName string `json:"device_name"`
}
type userResponse struct {
	ID string `json:"id"`
	Username string `json:"username"`
	DisplayName string `json:"display_name"`
	Role string `json:"role"`
}
type authResponse struct {
	Token string `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User userResponse `json:"user"`
}

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]{3,32}$`)

func (h authHandler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(r, &req); err != nil { writeError(w, http.StatusBadRequest, "invalid_json"); return }
	req.Username = strings.TrimSpace(req.Username)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if !usernamePattern.MatchString(req.Username) || len(req.Password) < 10 || len(req.Password) > 72 {
		writeError(w, http.StatusBadRequest, "username_must_be_3_to_32_characters_and_password_10_to_72_bytes"); return
	}
	if req.DisplayName == "" { req.DisplayName = req.Username }
	if len(req.DisplayName) > 64 { writeError(w, http.StatusBadRequest, "display_name_too_long"); return }
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil { writeError(w, http.StatusInternalServerError, "password_hash_failed"); return }
	ctx := r.Context()
	tx, err := h.db.Begin(ctx)
	if err != nil { writeError(w, http.StatusInternalServerError, "database_error"); return }
	defer tx.Rollback(ctx)
	var user userResponse
	err = tx.QueryRow(ctx, `INSERT INTO users(username,password_hash,display_name) VALUES($1,$2,$3) RETURNING id::text,username,display_name,role`, req.Username, string(hash), req.DisplayName).Scan(&user.ID,&user.Username,&user.DisplayName,&user.Role)
	if err != nil {
		if strings.Contains(err.Error(), "unique") { writeError(w, http.StatusConflict, "username_taken") } else { writeError(w, http.StatusInternalServerError, "database_error") }
		return
	}
	if _, err = tx.Exec(ctx, "INSERT INTO wallet_accounts(user_id) VALUES($1)", user.ID); err != nil { writeError(w, http.StatusInternalServerError, "database_error"); return }
	token, tokenHash, err := newSessionToken()
	if err != nil { writeError(w, http.StatusInternalServerError, "session_creation_failed"); return }
	expires := time.Now().UTC().Add(30*24*time.Hour)
	if _, err = tx.Exec(ctx, "INSERT INTO sessions(user_id,token_hash,expires_at) VALUES($1,$2,$3)", user.ID, tokenHash, expires); err != nil { writeError(w, http.StatusInternalServerError, "session_creation_failed"); return }
	if err = tx.Commit(ctx); err != nil { writeError(w, http.StatusInternalServerError, "database_error"); return }
	writeJSON(w, http.StatusCreated, authResponse{Token:token,ExpiresAt:expires,User:user})
}

func (h authHandler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r,&req); err != nil { writeError(w,http.StatusBadRequest,"invalid_json"); return }
	var user userResponse
	var passwordHash string
	err := h.db.QueryRow(r.Context(), `SELECT id::text,username,display_name,role,password_hash FROM users WHERE lower(username)=lower($1) AND is_active=TRUE`, strings.TrimSpace(req.Username)).Scan(&user.ID,&user.Username,&user.DisplayName,&user.Role,&passwordHash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(passwordHash),[]byte(req.Password)) != nil { writeError(w,http.StatusUnauthorized,"invalid_credentials"); return }
	token, tokenHash, err := newSessionToken()
	if err != nil { writeError(w,http.StatusInternalServerError,"session_creation_failed"); return }
	expires := time.Now().UTC().Add(30*24*time.Hour)
	_, err = h.db.Exec(r.Context(), "INSERT INTO sessions(user_id,token_hash,device_name,expires_at) VALUES($1,$2,$3,$4)",user.ID,tokenHash,strings.TrimSpace(req.DeviceName),expires)
	if err != nil { writeError(w,http.StatusInternalServerError,"session_creation_failed"); return }
	writeJSON(w,http.StatusOK,authResponse{Token:token,ExpiresAt:expires,User:user})
}

func (h authHandler) me(w http.ResponseWriter, r *http.Request) {
	id, _ := r.Context().Value(userIDKey).(string)
	var user userResponse
	if err := h.db.QueryRow(r.Context(),"SELECT id::text,username,display_name,role FROM users WHERE id=$1",id).Scan(&user.ID,&user.Username,&user.DisplayName,&user.Role); err != nil {
		writeError(w,http.StatusNotFound,"user_not_found"); return
	}
	writeJSON(w,http.StatusOK,user)
}

func (h authHandler) logout(w http.ResponseWriter, r *http.Request) {
	raw := bearerToken(r)
	sum := sha256.Sum256([]byte(raw))
	_, err := h.db.Exec(r.Context(),"UPDATE sessions SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL",sum[:])
	if err != nil { writeError(w,http.StatusInternalServerError,"logout_failed"); return }
	w.WriteHeader(http.StatusNoContent)
}

func (h authHandler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
		raw := bearerToken(r)
		if raw == "" { writeError(w,http.StatusUnauthorized,"authentication_required"); return }
		sum := sha256.Sum256([]byte(raw))
		var userID string
		err := h.db.QueryRow(r.Context(), `SELECT user_id::text FROM sessions WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>now()`,sum[:]).Scan(&userID)
		if err != nil { writeError(w,http.StatusUnauthorized,"invalid_or_expired_session"); return }
		next.ServeHTTP(w,r.WithContext(context.WithValue(r.Context(),userIDKey,userID)))
	})
}

func bearerToken(r *http.Request) string {
	value := r.Header.Get("Authorization")
	if !strings.HasPrefix(value,"Bearer ") { return "" }
	return strings.TrimSpace(strings.TrimPrefix(value,"Bearer "))
}
func newSessionToken() (string,[]byte,error) {
	raw := make([]byte,32)
	if _,err := rand.Read(raw); err != nil { return "",nil,err }
	token := hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	return token,sum[:],nil
}
func decodeJSON(r *http.Request,dst any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body,1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}
func writeError(w http.ResponseWriter,status int,code string) { writeJSON(w,status,map[string]string{"error":code}) }
