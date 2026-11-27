package httpapi

import (
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type usersHandler struct { db *pgxpool.Pool }

func (h usersHandler) search(w http.ResponseWriter,r *http.Request) {
	query:=strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query)<2||len(query)>64 { writeError(w,http.StatusBadRequest,"query_must_be_2_to_64_characters");return }
	query=strings.NewReplacer("\\","\\\\","%","\\%","_","\\_").Replace(query)
	rows,err:=h.db.Query(r.Context(),`SELECT id::text,username,display_name,bio FROM users WHERE is_active=TRUE AND (username ILIKE $1 ESCAPE '\\' OR display_name ILIKE $1 ESCAPE '\\') ORDER BY username LIMIT 25`,"%"+query+"%")
	if err!=nil { writeError(w,http.StatusInternalServerError,"user_search_failed");return };defer rows.Close()
	type result struct { ID string `json:"id"`; Username string `json:"username"`; DisplayName string `json:"display_name"`; Bio string `json:"bio"` }
	users:=make([]result,0)
	for rows.Next(){var u result;if err:=rows.Scan(&u.ID,&u.Username,&u.DisplayName,&u.Bio);err!=nil {writeError(w,http.StatusInternalServerError,"user_search_failed");return};users=append(users,u)}
	if rows.Err()!=nil {writeError(w,http.StatusInternalServerError,"user_search_failed");return}
	writeJSON(w,http.StatusOK,map[string]any{"users":users})
}
