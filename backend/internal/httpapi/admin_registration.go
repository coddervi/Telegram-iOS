package httpapi

import (
    "net/http"
    "strconv"
    "time"
)

func (h registrationHandler) adminOverview(w http.ResponseWriter, r *http.Request) {
    limit := 100
    if raw := r.URL.Query().Get("limit"); raw != "" {
        n, err := strconv.Atoi(raw)
        if err != nil || n < 1 || n > 200 {
            writeError(w, http.StatusBadRequest, "invalid_limit")
            return
        }
        limit = n
    }

    type requestItem struct {
        Number string    `json:"number"`
        Username string  `json:"username"`
        RequestedAt time.Time `json:"requested_at"`
        ExpiresAt time.Time `json:"expires_at"`
        Attempts int     `json:"attempts"`
        Status string    `json:"status"`
        LastEventAt *time.Time `json:"last_event_at,omitempty"`
    }
    rows, err := h.db.Query(r.Context(), `
        SELECT p.number, c.username, c.requested_at, c.expires_at, c.attempts,
               COALESCE((
                   SELECT e.event_type FROM registration_code_events e
                   WHERE e.number_id = p.id
                   ORDER BY e.created_at DESC LIMIT 1
               ), 'requested'),
               (
                   SELECT e.created_at FROM registration_code_events e
                   WHERE e.number_id = p.id
                   ORDER BY e.created_at DESC LIMIT 1
               )
        FROM registration_codes c
        JOIN platform_numbers p ON p.id = c.number_id
        ORDER BY c.requested_at DESC
        LIMIT $1`, limit)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "registration_codes_failed")
        return
    }
    requests := make([]requestItem, 0)
    for rows.Next() {
        var item requestItem
        var latest string
        if err := rows.Scan(&item.Number, &item.Username, &item.RequestedAt, &item.ExpiresAt, &item.Attempts, &latest, &item.LastEventAt); err != nil {
            rows.Close()
            writeError(w, http.StatusInternalServerError, "registration_codes_failed")
            return
        }
        item.Status = latest
        if item.Attempts >= 5 {
            item.Status = "locked"
        } else if time.Now().After(item.ExpiresAt) {
            item.Status = "expired"
        }
        requests = append(requests, item)
    }
    if err := rows.Err(); err != nil {
        rows.Close()
        writeError(w, http.StatusInternalServerError, "registration_codes_failed")
        return
    }
    rows.Close()

    type eventItem struct {
        ID string `json:"id"`
        Number string `json:"number"`
        Username string `json:"username"`
        Event string `json:"event"`
        Details map[string]any `json:"details"`
        CreatedAt time.Time `json:"created_at"`
    }
    eventsRows, err := h.db.Query(r.Context(), `
        SELECT id::text, number_snapshot, username, event_type, details, created_at
        FROM registration_code_events
        ORDER BY created_at DESC LIMIT $1`, limit)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "registration_events_failed")
        return
    }
    defer eventsRows.Close()
    events := make([]eventItem, 0)
    for eventsRows.Next() {
        var item eventItem
        if err := eventsRows.Scan(&item.ID, &item.Number, &item.Username, &item.Event, &item.Details, &item.CreatedAt); err != nil {
            writeError(w, http.StatusInternalServerError, "registration_events_failed")
            return
        }
        events = append(events, item)
    }
    if eventsRows.Err() != nil {
        writeError(w, http.StatusInternalServerError, "registration_events_failed")
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{
        "requests": requests,
        "events": events,
        "note": "Verification codes are never exposed; only hashes are stored.",
    })
}
