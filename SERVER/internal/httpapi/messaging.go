package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type messagingHandler struct { db *pgxpool.Pool }
type createConversationRequest struct { Kind string `json:"kind"`; Title string `json:"title"`; MemberIDs []string `json:"member_ids"` }
type sendMessageRequest struct { Body string `json:"body"`; ReplyTo *string `json:"reply_to"`; MediaIDs []string `json:"media_ids"` }
type conversationResponse struct { ID string `json:"id"`; Kind string `json:"kind"`; Title *string `json:"title"`; CreatedAt time.Time `json:"created_at"` }
type messageResponse struct { ID string `json:"id"`; ConversationID string `json:"conversation_id"`; SenderID string `json:"sender_id"`; Body string `json:"body"`; ReplyTo *string `json:"reply_to"`; MediaIDs []string `json:"media_ids"`; CreatedAt time.Time `json:"created_at"` }

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
		var active bool
		if err=tx.QueryRow(r.Context(),"SELECT is_active FROM users WHERE id=$1",memberID).Scan(&active);err!=nil||!active { writeError(w,http.StatusBadRequest,"invalid_member");return }
		if _,err=tx.Exec(r.Context(),`INSERT INTO conversation_members(conversation_id,user_id,member_role) VALUES($1,$2,'member') ON CONFLICT DO NOTHING`,c.ID,memberID);err!=nil { writeError(w,http.StatusBadRequest,"invalid_member");return }
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
	rows,err:=h.db.Query(r.Context(),`SELECT m.id::text,m.conversation_id::text,m.sender_id::text,m.body,m.reply_to::text,m.created_at,COALESCE((SELECT array_agg(mm.media_id::text) FROM message_media mm WHERE mm.message_id=m.id),ARRAY[]::text[]) FROM messages m WHERE m.conversation_id=$1 AND m.deleted_at IS NULL ORDER BY m.created_at DESC LIMIT 100`,conversationID)
	if err!=nil {writeError(w,http.StatusInternalServerError,"message_list_failed");return};defer rows.Close()
	items:=make([]messageResponse,0)
	for rows.Next(){var m messageResponse;if err:=rows.Scan(&m.ID,&m.ConversationID,&m.SenderID,&m.Body,&m.ReplyTo,&m.CreatedAt,&m.MediaIDs);err!=nil {writeError(w,http.StatusInternalServerError,"message_list_failed");return};items=append(items,m)}
	if rows.Err()!=nil {writeError(w,http.StatusInternalServerError,"message_list_failed");return}
	writeJSON(w,http.StatusOK,map[string]any{"messages":items})
}

func (h messagingHandler) sendMessage(w http.ResponseWriter,r *http.Request) {
	userID,_:=r.Context().Value(userIDKey).(string);conversationID:=r.PathValue("conversationID")
	var req sendMessageRequest
	if err:=decodeJSON(r,&req);err!=nil {writeError(w,http.StatusBadRequest,"invalid_json");return}
	req.Body=strings.TrimSpace(req.Body)
	if (req.Body==""&&len(req.MediaIDs)==0)||len(req.Body)>16000||len(req.MediaIDs)>10 {writeError(w,http.StatusBadRequest,"message_requires_text_or_media_and_has_limits");return}
	tx,err:=h.db.Begin(r.Context());if err!=nil {writeError(w,http.StatusInternalServerError,"database_error");return};defer tx.Rollback(r.Context())
	var m messageResponse
	err=tx.QueryRow(r.Context(),`INSERT INTO messages(conversation_id,sender_id,body,reply_to) SELECT c.id,$2,$3,$4 FROM conversations c JOIN conversation_members cm ON cm.conversation_id=c.id AND cm.user_id=$2 WHERE c.id=$1 AND ($4::uuid IS NULL OR EXISTS(SELECT 1 FROM messages rm WHERE rm.id=$4 AND rm.conversation_id=c.id)) RETURNING id::text,conversation_id::text,sender_id::text,body,reply_to::text,created_at`,conversationID,userID,req.Body,req.ReplyTo).Scan(&m.ID,&m.ConversationID,&m.SenderID,&m.Body,&m.ReplyTo,&m.CreatedAt)
	if err!=nil {writeError(w,http.StatusNotFound,"conversation_not_found_or_reply_invalid");return}
	for _,mediaID:=range req.MediaIDs {
		tag,attachErr:=tx.Exec(r.Context(),`INSERT INTO message_media(message_id,media_id) SELECT $1,id FROM media_objects WHERE id=$2 AND owner_id=$3 ON CONFLICT DO NOTHING`,m.ID,mediaID,userID)
		if attachErr!=nil||tag.RowsAffected()!=1 {writeError(w,http.StatusBadRequest,"media_not_found_or_not_owned");return}
	}
	if err=tx.Commit(r.Context());err!=nil {writeError(w,http.StatusInternalServerError,"message_send_failed");return}
	m.MediaIDs=append([]string{},req.MediaIDs...)
	writeJSON(w,http.StatusCreated,m)
}
