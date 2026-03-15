# Docker Deployment Guide

## Production Deployment

### Prerequisites
- Docker and Docker Compose installed
- `.env` file with required environment variables

### Environment Variables
Create a `.env` file based on `.env.example`:
```bash
BOT_TOKEN=your_telegram_bot_token
API_URL=https://your-api-url.com
API_BASIC_AUTH_USER=your_username
API_BASIC_AUTH_PASSWORD=your_password
```

### Build and Run

Build and start the bot:
```bash
docker-compose -f docker-compose.prod.yml up -d --build
```

### Management Commands

View logs:
```bash
docker-compose -f docker-compose.prod.yml logs -f
```

Stop the bot:
```bash
docker-compose -f docker-compose.prod.yml down
```

Restart the bot:
```bash
docker-compose -f docker-compose.prod.yml restart
```

Rebuild and restart:
```bash
docker-compose -f docker-compose.prod.yml up -d --build --force-recreate
```

### Restart Policy

The bot is configured with `restart: unless-stopped`, which means:
- Automatically restarts on errors
- Restarts after system reboot
- Only stops when manually stopped with `docker-compose down`

### Resource Limits

The production setup includes resource limits:
- CPU: 0.25-0.5 cores
- Memory: 128-256 MB

Adjust these in `docker-compose.prod.yml` if needed.
