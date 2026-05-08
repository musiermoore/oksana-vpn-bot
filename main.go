package main

import (
	"log"
	"oksana-vpn-telegram-bot/internal/commands"
	"oksana-vpn-telegram-bot/internal/telegram"
	"os"
	"time"

	"github.com/joho/godotenv"
	tele "gopkg.in/telebot.v4"
)

func main() {
	// Load .env file if it exists (optional, for local development)
	_ = godotenv.Load()

	telegramClient, err := telegram.NewHTTPClient(os.Getenv("TELEGRAM_PROXY"))
	if err != nil {
		log.Fatal(err)
		return
	}

	pref := tele.Settings{
		Token:  os.Getenv("BOT_TOKEN"),
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
		Client: telegramClient,
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return
	}

	commands.RegisterCommands(bot)

	bot.Start()
}
