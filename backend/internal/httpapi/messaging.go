package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type messagingHandler struct { db *pgxpool.Pool }
type createConversationRequest struct { Kind string `json:"kind"`; Title string `json:"title"`; MemberIDs []string `json:"member_ids"` }
type sendMessageRequest struct { Body string `json:"body"`; ReplyTo *string `json:"reply_to"` }
type conversationResponse struct { ID string `json:"id"`; Kind string `json:"kind"`; Title *string `json:"title"`; CreatedAt time.Time `json:"created_at"` }
type messageResponse struct { ID string `json:"id"`; ConversationID string `json:"conversation_id"`; SenderID string `json:"sender_id"`; Body string `json:"body"`; ReplyTo *string `json:"reply_to"`; CreatedAt time.Time `json:"created_at"` }

func (h messagingHandler) createConversation(w http.ResponseWriter,r *http.Request) {
	var req createConversationRequest
	if err:=decodeJSON(r,&req);err!=nil { writeError(w,http.StatusBadRequest,"invalid_json");return }
	kind:=strings.TrimSpace(req.Kind);title:=strings.TrimSpace(req.Title)
	if kind!="direct"&&kind!="group"&&kind!="channel" { writeError(w,http.StatusBadRequest,"invalid_conversation_kind");return }
	if (kind=="group"||kind=="channel")&&(title==""||len(title)>128) { writeError(w,http.StatusBadRequest,"title_required");return }
	if kind=="direct"&&len(req.MemberIDs)!=1 { writeError(w,http.StatusBadRequest,"direct_chat_requires_one_peer");return }
	if len(req.MemberIDs)>199 { writeError(w,http.StatusBadRequest,"too_many_members");return }
	creator,_:=r.Context().Value(userIDKey).(string)
	tx,err:=h.db.Begin(r.Context());if err!=nil { writeError(w,http.StatusInternalServerError,"database_error");return };defer tx.Rollback(r.Context())
	var c conversationResponse
	err=tx.QueryRow(r.Context(),`INSERT INTO conversations(kind,title,created_by) VALUES($1,NULLIF($2,''),$3) RETURNING id::text,kind,title,created_at`,kind,title,creator).Scan(&c.ID,&c.Kind,&c.Title,&c.CreatedAt)
	if err!=nil { writeError(w,http.StatusInternalServerError,"conversation_create_failed");return }
	if _,err=tx.Exec(r.Context(),`INSERT INTO conversation_members(conversation_id,user_id,member_role) VALUES($1,$2,'owner')`,c.ID,creator);err!=nil { writeError(w,http.StatusInternalServerError,"conversation_create_failed");return }
	for _,memberID:=range req.MemberIDs {
		memberID=strings.TrimSpace(memberID);if memberID==""||memberID==creator { continue }
		if _,err=tx.Exec(r.Context(),`INSERT INTO conversation_members(conversation_id,user_id,member_role) SELECT $1,id,'member' FROM users WHERE id=$2 AND is_active=TRUE ON CONFLICT DO NOTHING`,c.ID,memberID);err!=nil { writeError(w,http.StatusBadRequest,"invalid_member");return }
	}
	if kind=="direct" {
		var count int
		if err=tx.QueryRow(r.Context(),"SELECT count(*) FROM conversation_members WHERE conversation_id=$1",c.ID).Scan(&count);err!=nil||count!=2 { writeError(w,http.StatusBadRequest,"peer_not_found");return }
	}
	if err=tx.Commit(r.Context());err!=nil { writeError(w,http.StatusInternalServerError,"conversation_create_failed");return }
	writeJSON(w,http.StatusCreated,c)
}

func (h messagingHandler) listConversations(w http.ResponseWriter,r *http.Request) {
	userID,_:=r.Context().Value(userIDKey).(string)
	rows,err:=h.db.Query(r.Context(),`SELECT c.id::text,c.kind,c.title,c.created_at FROM conversations c JOIN conversation_members m ON m.conversation_id=c.id WHERE m.user_id=$1 ORDER BY c.created_at DESC LIMIT 200`,userID)
	if err!=nil { writeError(w,http.StatusInternalServerError,"conversation_list_failed");return };defer rows.Close()
	items:=make([]conversationResponse,0)
	for rows.Next(){var c conversationResponse;if err:=rows.Scan(&c.ID,&c.Kind,&c.Title,&c.CreatedAt);err!=nil {writeError(w,http.StatusInternalServerError,"conversation_list_failed");return};items=append(items,c)}
	if rows.Err()!=nil {writeError(w,http.StatusInternalServerError,"conversation_list_failed");return}
	writeJSON(w,http.StatusOK,map[string]any{"conversations":items})
}

func (h messagingHandler) listMessages(w http.ResponseWriter,r *http.Request) {
	userID,_:=r.Context().Value(userIDKey).(string);conversationID:=r.PathValue("conversationID")
	var member bool
	if err:=h.db.QueryRow(r.Context(),"SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id=$1 AND user_id=$2)",conversationID,userID).Scan(&member);err!=nil||!member {writeError(w,http.StatusNotFound,"conversation_not_found");return}
	rows,err:=h.db.Query(r.Context(),`SELECT id::text,conversation_id::text,sender_id::text,body,reply_to::text,created_at FROM messages WHERE conversation_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 100`,conversationID)
	if err!=nil {writeError(w,http.StatusInternalServerError,"message_list_failed");return};defer rows.Close()
	items:=make([]messageResponse,0)
	for rows.Next(){var m messageResponse;if err:=rows.Scan(&m.ID,&m.ConversationID,&m.SenderID,&m.Body,&m.ReplyTo,&m.CreatedAt);err!=nil {writeError(w,http.StatusInternalServerError,"message_list_failed");return};items=append(items,m)}
	if rows.Err()!=nil {writeError(w,http.StatusInternalServerError,"message_list_failed");return}
	writeJSON(w,http.StatusOK,map[string]any{"messages":items})
}

func (h messagingHandler) sendMessage(w http.ResponseWriter,r *http.Request) {
	userID,_:=r.Context().Value(userIDKey).(string);conversationID:=r.PathValue("conversationID")
	var req sendMessageRequest
	if err:=decodeJSON(r,&req);err!=nil {writeError(w,http.StatusBadRequest,"invalid_json");return}
	req.Body=strings.TrimSpace(req.Body)
	if req.Body==""||len(req.Body)>16000 {writeError(w,http.StatusBadRequest,"message_body_must_be_1_to_16000_bytes");return}
	var m messageResponse
	err:=h.db.QueryRow(r.Context(),`INSERT INTO messages(conversation_id,sender_id,body,reply_to) SELECT c.id,$2,$3,$4 FROM conversations c JOIN conversation_members cm ON cm.conversation_id=c.id AND cm.user_id=$2 WHERE c.id=$1 RETURNING id::text,conversation_id::text,sender_id::text,body,reply_to::text,created_at`,conversationID,userID,req.Body,req.ReplyTo).Scan(&m.ID,&m.ConversationID,&m.SenderID,&m.Body,&m.ReplyTo,&m.CreatedAt)
	if err!=nil {writeError(w,http.StatusNotFound,"conversation_not_found_or_reply_invalid");return}
	writeJSON(w,http.StatusCreated,m)
}
