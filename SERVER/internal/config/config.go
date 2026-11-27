package config

import (
 "fmt"
 "os"
)
type Config struct { HTTPAddr string; DatabaseURL string; Environment string; MediaDir string; TelegramBotToken string; TelegramWebhookSecret string }
func Load()(Config,error){
 cfg:=Config{HTTPAddr:env("HTTP_ADDR",":8080"),DatabaseURL:os.Getenv("DATABASE_URL"),Environment:env("APP_ENV","development"),MediaDir:env("MEDIA_DIR","./data/media"),TelegramBotToken:os.Getenv("TELEGRAM_BOT_TOKEN"),TelegramWebhookSecret:os.Getenv("TELEGRAM_WEBHOOK_SECRET")}
 if cfg.DatabaseURL==""{return Config{},fmt.Errorf("DATABASE_URL is required")}
 if (cfg.TelegramBotToken=="")!=(cfg.TelegramWebhookSecret==""){return Config{},fmt.Errorf("TELEGRAM_BOT_TOKEN and TELEGRAM_WEBHOOK_SECRET must be configured together")}
 return cfg,nil
}
func env(key,fallback string)string{if value:=os.Getenv(key);value!=""{return value};return fallback}
