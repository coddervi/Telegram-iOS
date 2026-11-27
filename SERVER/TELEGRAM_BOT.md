# Telegram bot registration

Set `TELEGRAM_BOT_TOKEN` to the token from BotFather and `TELEGRAM_WEBHOOK_SECRET` to a random secret (at least 32 characters). After deploying the API on HTTPS, configure the bot webhook:

```bash
curl -X POST "https://api.telegram.org/bot$TELEGRAM_BOT_TOKEN/setWebhook" \
  -d "url=https://YOUR_API_HOST/v1/telegram/webhook" \
  -d "secret_token=$TELEGRAM_WEBHOOK_SECRET"
```

Users send `/start` to the bot and receive a unique internal `PP-XXXXXXXX` account number. In the app they enter that number and their username/password, request a code, then enter the code delivered to the same Telegram chat. These are platform IDs, not phone numbers or Telegram login codes. A number is bound to one account and cannot be reused.
