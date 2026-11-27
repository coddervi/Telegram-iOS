package httpapi
import (
 "crypto/rand"
 "encoding/json"
 "fmt"
 "io"
 "net/http"
 "strings"
 "time"
 "github.com/jackc/pgx/v5"
 "github.com/jackc/pgx/v5/pgxpool"
)
type telegramBotHandler struct { db *pgxpool.Pool; token,secret string }
type telegramUpdate struct { Message *struct { Chat struct { ID int64 `json:"id"` } `json:"chat"`; Text string `json:"text"` } `json:"message"` }
func(h telegramBotHandler)webhook(w http.ResponseWriter,r *http.Request){
 if h.token==""||h.secret==""||r.Header.Get("X-Telegram-Bot-Api-Secret-Token")!=h.secret{http.Error(w,"forbidden",403);return}
 var u telegramUpdate;err:=json.NewDecoder(io.LimitReader(r.Body,1<<20)).Decode(&u);if err!=nil||u.Message==nil{w.WriteHeader(200);return}
 text:=strings.TrimSpace(u.Message.Text)
 if text!="/start"&&text!="/number"&&text!="Получить номер"{_ = h.send(r,u.Message.Chat.ID,"Отправьте /start, чтобы получить внутренний номер аккаунта.");w.WriteHeader(200);return}
 number,err:=h.issueNumber(r,u.Message.Chat.ID);if err!=nil{writeError(w,500,"number_issue_failed");return}
 if err=h.send(r,u.Message.Chat.ID,"Ваш внутренний номер Private Platform:\n\n"+number+"\n\nВведите его в приложении. Не передавайте никому коды подтверждения.");err!=nil{writeError(w,502,"telegram_delivery_failed");return}
 w.WriteHeader(200)
}
func(h telegramBotHandler)issueNumber(r *http.Request,chatID int64)(string,error){
 var existing string;err:=h.db.QueryRow(r.Context(),"SELECT number FROM platform_numbers WHERE telegram_chat_id=$1",chatID).Scan(&existing);if err==nil{return existing,nil};if err!=pgx.ErrNoRows{return "",err}
 for i:=0;i<8;i++{b:=make([]byte,4);if _,err=rand.Read(b);err!=nil{return "",err};n:=uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]);number:=fmt.Sprintf("PP-%08d",n%100000000)
 _,err=h.db.Exec(r.Context(),"INSERT INTO platform_numbers(number,telegram_chat_id) VALUES($1,$2) ON CONFLICT DO NOTHING",number,chatID);if err!=nil{return "",err}
 err=h.db.QueryRow(r.Context(),"SELECT number FROM platform_numbers WHERE telegram_chat_id=$1",chatID).Scan(&existing);if err==nil{return existing,nil};if err!=pgx.ErrNoRows{return "",err}}
 return "",fmt.Errorf("could not allocate unique number")
}
func(h telegramBotHandler)send(r *http.Request,chatID int64,message string)error{
 body,err:=json.Marshal(map[string]any{"chat_id":chatID,"text":message});if err!=nil{return err}
 req,err:=http.NewRequestWithContext(r.Context(),http.MethodPost,"https://api.telegram.org/bot"+h.token+"/sendMessage",strings.NewReader(string(body)));if err!=nil{return err};req.Header.Set("Content-Type","application/json")
 resp,err:=(&http.Client{Timeout:8*time.Second}).Do(req);if err!=nil{return err};defer resp.Body.Close();if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("telegram returned %s",resp.Status)};return nil
}
