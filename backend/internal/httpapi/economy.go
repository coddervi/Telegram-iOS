package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgxpool"
)

type economyHandler struct { db *pgxpool.Pool }
type adjustWalletRequest struct { UserID string `json:"user_id"`; Amount int64 `json:"amount"`; Reason string `json:"reason"`; Note string `json:"note"` }
type createGiftRequest struct { Code string `json:"code"`; Title string `json:"title"`; Description string `json:"description"`; Rarity string `json:"rarity"`; PriceStars int64 `json:"price_stars"`; UpgradeCostStars int64 `json:"upgrade_cost_stars"`; MaxLevel int `json:"max_level"`; MaxSupply *int64 `json:"max_supply"` }
type giftResponse struct { ID string `json:"id"`; Code string `json:"code"`; Title string `json:"title"`; Description string `json:"description"`; Rarity string `json:"rarity"`; PriceStars int64 `json:"price_stars"`; UpgradeCostStars int64 `json:"upgrade_cost_stars"`; MaxLevel int `json:"max_level"`; MaxSupply *int64 `json:"max_supply"`; IssuedCount int64 `json:"issued_count"` }
type purchaseGiftResponse struct { GiftID string `json:"gift_id"`; SerialNumber int64 `json:"serial_number"`; Level int `json:"level"`; Balance int64 `json:"balance"` }

func (h economyHandler) wallet(w http.ResponseWriter,r *http.Request) {
	userID,_:=r.Context().Value(userIDKey).(string)
	var balance int64
	if err:=h.db.QueryRow(r.Context(),"SELECT balance FROM wallet_accounts WHERE user_id=$1",userID).Scan(&balance);err!=nil { writeError(w,http.StatusNotFound,"wallet_not_found");return }
	rows,err:=h.db.Query(r.Context(),`SELECT id::text,amount,balance_after,reason,note,created_at FROM wallet_transactions WHERE user_id=$1 ORDER BY created_at DESC LIMIT 50`,userID)
	if err!=nil { writeError(w,http.StatusInternalServerError,"wallet_history_failed");return };defer rows.Close()
	type entry struct { ID string `json:"id"`; Amount int64 `json:"amount"`; BalanceAfter int64 `json:"balance_after"`; Reason string `json:"reason"`; Note string `json:"note"`; CreatedAt time.Time `json:"created_at"` }
	history:=make([]entry,0)
	for rows.Next(){ var e entry; if err:=rows.Scan(&e.ID,&e.Amount,&e.BalanceAfter,&e.Reason,&e.Note,&e.CreatedAt);err!=nil { writeError(w,http.StatusInternalServerError,"wallet_history_failed");return };history=append(history,e) }
	if rows.Err()!=nil { writeError(w,http.StatusInternalServerError,"wallet_history_failed");return }
	writeJSON(w,http.StatusOK,map[string]any{"currency":"internal_stars","balance":balance,"transactions":history})
}

func (h economyHandler) adjustWallet(w http.ResponseWriter,r *http.Request) {
	var req adjustWalletRequest
	if err:=decodeJSON(r,&req);err!=nil { writeError(w,http.StatusBadRequest,"invalid_json");return }
	req.Reason=strings.TrimSpace(req.Reason)
	if req.Amount==0||req.Reason==""||len(req.Reason)>32 { writeError(w,http.StatusBadRequest,"invalid_adjustment");return }
	key:=strings.TrimSpace(r.Header.Get("Idempotency-Key"));if key==""||len(key)>128 { writeError(w,http.StatusBadRequest,"idempotency_key_required");return }
	actor,_:=r.Context().Value(userIDKey).(string)
	tx,err:=h.db.Begin(r.Context());if err!=nil { writeError(w,http.StatusInternalServerError,"database_error");return };defer tx.Rollback(r.Context())
	var balance int64
	if err=tx.QueryRow(r.Context(),"SELECT balance FROM wallet_accounts WHERE user_id=$1 FOR UPDATE",req.UserID).Scan(&balance);err!=nil { writeError(w,http.StatusNotFound,"wallet_not_found");return }
	if balance+req.Amount<0 { writeError(w,http.StatusConflict,"insufficient_balance");return }
	newBalance:=balance+req.Amount
	if _,err=tx.Exec(r.Context(),"UPDATE wallet_accounts SET balance=$2,updated_at=now() WHERE user_id=$1",req.UserID,newBalance);err!=nil { writeError(w,http.StatusInternalServerError,"database_error");return }
	_,err=tx.Exec(r.Context(),`INSERT INTO wallet_transactions(user_id,amount,balance_after,reason,idempotency_key,actor_id,note) VALUES($1,$2,$3,$4,$5,$6,$7)`,req.UserID,req.Amount,newBalance,req.Reason,key,actor,req.Note)
	if err!=nil { writeError(w,http.StatusConflict,"duplicate_or_invalid_transaction");return }
	_,err=tx.Exec(r.Context(),`INSERT INTO admin_audit_events(actor_id,action,target_user_id,details) VALUES($1,'wallet.adjust',$2,jsonb_build_object('amount',$3,'reason',$4,'note',$5))`,actor,req.UserID,req.Amount,req.Reason,req.Note)
	if err!=nil { writeError(w,http.StatusInternalServerError,"audit_failed");return }
	if err=tx.Commit(r.Context());err!=nil { writeError(w,http.StatusInternalServerError,"database_error");return }
	writeJSON(w,http.StatusOK,map[string]any{"user_id":req.UserID,"balance":newBalance})
}

func (h economyHandler) listGifts(w http.ResponseWriter,r *http.Request) {
	rows,err:=h.db.Query(r.Context(),`SELECT id::text,code,title,description,rarity,price_stars,upgrade_cost_stars,max_level,max_supply,issued_count FROM gift_definitions WHERE is_active=TRUE ORDER BY price_stars,created_at DESC`)
	if err!=nil { writeError(w,http.StatusInternalServerError,"gift_catalog_failed");return };defer rows.Close()
	items:=make([]giftResponse,0)
	for rows.Next(){ var g giftResponse;if err:=rows.Scan(&g.ID,&g.Code,&g.Title,&g.Description,&g.Rarity,&g.PriceStars,&g.UpgradeCostStars,&g.MaxLevel,&g.MaxSupply,&g.IssuedCount);err!=nil { writeError(w,http.StatusInternalServerError,"gift_catalog_failed");return };items=append(items,g) }
	if rows.Err()!=nil { writeError(w,http.StatusInternalServerError,"gift_catalog_failed");return }
	writeJSON(w,http.StatusOK,map[string]any{"gifts":items})
}

func (h economyHandler) createGift(w http.ResponseWriter,r *http.Request) {
	var req createGiftRequest
	if err:=decodeJSON(r,&req);err!=nil { writeError(w,http.StatusBadRequest,"invalid_json");return }
	req.Code=strings.TrimSpace(req.Code);req.Title=strings.TrimSpace(req.Title);req.Rarity=strings.TrimSpace(req.Rarity)
	valid:=map[string]bool{"common":true,"uncommon":true,"rare":true,"epic":true,"legendary":true}
	if req.Code==""||len(req.Code)>48||req.Title==""||len(req.Title)>96||len(req.Description)>2000||req.PriceStars<0||req.UpgradeCostStars<0||req.MaxLevel<1||req.MaxLevel>100||!valid[req.Rarity]||(req.MaxSupply!=nil&&*req.MaxSupply<1) { writeError(w,http.StatusBadRequest,"invalid_gift_definition");return }
	actor,_:=r.Context().Value(userIDKey).(string)
	var g giftResponse
	err:=h.db.QueryRow(r.Context(),`INSERT INTO gift_definitions(code,title,description,rarity,price_stars,upgrade_cost_stars,max_level,max_supply,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id::text,code,title,description,rarity,price_stars,upgrade_cost_stars,max_level,max_supply,issued_count`,req.Code,req.Title,req.Description,req.Rarity,req.PriceStars,req.UpgradeCostStars,req.MaxLevel,req.MaxSupply,actor).Scan(&g.ID,&g.Code,&g.Title,&g.Description,&g.Rarity,&g.PriceStars,&g.UpgradeCostStars,&g.MaxLevel,&g.MaxSupply,&g.IssuedCount)
	if err!=nil { writeError(w,http.StatusConflict,"gift_code_already_exists_or_invalid");return }
	_,_=h.db.Exec(r.Context(),`INSERT INTO admin_audit_events(actor_id,action,details) VALUES($1,'gift.create',jsonb_build_object('gift_id',$2,'code',$3))`,actor,g.ID,g.Code)
	writeJSON(w,http.StatusCreated,g)
}

func (h economyHandler) purchaseGift(w http.ResponseWriter,r *http.Request) {
	giftID:=r.PathValue("giftID");key:=strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key==""||len(key)>128 { writeError(w,http.StatusBadRequest,"idempotency_key_required");return }
	userID,_:=r.Context().Value(userIDKey).(string);ctx:=r.Context()
	tx,err:=h.db.Begin(ctx);if err!=nil { writeError(w,http.StatusInternalServerError,"database_error");return };defer tx.Rollback(ctx)
	var price,issued int64;var maxSupply *int64
	err=tx.QueryRow(ctx,`SELECT price_stars,issued_count,max_supply FROM gift_definitions WHERE id=$1 AND is_active=TRUE FOR UPDATE`,giftID).Scan(&price,&issued,&maxSupply)
	if err!=nil { writeError(w,http.StatusNotFound,"gift_not_found");return }
	if maxSupply!=nil&&issued>=*maxSupply { writeError(w,http.StatusConflict,"gift_sold_out");return }
	var balance int64
	if err=tx.QueryRow(ctx,"SELECT balance FROM wallet_accounts WHERE user_id=$1 FOR UPDATE",userID).Scan(&balance);err!=nil { writeError(w,http.StatusNotFound,"wallet_not_found");return }
	if balance<price { writeError(w,http.StatusConflict,"insufficient_balance");return }
	newBalance:=balance-price
	if price>0 {
		if _,err=tx.Exec(ctx,"UPDATE wallet_accounts SET balance=$2,updated_at=now() WHERE user_id=$1",userID,newBalance);err!=nil { writeError(w,http.StatusInternalServerError,"database_error");return }
		_,err=tx.Exec(ctx,`INSERT INTO wallet_transactions(user_id,amount,balance_after,reason,idempotency_key,note) VALUES($1,$2,$3,'gift_purchase',$4,'Gift purchase')`,userID,-price,newBalance,key)
		if err!=nil { writeError(w,http.StatusConflict,"duplicate_purchase");return }
	}
	var serial int64
	err=tx.QueryRow(ctx,`UPDATE gift_definitions SET issued_count=issued_count+1 WHERE id=$1 RETURNING issued_count`,giftID).Scan(&serial)
	if err!=nil { writeError(w,http.StatusInternalServerError,"gift_issue_failed");return }
	var ownershipID string
	err=tx.QueryRow(ctx,`INSERT INTO gift_ownership(gift_definition_id,owner_id,serial_number,purchase_key) VALUES($1,$2,$3,$4) RETURNING id::text`,giftID,userID,serial,key).Scan(&ownershipID)
	if err!=nil { writeError(w,http.StatusConflict,"duplicate_purchase");return }
	_,err=tx.Exec(ctx,`INSERT INTO gift_events(gift_ownership_id,actor_id,event_type,idempotency_key,details) VALUES($1,$2,'issued',$4,jsonb_build_object('serial_number',$3))`,ownershipID,userID,serial,key)
	if err!=nil { writeError(w,http.StatusInternalServerError,"gift_issue_failed");return }
	if err=tx.Commit(ctx);err!=nil { writeError(w,http.StatusInternalServerError,"database_error");return }
	writeJSON(w,http.StatusCreated,purchaseGiftResponse{GiftID:giftID,SerialNumber:serial,Level:1,Balance:newBalance})
}

func (h economyHandler) adminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
		userID,_:=r.Context().Value(userIDKey).(string);var role string
		if err:=h.db.QueryRow(r.Context(),"SELECT role FROM users WHERE id=$1 AND is_active=TRUE",userID).Scan(&role);err!=nil||role!="admin" { writeError(w,http.StatusForbidden,"admin_required");return }
		next.ServeHTTP(w,r)
	})
}


type transferGiftRequest struct { RecipientUserID string `json:"recipient_user_id"` }

func (h economyHandler) inventory(w http.ResponseWriter,r *http.Request) {
	userID,_:=r.Context().Value(userIDKey).(string)
	rows,err:=h.db.Query(r.Context(),`SELECT go.id::text,gd.id::text,gd.title,gd.rarity,go.serial_number,go.level,go.acquired_at FROM gift_ownership go JOIN gift_definitions gd ON gd.id=go.gift_definition_id WHERE go.owner_id=$1 ORDER BY go.acquired_at DESC LIMIT 200`,userID)
	if err!=nil { writeError(w,http.StatusInternalServerError,"inventory_failed");return };defer rows.Close()
	type item struct { OwnershipID string `json:"ownership_id"`; GiftID string `json:"gift_id"`; Title string `json:"title"`; Rarity string `json:"rarity"`; SerialNumber int64 `json:"serial_number"`; Level int `json:"level"`; AcquiredAt time.Time `json:"acquired_at"` }
	items:=make([]item,0)
	for rows.Next(){var v item;if err:=rows.Scan(&v.OwnershipID,&v.GiftID,&v.Title,&v.Rarity,&v.SerialNumber,&v.Level,&v.AcquiredAt);err!=nil {writeError(w,http.StatusInternalServerError,"inventory_failed");return};items=append(items,v)}
	if rows.Err()!=nil {writeError(w,http.StatusInternalServerError,"inventory_failed");return}
	writeJSON(w,http.StatusOK,map[string]any{"items":items})
}

func (h economyHandler) transferGift(w http.ResponseWriter,r *http.Request) {
	ownershipID:=r.PathValue("ownershipID");key:=strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key==""||len(key)>128 {writeError(w,http.StatusBadRequest,"idempotency_key_required");return}
	var req transferGiftRequest
	if err:=decodeJSON(r,&req);err!=nil {writeError(w,http.StatusBadRequest,"invalid_json");return}
	actor,_:=r.Context().Value(userIDKey).(string);ctx:=r.Context()
	tx,err:=h.db.Begin(ctx);if err!=nil {writeError(w,http.StatusInternalServerError,"database_error");return};defer tx.Rollback(ctx)
	var currentOwner string
	if err=tx.QueryRow(ctx,"SELECT owner_id::text FROM gift_ownership WHERE id=$1 FOR UPDATE",ownershipID).Scan(&currentOwner);err!=nil||currentOwner!=actor {writeError(w,http.StatusNotFound,"gift_not_found");return}
	if req.RecipientUserID==""||req.RecipientUserID==actor {writeError(w,http.StatusBadRequest,"invalid_recipient");return}
	var active bool
	if err=tx.QueryRow(ctx,"SELECT is_active FROM users WHERE id=$1",req.RecipientUserID).Scan(&active);err!=nil||!active {writeError(w,http.StatusNotFound,"recipient_not_found");return}
	if _,err=tx.Exec(ctx,"UPDATE gift_ownership SET owner_id=$2 WHERE id=$1",ownershipID,req.RecipientUserID);err!=nil {writeError(w,http.StatusInternalServerError,"transfer_failed");return}
	_,err=tx.Exec(ctx,`INSERT INTO gift_events(gift_ownership_id,actor_id,event_type,idempotency_key,details) VALUES($1,$2,'transferred',$3,jsonb_build_object('recipient_user_id',$4))`,ownershipID,actor,key,req.RecipientUserID)
	if err!=nil {writeError(w,http.StatusConflict,"duplicate_transfer");return}
	if err=tx.Commit(ctx);err!=nil {writeError(w,http.StatusInternalServerError,"transfer_failed");return}
	w.WriteHeader(http.StatusNoContent)
}

func (h economyHandler) upgradeGift(w http.ResponseWriter,r *http.Request) {
	ownershipID:=r.PathValue("ownershipID");key:=strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key==""||len(key)>128 {writeError(w,http.StatusBadRequest,"idempotency_key_required");return}
	userID,_:=r.Context().Value(userIDKey).(string);ctx:=r.Context()
	tx,err:=h.db.Begin(ctx);if err!=nil {writeError(w,http.StatusInternalServerError,"database_error");return};defer tx.Rollback(ctx)
	var ownerID,giftID string;var level,maxLevel int;var cost int64
	err=tx.QueryRow(ctx,`SELECT go.owner_id::text,go.gift_definition_id::text,go.level,gd.max_level,gd.upgrade_cost_stars FROM gift_ownership go JOIN gift_definitions gd ON gd.id=go.gift_definition_id WHERE go.id=$1 FOR UPDATE OF go`,ownershipID).Scan(&ownerID,&giftID,&level,&maxLevel,&cost)
	if err!=nil||ownerID!=userID {writeError(w,http.StatusNotFound,"gift_not_found");return}
	if level>=maxLevel {writeError(w,http.StatusConflict,"maximum_level_reached");return}
	var balance int64
	if err=tx.QueryRow(ctx,"SELECT balance FROM wallet_accounts WHERE user_id=$1 FOR UPDATE",userID).Scan(&balance);err!=nil {writeError(w,http.StatusNotFound,"wallet_not_found");return}
	if balance<cost {writeError(w,http.StatusConflict,"insufficient_balance");return}
	newBalance:=balance-cost
	if cost>0 {
		if _,err=tx.Exec(ctx,"UPDATE wallet_accounts SET balance=$2,updated_at=now() WHERE user_id=$1",userID,newBalance);err!=nil {writeError(w,http.StatusInternalServerError,"database_error");return}
		_,err=tx.Exec(ctx,`INSERT INTO wallet_transactions(user_id,amount,balance_after,reason,idempotency_key,note) VALUES($1,$2,$3,'gift_upgrade',$4,'Gift upgrade')`,userID,-cost,newBalance,key)
		if err!=nil {writeError(w,http.StatusConflict,"duplicate_upgrade");return}
	}
	newLevel:=level+1
	if _,err=tx.Exec(ctx,"UPDATE gift_ownership SET level=$2 WHERE id=$1",ownershipID,newLevel);err!=nil {writeError(w,http.StatusInternalServerError,"upgrade_failed");return}
	_,err=tx.Exec(ctx,`INSERT INTO gift_events(gift_ownership_id,actor_id,event_type,idempotency_key,details) VALUES($1,$2,'upgraded',$3,jsonb_build_object('from_level',$4,'to_level',$5,'cost',$6))`,ownershipID,userID,key,level,newLevel,cost)
	if err!=nil {writeError(w,http.StatusConflict,"duplicate_upgrade");return}
	if err=tx.Commit(ctx);err!=nil {writeError(w,http.StatusInternalServerError,"upgrade_failed");return}
	writeJSON(w,http.StatusOK,map[string]any{"ownership_id":ownershipID,"level":newLevel,"upgrade_cost":cost,"balance":newBalance})
}
