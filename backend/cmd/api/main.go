package main

import (
 "context"
 "errors"
 "log"
 "net/http"
 "os"
 "os/signal"
 "syscall"
 "time"
 "privategram/backend/internal/config"
 "privategram/backend/internal/httpapi"
 "privategram/backend/internal/store"
)
func main(){
 cfg,err:=config.Load();if err!=nil{log.Fatal(err)}
 ctx:=context.Background();db,err:=store.Open(ctx,cfg.DatabaseURL);if err!=nil{log.Fatal(err)};defer db.Close()
 server:=&http.Server{Addr:cfg.HTTPAddr,Handler:httpapi.NewRouter(db,cfg.MediaDir,cfg.TelegramBotToken,cfg.TelegramWebhookSecret),ReadHeaderTimeout:5*time.Second,ReadTimeout:15*time.Second,WriteTimeout:30*time.Second,IdleTimeout:60*time.Second}
 go func(){log.Printf("private platform API listening on %s",cfg.HTTPAddr);if err:=server.ListenAndServe();err!=nil&&!errors.Is(err,http.ErrServerClosed){log.Printf("http server: %v",err)}}()
 stop:=make(chan os.Signal,1);signal.Notify(stop,syscall.SIGINT,syscall.SIGTERM);<-stop
 shutdownCtx,cancel:=context.WithTimeout(context.Background(),10*time.Second);defer cancel();if err:=server.Shutdown(shutdownCtx);err!=nil{log.Printf("shutdown: %v",err)}
}
