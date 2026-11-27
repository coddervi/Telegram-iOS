package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const maxMediaBytes = 25 << 20

type mediaHandler struct {
	db *pgxpool.Pool
	root string
}

func (h mediaHandler) upload(w http.ResponseWriter,r *http.Request) {
	r.Body=http.MaxBytesReader(w,r.Body,maxMediaBytes+(1<<20))
	if err:=r.ParseMultipartForm(1<<20);err!=nil {writeError(w,http.StatusBadRequest,"invalid_or_too_large_upload");return}
	if r.MultipartForm!=nil {defer r.MultipartForm.RemoveAll()}
	file,header,err:=r.FormFile("file")
	if err!=nil {writeError(w,http.StatusBadRequest,"file_field_required");return}
	defer file.Close()
	if header.Size>maxMediaBytes {writeError(w,http.StatusRequestEntityTooLarge,"file_too_large");return}
	sample:=make([]byte,512)
	n,_:=file.Read(sample)
	contentType:=http.DetectContentType(sample[:n])
	if !allowedMediaType(contentType) {writeError(w,http.StatusUnsupportedMediaType,"unsupported_media_type");return}
	if _,err=file.Seek(0,io.SeekStart);err!=nil {writeError(w,http.StatusBadRequest,"invalid_upload");return}
	random:=make([]byte,16)
	if _,err=rand.Read(random);err!=nil {writeError(w,http.StatusInternalServerError,"storage_error");return}
	objectKey:=hex.EncodeToString(random)
	if err=os.MkdirAll(h.root,0700);err!=nil {writeError(w,http.StatusInternalServerError,"storage_unavailable");return}
	path:=filepath.Join(h.root,objectKey)
	dst,err:=os.OpenFile(path,os.O_CREATE|os.O_EXCL|os.O_WRONLY,0600)
	if err!=nil {writeError(w,http.StatusInternalServerError,"storage_error");return}
	written,copyErr:=io.Copy(dst,io.LimitReader(file,maxMediaBytes+1))
	closeErr:=dst.Close()
	if copyErr!=nil||closeErr!=nil||written>maxMediaBytes {
		_ = os.Remove(path)
		writeError(w,http.StatusRequestEntityTooLarge,"file_too_large_or_unreadable")
		return
	}
	userID,_:=r.Context().Value(userIDKey).(string)
	var id string
	err=h.db.QueryRow(r.Context(),`INSERT INTO media_objects(owner_id,object_key,content_type,size_bytes) VALUES($1,$2,$3,$4) RETURNING id::text`,userID,objectKey,contentType,written).Scan(&id)
	if err!=nil { _=os.Remove(path);writeError(w,http.StatusInternalServerError,"media_metadata_failed");return }
	writeJSON(w,http.StatusCreated,map[string]any{"id":id,"content_type":contentType,"size_bytes":written})
}

func (h mediaHandler) download(w http.ResponseWriter,r *http.Request) {
	mediaID:=r.PathValue("mediaID")
	userID,_:=r.Context().Value(userIDKey).(string)
	var objectKey,contentType string
	err:=h.db.QueryRow(r.Context(),`SELECT mo.object_key,mo.content_type FROM media_objects mo WHERE mo.id=$1 AND (mo.owner_id=$2 OR EXISTS (SELECT 1 FROM message_media mm JOIN messages m ON m.id=mm.message_id JOIN conversation_members cm ON cm.conversation_id=m.conversation_id WHERE mm.media_id=mo.id AND cm.user_id=$2))`,mediaID,userID).Scan(&objectKey,&contentType)
	if err!=nil {writeError(w,http.StatusNotFound,"media_not_found");return}
	path:=filepath.Join(h.root,filepath.Base(objectKey))
	file,err:=os.Open(path)
	if err!=nil {writeError(w,http.StatusNotFound,"media_not_found");return}
	defer file.Close()
	info,err:=file.Stat();if err!=nil {writeError(w,http.StatusNotFound,"media_not_found");return}
	w.Header().Set("Content-Type",contentType)
	w.Header().Set("Content-Length",stringInt(info.Size()))
	w.Header().Set("X-Content-Type-Options","nosniff")
	w.Header().Set("Cache-Control","private, no-store")
	if !strings.HasPrefix(contentType,"image/")&&!strings.HasPrefix(contentType,"audio/")&&!strings.HasPrefix(contentType,"video/") {w.Header().Set("Content-Disposition",`attachment; filename="download"`)}
	http.ServeContent(w,r,"media",info.ModTime(),file)
}

func allowedMediaType(value string) bool {
	return strings.HasPrefix(value,"image/")||strings.HasPrefix(value,"video/")||strings.HasPrefix(value,"audio/")||value=="application/pdf"||value=="application/zip"||value=="application/octet-stream"||value=="text/plain"
}

func stringInt(v int64) string {
	if v==0 {return "0"}
	var buf [20]byte
	i:=len(buf)
	for v>0 {i--;buf[i]=byte('0'+v%10);v/=10}
	return string(buf[i:])
}
