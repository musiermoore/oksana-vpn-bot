# Oksana VPN Bot (Go)

Telegram bot for managing VPN access, rewritten from Python to Go for better performance, scalability, and maintainability.

## 🚀 About

This project is a **Go rewrite** of the original Python bot:  
- Old version: https://github.com/musiermoore/oksana-vpn-python-bot  
- New version: https://github.com/musiermoore/oksana-vpn-bot  

The rewrite focuses on:
- ⚡ Better performance (Go concurrency)
- 🧠 Cleaner architecture
- 📦 Easier deployment
- 🔒 Improved stability for production usage

---

## ✨ Features

- Telegram bot interface
- VPN user management
- Key / configuration generation (e.g. WireGuard)
- Subscription / access control (if implemented)
- Admin controls
- Scalable architecture using Go routines
- Logging and error handling

---

## 🆚 Migration from Python

| Aspect        | Python Bot | Go Bot |
|--------------|-----------|--------|
| Performance  | Moderate  | High ⚡ |
| Concurrency  | Async (aiogram / asyncio) | Native goroutines |
| Deployment   | Python env required | Single binary |
| Maintainability | Medium | High |

The Go version replaces the Python implementation while keeping core logic and behavior consistent.

---

## 🛠 Tech Stack

- Go (Golang)
- Telegram Bot API
- VPN backend (e.g. WireGuard / scripts / API)
- Optional:
  - Docker
  - Database (if used)
  - External APIs

---

## 📦 Installation

### 1. Clone repository

```bash
git clone https://github.com/musiermoore/oksana-vpn-bot.git
cd oksana-vpn-bot
```

### 2. Configure environment

Create a `.env` file:

```env
BOT_TOKEN=your_telegram_bot_token

# API Url
API_URL=http://localhost/api

# API Basic Auth (optional)
API_BASIC_AUTH_USER=
API_BASIC_AUTH_PASSWORD=
```

### 3. Run locally

```bash
go mod tidy
go run main.go
```

### 4. Build binary

```bash
go build -o bot
./bot
```
