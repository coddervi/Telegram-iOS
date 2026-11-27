package httpapi

import (
 "crypto/rand"
 "crypto/sha256"
 "crypto/subtle"
 "fmt"
 "net/http"
 "strings"
 "time"

 "github.com/jackc/pgx/v5/pgxpool"
 "golang.org/x/crypto/bcrypt"
)

type registrationHandler struct { db *pgxpool.Pool; bot telegramBotHandler }
type requestCodeRequest struct { Number string `json:"number"`; Username string `json:"username"`; Password string `json:"password"`; DisplayName string `json:"display_name"` }
type verifyCodeRequest struct { Number string `json:"number"`; Code string `json:"code"` }

func (h registrationHandler) requestCode(w http.ResponseWriter,r *http.Request) {
 var req requestCodeRequest
 if err:=decodeJSON(r,&req); err!=nil { writeError(w,400,"invalid_json");return }
 req.Number=strings.ToUpper(strings.TrimSpace(req.Number)); req.Username=strings.TrimSpace(req.Username); req.DisplayName=strings.TrimSpace(req.DisplayName)
 if !usernamePattern.MatchString(req.Username)||len(req.Password)<10||len(req.Password)>72 {writeError(w,400,"invalid_registration_details");return}
 if req.DisplayName=="" {req.DisplayName=req.Username}; if len(req.DisplayName)>64 {writeError(w,400,"display_name_too_long");return}
 var id string; var chatID int64; var linked *string
 err:=h.db.QueryRow(r.Context(),"SELECT id::text,telegram_chat_id,linked_user_id::text FROM platform_numbers WHERE number=$1",req.Number).Scan(&id,&chatID,&linked)
 if err!=nil||linked!=nil {writeError(w,400,"invalid_or_used_platform_number");return}
 var last time.Time
 err=h.db.QueryRow(r.Context(),"SELECT requested_at FROM registration_codes WHERE number_id=$1",id).Scan(&last)
 if err==nil && time.Since(last)<time.Minute {writeError(w,429,"code_cooldown");return}
 hash,err:=bcrypt.GenerateFromPassword([]byte(req.Password),bcrypt.DefaultCost);if err!=nil{writeError(w,500,"password_hash_failed");return}
 raw:=make([]byte,4);if _,err=rand.Read(raw);err!=nil{writeError(w,500,"code_generation_failed");return}
 n:=uint32(raw[0])<<24|uint32(raw[1])<<16|uint32(raw[2])<<8|uint32(raw[3]); codeText:=fmt.Sprintf("%06d",n%1000000); sum:=sha256.Sum256([]byte(codeText))
 _,err=h.db.Exec(r.Context(),`INSERT INTO registration_codes(number_id,username,display_name,password_hash,code_hash,expires_at,requested_at,attempts) VALUES($1,$2,$3,$4,$5,now()+interval '5 minutes',now(),0) ON CONFLICT(number_id) DO UPDATE SET username=EXCLUDED.username,display_name=EXCLUDED.display_name,password_hash=EXCLUDED.password_hash,code_hash=EXCLUDED.code_hash,expires_at=EXCLUDED.expires_at,requested_at=now(),attempts=0`,id,req.Username,req.DisplayName,string(hash),sum[:])
 if err!=nil{writeError(w,500,"code_store_failed");return}
 if err=h.bot.send(r,chatID,"Код регистрации Private Platform: "+codeText+"\n\nДействует 5 минут. Не сообщайте его другим.");err!=nil{writeError(w,502,"telegram_delivery_failed");return}
 writeJSON(w,200,map[string]string{"status":"code_sent"})
}
func (h registrationHandler) verifyCode(w http.ResponseWriter,r *http.Request) {
 var req verifyCodeRequest;if err:=decodeJSON(r,&req);err!=nil{writeError(w,400,"invalid_json");return}
 req.Number=strings.ToUpper(strings.TrimSpace(req.Number));req.Code=strings.TrimSpace(req.Code)
 tx,err:=h.db.Begin(r.Context());if err!=nil{writeError(w,500,"database_error");return};defer tx.Rollback(r.Context())
 var id,username,display,passwordHash string;var stored []byte;var expires time.Time;var attempts int
 err=tx.QueryRow(r.Context(),`SELECT p.id::text,c.username,c.display_name,c.password_hash,c.code_hash,c.expires_at,c.attempts FROM platform_numbers p JOIN registration_codes c ON c.number_id=p.id WHERE p.number=$1 AND p.linked_user_id IS NULL FOR UPDATE OF p,c`,req.Number).Scan(&id,&username,&display,&passwordHash,&stored,&expires,&attempts)
 if err!=nil||time.Now().After(expires)||attempts>=5 {writeError(w,400,"code_expired_or_invalid");return}
 sum:=sha256.Sum256([]byte(req.Code));if len(req.Code)!=6||subtle.ConstantTimeCompare(sum[:],stored)!=1{_,_=tx.Exec(r.Context(),"UPDATE registration_codes SET attempts=attempts+1 WHERE number_id=$1",id);writeError(w,400,"wrong_code");return}
 var user userResponse
 err=tx.QueryRow(r.Context(),`INSERT INTO users(username,password_hash,display_name) VALUES($1,$2,$3) RETURNING id::text,username,display_name,role`,username,passwordHash,display).Scan(&user.ID,&user.Username,&user.DisplayName,&user.Role)
 if err!=nil{if strings.Contains(err.Error(),"unique"){writeError(w,409,"username_taken")}else{writeError(w,500,"database_error")};return}
 if _,err=tx.Exec(r.Context(),"INSERT INTO wallet_accounts(user_id) VALUES($1)",user.ID);err!=nil{writeError(w,500,"database_error");return}
 if _,err=tx.Exec(r.Context(),"UPDATE platform_numbers SET linked_user_id=$1 WHERE id=$2",user.ID,id);err!=nil{writeError(w,500,"database_error");return}
 if _,err=tx.Exec(r.Context(),"DELETE FROM registration_codes WHERE number_id=$1",id);err!=nil{writeError(w,500,"database_error");return}
 token,tokenHash,err:=newSessionToken();if err!=nil{writeError(w,500,"session_creation_failed");return};expiresAt:=time.Now().UTC().Add(30*24*time.Hour)
 if _,err=tx.Exec(r.Context(),"INSERT INTO sessions(user_id,token_hash,expires_at) VALUES($1,$2,$3)",user.ID,tokenHash,expiresAt);err!=nil{writeError(w,500,"session_creation_failed");return}
 if err=tx.Commit(r.Context());err!=nil{writeError(w,500,"database_error");return}
 writeJSON(w,201,authResponse{Token:token,ExpiresAt:expiresAt,User:user})
}
