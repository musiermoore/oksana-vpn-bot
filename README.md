# Oksana VPN Telegram Bot

The bot responds to `/start` with a welcome message and links to the website, news channel, and chat.

## Configuration

Create a `.env` file with the bot token. The Telegram proxy is optional.

```env
BOT_TOKEN=your_telegram_bot_token
TELEGRAM_PROXY=socks5h://host.docker.internal:1080
```

Run locally with `go run .`, or build a binary with `go build -o bot .`.
